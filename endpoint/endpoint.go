package endpoint

import (
	"context"
	"crypto/tls"
	"time"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"

	"google.golang.org/api/support/bundler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	data_registry "github.com/zenoss/zenoss-protobufs/go/cloud/data-registry"
	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/internal/ttl"
	"github.com/zenoss/zenoss-go-sdk/log"
	zstats "github.com/zenoss/zenoss-go-sdk/stats"
)

const (
	// DefaultAddress is the default Zenoss API endpoint address.
	// Overridden by Config.Address.
	DefaultAddress = "api.zenoss.io:443"

	// DefaultTimeout is the default time to wait before timing out requests to the endpoint.
	// Overridden by Config.Timeout.
	DefaultTimeout = 1 * time.Minute

	// DefaultModelTTL controls how long duplicate models will be suppressed before being resent.
	// Overridden by Config.ModelTTL.
	DefaultModelTTL = 1 * time.Hour

	// DefaultBundleCountThreshold is the maximum number of items (metrics, models) will be buffered before sending.
	// Each type of item has its own buffer of this size.
	// Overridden by Config.BundleConfig.BundleCountThreshold.
	DefaultBundleCountThreshold = 1000

	// DefaultBundleDelayThreshold is the longest time an item will be buffered before sending.
	// Overridden by Config.BundleConfig.BundleDelayThreshold.
	DefaultBundleDelayThreshold = 1 * time.Second

	// APIKeyHeader is the gRPC header field containing a Zenoss API key.
	APIKeyHeader = "zenoss-api-key"
)

var (
	// Ensure Endpoint implements DataReceiverServiceClient interface.
	_ data_receiver.DataReceiverServiceClient = (*Endpoint)(nil)

	// Ensure Endpoint implements log.Logger interface.
	_ log.Logger = (*Endpoint)(nil)
)

// Config specifies an endpoint's configuration.
type Config struct {
	// Name is an arbitrary name for the endpoint for disambiguation of multiple endpoints.
	// Optional.
	Name string `yaml:"name"`

	// APIKey is a Zenoss API key.
	// Required when sending to an official Zenoss API endpoint.
	// Optional when sending elsewhere such as to zenoss-api-proxy.
	APIKey string `yaml:"apiKey"`

	// Address is the gRPC endpoint address of the API.
	// Default: api.zenoss.io:443
	Address string `yaml:"address"`

	// DisableTLS will disable TLS (a.k.a. use HTTP) if set to true.
	// Default: false
	DisableTLS bool `yaml:"disableTLS"`

	// InsecureTLS will disable certificate validation if set to true.
	// Default: false
	InsecureTLS bool `yaml:"insecureTLS"`

	// Timeout is the time to wait before timing out requests to the endpoint.
	// Default: 1 minute
	Timeout time.Duration `yaml:"timeout"`

	// ModelTTL controls how long duplicate models will be suppressed before being resent.
	// Default: 1 hour
	ModelTTL time.Duration `yaml:"modelTTL"`

	// BundlerConfig specifies the bundler configuration.
	// Optional.
	BundlerConfig BundlerConfig `yaml:"bundler"`

	// LoggerConfig specifies the logging configuration.
	// Optional.
	LoggerConfig log.LoggerConfig `yaml:"logger"`

	// TestClient is for testing only. Do not set for normal use.
	TestClient data_receiver.DataReceiverServiceClient
}

// BundlerConfig specifies a bundler configuration.
type BundlerConfig struct {
	// Starting from the time that the first message is added to a bundle, once
	// this delay has passed, handle the bundle. The default is DefaultDelayThreshold.
	DelayThreshold time.Duration `yaml:"delayThreshold"`

	// Once a bundle has this many items, handle the bundle. Since only one
	// item at a time is added to a bundle, no bundle will exceed this
	// threshold, so it also serves as a limit. The default is
	// DefaultBundleCountThreshold.
	BundleCountThreshold int `yaml:"bundleCountThreshold"`

	// Once the number of bytes in current bundle reaches this threshold, handle
	// the bundle. The default is DefaultBundleByteThreshold. This triggers handling,
	// but does not cap the total size of a bundle.
	BundleByteThreshold int `yaml:"bundleByteThreshold"`

	// The maximum size of a bundle, in bytes. Zero means unlimited.
	BundleByteLimit int `yaml:"bundleByteLimit"`

	// The maximum number of bytes that the Bundler will keep in memory before
	// returning ErrOverflow. The default is DefaultBufferedByteLimit.
	BufferedByteLimit int `yaml:"bufferedByteLimit"`

	// The maximum number of handler invocations that can be running at once.
	// The default is 1.
	HandlerLimit int `yaml:"handlerLimit"`
}

// Endpoint represents a Zenoss API endpoint.
type Endpoint struct {
	config       Config
	client       data_receiver.DataReceiverServiceClient
	regclient    data_registry.DataRegistryServiceClient
	modelTracker *ttl.Tracker
	tagMutators  []tag.Mutator

	bundlers struct {
		events         *bundler.Bundler
		models         *bundler.Bundler
		metrics        *bundler.Bundler
		taggedMetrics  *bundler.Bundler
		compactMetrics *bundler.Bundler
	}
}

// New returns a new Endpoint with specified configuration.
func New(config Config) (*Endpoint, error) {
	if config.Name == "" {
		config.Name = "default"
	}

	if config.Address == "" {
		config.Address = DefaultAddress
	}

	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	if config.ModelTTL == 0 {
		config.ModelTTL = DefaultModelTTL
	}

	if config.LoggerConfig.Fields == nil {
		config.LoggerConfig.Fields = log.Fields{"endpoint": config.Name}
	}

	var client data_receiver.DataReceiverServiceClient
	var regclient data_registry.DataRegistryServiceClient

	if config.TestClient != nil {
		client = config.TestClient
	} else {
		dialOptions := make([]grpc.DialOption, 4)

		if config.DisableTLS {
			dialOptions[0] = grpc.WithInsecure()
		} else {
			dialOptions[0] = grpc.WithTransportCredentials(
				credentials.NewTLS(
					&tls.Config{
						InsecureSkipVerify: config.InsecureTLS}))
		}

		// Enable gRPC retry middleware.
		retryOptions := []grpc_retry.CallOption{
			grpc_retry.WithMax(5),
			grpc_retry.WithPerRetryTimeout(config.Timeout / 3),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(200 * time.Millisecond)),
		}

		dialOptions[1] = grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOptions...))
		dialOptions[2] = grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOptions...))

		// Enable OpenCensus gRPC client stats.
		dialOptions[3] = grpc.WithStatsHandler(&ocgrpc.ClientHandler{})

		// Dial doesn't block by default. So no error can actually occur.
		conn, _ := grpc.Dial(config.Address, dialOptions...)
		client = data_receiver.NewDataReceiverServiceClient(conn)
		regDialOption := grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
		regconn, _ := grpc.Dial(config.Address, regDialOption)
		regclient = data_registry.NewDataRegistryServiceClient(regconn)
	}

	e := &Endpoint{
		config:       config,
		client:       client,
		regclient:    regclient,
		modelTracker: ttl.NewTracker(config.ModelTTL, time.Second),
		tagMutators: []tag.Mutator{
			tag.Upsert(zstats.KeyModuleType, "endpoint"),
			tag.Upsert(zstats.KeyModuleName, config.Name),
		},
	}

	e.createBundlers()

	return e, nil
}

func (e *Endpoint) createBundlers() {

	e.bundlers.events = e.createBundler((*data_receiver.Event)(nil), func(bundle interface{}) {
		e.putEvents(&data_receiver.Events{Events: bundle.([]*data_receiver.Event)})
	})

	e.bundlers.models = e.createBundler((*data_receiver.Model)(nil), func(bundle interface{}) {
		e.putModels(&data_receiver.Models{Models: bundle.([]*data_receiver.Model)})
	})

	e.bundlers.metrics = e.createBundler((*data_receiver.Metric)(nil), func(bundle interface{}) {
		e.putMetrics(&data_receiver.Metrics{Metrics: bundle.([]*data_receiver.Metric)})
	})

	e.bundlers.taggedMetrics = e.createBundler((*data_receiver.TaggedMetric)(nil), func(bundle interface{}) {
		e.putMetrics(&data_receiver.Metrics{TaggedMetrics: bundle.([]*data_receiver.TaggedMetric)})
	})

	e.bundlers.compactMetrics = e.createBundler((*data_receiver.CompactMetric)(nil), func(bundle interface{}) {
		e.putMetrics(&data_receiver.Metrics{CompactMetrics: bundle.([]*data_receiver.CompactMetric)})
	})
}

func (e *Endpoint) createBundler(itemExample interface{}, handler func(interface{})) *bundler.Bundler {
	b := bundler.NewBundler(itemExample, handler)

	if e.config.BundlerConfig.DelayThreshold > 0 {
		b.DelayThreshold = e.config.BundlerConfig.DelayThreshold
	} else {
		b.DelayThreshold = DefaultBundleDelayThreshold
	}

	if e.config.BundlerConfig.BundleCountThreshold > 0 {
		b.BundleCountThreshold = e.config.BundlerConfig.BundleCountThreshold
	} else {
		b.BundleCountThreshold = DefaultBundleCountThreshold
	}

	if e.config.BundlerConfig.BundleByteLimit > 0 {
		b.BundleByteLimit = e.config.BundlerConfig.BundleByteLimit
	}

	if e.config.BundlerConfig.BufferedByteLimit > 0 {
		b.BufferedByteLimit = e.config.BundlerConfig.BufferedByteLimit
	}

	if e.config.BundlerConfig.HandlerLimit > 0 {
		b.HandlerLimit = e.config.BundlerConfig.HandlerLimit
	}

	return b
}

// GetLoggerConfig returns logging configuration.
// Satisfies log.Logger interface.
func (e *Endpoint) GetLoggerConfig() log.LoggerConfig {
	return e.config.LoggerConfig
}

// CreateOrUpdateMetrics uses DataRegistryService CreateOrUpdateMetrics to create or update metrics
func (e *Endpoint) CreateOrUpdateMetrics(ctx context.Context, metricWrappers []*data_receiver.MetricWrapper) (*data_registry.RegisterMetricsResponse, error) {
	var failedRegistrationCount, succeededRegistrationsCount int32
	if e.config.APIKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, APIKeyHeader, e.config.APIKey)
	}
	stream, err := e.regclient.CreateOrUpdateMetrics(ctx)
	if err != nil {
		return nil, err
	}
	for _, metricWrapper := range metricWrappers {
		request := &data_registry.RegisterMetricRequest{
			Metric:     metricWrapper,
			UpdateMode: data_registry.UpdateMode_REPLACE, // replace by default
		}
		if err := stream.Send(request); err != nil {
			log.Log(e, log.LevelError, log.Fields{
				"metric wrapper": metricWrapper,
			}, "Error sending metric wrapper to data_registry")
			_ = stats.RecordWithTags(ctx, e.tagMutators,
				zstats.SucceededRegistrations.M(int64(succeededRegistrationsCount)),
				zstats.FailedRegistrations.M(int64(len(metricWrappers))),
			)
			return nil, err
		}
	}
	registerResponse, err := stream.CloseAndRecv()
	if err != nil {
		log.Log(e, log.LevelError, log.Fields{}, "Error closing stream")
		_ = stats.RecordWithTags(ctx, e.tagMutators,
			zstats.SucceededRegistrations.M(int64(succeededRegistrationsCount)),
			zstats.FailedRegistrations.M(int64(len(metricWrappers))),
		)
		return nil, err
	}

	for _, response := range registerResponse.Responses {
		if response.Error != "" {
			failedRegistrationCount++
		} else {
			succeededRegistrationsCount++
		}
	}
	_ = stats.RecordWithTags(ctx, e.tagMutators,
		zstats.SucceededRegistrations.M(int64(succeededRegistrationsCount)),
		zstats.FailedRegistrations.M(int64(failedRegistrationCount)),
	)
	return registerResponse, nil
}

// PutEvents implements DataReceiverService PutEvents unary RPC.
func (e *Endpoint) PutEvents(ctx context.Context, events *data_receiver.Events, opts ...grpc.CallOption) (*data_receiver.EventStatusResult, error) {
	var failedEventsCount, succeededEventsCount int32
	var failedEvents []*data_receiver.EventError
	var err error

	if events.DetailedResponse {
		failedEvents = make([]*data_receiver.EventError, 0, len(events.Events))
	}

	for _, event := range events.Events {
		if err = e.bundlers.events.Add(event, 1); err != nil {
			failedEventsCount++

			if events.DetailedResponse {
				failedEvents = append(failedEvents, &data_receiver.EventError{
					Error: err.Error(),
					Event: event,
				})
			}
		} else {
			succeededEventsCount++
		}
	}

	// Only record received and failed here.
	// Record sent and failed (again) when actually sent.
	_ = stats.RecordWithTags(ctx, e.tagMutators,
		zstats.ReceivedEvents.M(int64(len(events.Events))),
		zstats.FailedEvents.M(int64(failedEventsCount)),
	)

	return &data_receiver.EventStatusResult{
		Failed:       failedEventsCount,
		Succeeded:    succeededEventsCount,
		FailedEvents: failedEvents,
	}, nil
}

// PutEvent implements DataReceiverService PutEvent streaming RPC.
func (e *Endpoint) PutEvent(ctx context.Context, opts ...grpc.CallOption) (data_receiver.DataReceiverService_PutEventClient, error) {
	return nil, status.Error(codes.Unimplemented, "PutEvent is not supported")

}

// PutMetric implements DataReceiverService PutMetric streaming RPC.
func (e *Endpoint) PutMetric(context.Context, ...grpc.CallOption) (data_receiver.DataReceiverService_PutMetricClient, error) {
	return nil, status.Error(codes.Unimplemented, "PutMetric is not supported")
}

// PutMetrics implements DataReceiverService PutMetrics unary RPC.
func (e *Endpoint) PutMetrics(ctx context.Context, metrics *data_receiver.Metrics, _ ...grpc.CallOption) (*data_receiver.StatusResult, error) {
	var failedCompactMetricsCount, succeededCompactMetricsCount int32
	var failedCompactMetrics []*data_receiver.CompactMetricError
	var failedTaggedMetricsCount, succeededTaggedMetricsCount int32
	var failedTaggedMetrics []*data_receiver.TaggedMetricError
	var failedMetricsCount, succeededMetricsCount int32
	var failedMetrics []*data_receiver.MetricError
	var err error

	if metrics.DetailedResponse {
		failedCompactMetrics = make([]*data_receiver.CompactMetricError, 0, len(metrics.CompactMetrics))
		failedTaggedMetrics = make([]*data_receiver.TaggedMetricError, 0, len(metrics.TaggedMetrics))
		failedMetrics = make([]*data_receiver.MetricError, 0, len(metrics.Metrics))
	}

	if metrics.CompactMetrics != nil {
		for _, compactMetric := range metrics.CompactMetrics {
			if err = e.bundlers.compactMetrics.Add(compactMetric, 1); err != nil {
				failedCompactMetricsCount++

				if metrics.DetailedResponse {
					failedCompactMetrics = append(failedCompactMetrics, &data_receiver.CompactMetricError{
						Error:  err.Error(),
						Metric: compactMetric,
					})
				}
			} else {
				succeededCompactMetricsCount++
			}
		}
	}

	if metrics.TaggedMetrics != nil {
		for _, taggedMetric := range metrics.TaggedMetrics {
			if err = e.bundlers.taggedMetrics.Add(taggedMetric, 1); err != nil {
				failedTaggedMetricsCount++

				if metrics.DetailedResponse {
					failedTaggedMetrics = append(failedTaggedMetrics, &data_receiver.TaggedMetricError{
						Error:  err.Error(),
						Metric: taggedMetric,
					})
				}
			} else {
				succeededTaggedMetricsCount++
			}
		}
	}

	if metrics.Metrics != nil {
		for _, metric := range metrics.Metrics {
			if err = e.bundlers.metrics.Add(metric, 1); err != nil {
				failedMetricsCount++

				if metrics.DetailedResponse {
					failedMetrics = append(failedMetrics, &data_receiver.MetricError{
						Error:  err.Error(),
						Metric: metric,
					})
				}
			} else {
				succeededMetricsCount++
			}
		}
	}

	// Only record received and failed here.
	// Record sent and failed (again) when actually sent.
	_ = stats.RecordWithTags(ctx, e.tagMutators,
		zstats.ReceivedCompactMetrics.M(int64(len(metrics.CompactMetrics))),
		zstats.FailedCompactMetrics.M(int64(failedCompactMetricsCount)),
		zstats.ReceivedTaggedMetrics.M(int64(len(metrics.TaggedMetrics))),
		zstats.FailedTaggedMetrics.M(int64(failedTaggedMetricsCount)),
		zstats.ReceivedMetrics.M(int64(len(metrics.Metrics))),
		zstats.FailedMetrics.M(int64(failedMetricsCount)),
	)

	return &data_receiver.StatusResult{
		Succeeded:            succeededCompactMetricsCount + succeededTaggedMetricsCount + succeededMetricsCount,
		Failed:               failedCompactMetricsCount + failedTaggedMetricsCount + failedMetricsCount,
		FailedCompactMetrics: failedCompactMetrics,
		FailedTaggedMetrics:  failedTaggedMetrics,
		FailedMetrics:        failedMetrics,
	}, nil
}

// PutModels implements DataReceiverService PutModels unary RPC.
func (e *Endpoint) PutModels(ctx context.Context, models *data_receiver.Models, _ ...grpc.CallOption) (*data_receiver.ModelStatusResult, error) {
	var failedModelsCount, succeededModelsCount int32
	var failedModels []*data_receiver.ModelError
	var err error

	if models.DetailedResponse {
		failedModels = make([]*data_receiver.ModelError, 0, len(models.Models))
	}

	if len(models.Models) > 0 {
		for _, model := range models.Models {
			if e.modelTracker.IsModelAlive(model) {
				// We've sent this exact model within the last ModelTTL.
				continue
			}

			if err = e.bundlers.models.Add(model, 1); err != nil {
				failedModelsCount++

				if models.DetailedResponse {
					failedModels = append(failedModels, &data_receiver.ModelError{
						Error: err.Error(),
						Model: model,
					})
				}
			} else {
				succeededModelsCount++
			}
		}
	}

	// Only record received and failed here.
	// Record sent and failed (again) when actually sent.
	_ = stats.RecordWithTags(ctx, e.tagMutators,
		zstats.ReceivedModels.M(int64(len(models.Models))),
		zstats.FailedModels.M(int64(failedModelsCount)),
	)

	return &data_receiver.ModelStatusResult{
		Failed:       failedModelsCount,
		Succeeded:    succeededModelsCount,
		FailedModels: failedModels,
	}, nil
}

// Flush waits for buffered data to be sent. Call before program termination.
func (e *Endpoint) Flush() {
	e.bundlers.models.Flush()
	e.bundlers.metrics.Flush()
	e.bundlers.compactMetrics.Flush()
	e.bundlers.taggedMetrics.Flush()
	e.bundlers.events.Flush()
}

// putMetrics is called by Endpoint.bundlers.*metrics.
func (e *Endpoint) putMetrics(metrics *data_receiver.Metrics) {
	ctx, cancel := context.WithTimeout(context.Background(), e.config.Timeout)
	defer cancel()

	if e.config.APIKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, APIKeyHeader, e.config.APIKey)
	}

	// For compiling individual results into final result.
	var failedCompactMetricsCount, succeededCompactMetricsCount int32
	var failedTaggedMetricsCount, succeededTaggedMetricsCount int32
	var failedMetricsCount, succeededMetricsCount int32
	var err error
	var r *data_receiver.StatusResult

	// Compact metrics.
	if len(metrics.CompactMetrics) > 0 {
		if r, err = e.client.PutMetrics(ctx, &data_receiver.Metrics{
			CompactMetrics: metrics.CompactMetrics,
		}); err != nil {
			failedCompactMetricsCount += int32(len(metrics.CompactMetrics))

			log.Log(e, log.LevelWarning, log.Fields{
				"error":     err,
				"failed":    len(metrics.CompactMetrics),
				"succeeded": 0,
			}, "failed to send compact metrics")
		} else {
			failedCompactMetricsCount += r.GetFailed()
			succeededCompactMetricsCount += r.GetSucceeded()

			if r.GetFailed() > 0 {
				log.Log(e, log.LevelWarning, log.Fields{
					"failed":    r.GetFailed(),
					"succeeded": r.GetSucceeded(),
				}, "failed to send some compact metrics")
			} else {
				log.Log(e, log.LevelDebug, log.Fields{
					"failed":    0,
					"succeeded": len(metrics.CompactMetrics),
				}, "sent compact metrics")
			}
		}
	}

	// Tagged metrics.
	if len(metrics.TaggedMetrics) > 0 {
		if r, err = e.client.PutMetrics(ctx, &data_receiver.Metrics{
			TaggedMetrics: metrics.TaggedMetrics,
		}); err != nil {
			failedTaggedMetricsCount += int32(len(metrics.TaggedMetrics))

			log.Log(e, log.LevelWarning, log.Fields{
				"error":     err,
				"failed":    len(metrics.TaggedMetrics),
				"succeeded": 0,
			}, "failed to send tagged metrics")
		} else {
			failedTaggedMetricsCount += r.GetFailed()
			succeededTaggedMetricsCount += r.GetSucceeded()

			if r.GetFailed() > 0 {
				log.Log(e, log.LevelWarning, log.Fields{
					"failed":    r.GetFailed(),
					"succeeded": r.GetSucceeded(),
				}, "failed to send some tagged metrics")
			} else {
				log.Log(e, log.LevelDebug, log.Fields{
					"failed":    0,
					"succeeded": len(metrics.TaggedMetrics),
				}, "sent tagged metrics")
			}
		}
	}

	// Canonical metrics.
	if len(metrics.Metrics) > 0 {
		if r, err = e.client.PutMetrics(ctx, &data_receiver.Metrics{
			Metrics: metrics.Metrics,
		}); err != nil {
			failedMetricsCount += int32(len(metrics.Metrics))

			log.Log(e, log.LevelWarning, log.Fields{
				"error":     err,
				"failed":    len(metrics.Metrics),
				"succeeded": 0,
			}, "failed to send metrics")
		} else {
			failedMetricsCount += r.GetFailed()
			succeededMetricsCount += r.GetSucceeded()

			if r.GetFailed() > 0 {
				log.Log(e, log.LevelWarning, log.Fields{
					"failed":    r.GetFailed(),
					"succeeded": r.GetSucceeded(),
				}, "failed to send some metrics")
			} else {
				log.Log(e, log.LevelDebug, log.Fields{
					"failed":    0,
					"succeeded": len(metrics.Metrics),
				}, "sent metrics")
			}
		}
	}

	// Only record sent and failed here. Received is recorded in PutMetrics.
	_ = stats.RecordWithTags(ctx, e.tagMutators,
		zstats.SentCompactMetrics.M(int64(succeededCompactMetricsCount)),
		zstats.FailedCompactMetrics.M(int64(failedCompactMetricsCount)),
		zstats.SentTaggedMetrics.M(int64(succeededTaggedMetricsCount)),
		zstats.FailedTaggedMetrics.M(int64(failedTaggedMetricsCount)),
		zstats.SentMetrics.M(int64(succeededMetricsCount)),
		zstats.FailedMetrics.M(int64(failedMetricsCount)),
	)
}

// putModels is called by Endpoint.bundlers.models.
func (e *Endpoint) putModels(models *data_receiver.Models) {
	ctx, cancel := context.WithTimeout(context.Background(), e.config.Timeout)
	defer cancel()

	if e.config.APIKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, APIKeyHeader, e.config.APIKey)
	}

	var failedModelsCount, succeededModelsCount int32
	var err error
	var r *data_receiver.ModelStatusResult

	if len(models.Models) > 0 {
		if r, err = e.client.PutModels(ctx, &data_receiver.Models{
			Models: models.Models,
		}); err != nil {
			failedModelsCount += int32(len(models.Models))

			log.Log(e, log.LevelWarning, log.Fields{
				"error":     err,
				"failed":    len(models.Models),
				"succeeded": 0,
			}, "failed to send models")
		} else {
			failedModelsCount += r.GetFailed()
			succeededModelsCount += r.GetSucceeded()

			if r.GetFailed() > 0 {
				log.Log(e, log.LevelWarning, log.Fields{
					"failed":    r.GetFailed(),
					"succeeded": r.GetSucceeded(),
				}, "failed to send some models")
			} else {
				log.Log(e, log.LevelDebug, log.Fields{
					"failed":    0,
					"succeeded": len(models.Models),
				}, "sent models")
			}
		}
	}

	// Only record sent and failed here. Received is recorded in PutModels.
	_ = stats.RecordWithTags(ctx, e.tagMutators,
		zstats.SentModels.M(int64(succeededModelsCount)),
		zstats.FailedModels.M(int64(failedModelsCount)),
	)
}

func (e *Endpoint) putEvents(events *data_receiver.Events) {
	ctx, cancel := context.WithTimeout(context.Background(), e.config.Timeout)
	defer cancel()

	if e.config.APIKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, APIKeyHeader, e.config.APIKey)
	}

	var failedEventsCount, succeededEventsCount int32
	var err error
	var r *data_receiver.EventStatusResult

	if len(events.Events) > 0 {
		if r, err = e.client.PutEvents(ctx, &data_receiver.Events{
			Events: events.Events,
		}); err != nil {
			failedEventsCount += int32(len(events.Events))

			log.Log(e, log.LevelWarning, log.Fields{
				"error":     err,
				"failed":    len(events.Events),
				"succeeded": 0,
			}, "failed to send events")
		} else {
			failedEventsCount += r.GetFailed()
			succeededEventsCount += r.GetSucceeded()

			if r.GetFailed() > 0 {
				log.Log(e, log.LevelWarning, log.Fields{
					"failed":    r.GetFailed(),
					"succeeded": r.GetSucceeded(),
				}, "failed to send some events")
			} else {
				log.Log(e, log.LevelDebug, log.Fields{
					"failed":    0,
					"succeeded": len(events.Events),
				}, "sent events")
			}
		}
	}

	// Only record sent and failed here. Received is recorded in PutEvents.
	_ = stats.RecordWithTags(ctx, e.tagMutators,
		zstats.SentEvents.M(int64(succeededEventsCount)),
		zstats.FailedEvents.M(int64(failedEventsCount)),
	)
}

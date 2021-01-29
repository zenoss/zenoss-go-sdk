package proxy

import (
	"context"
	"fmt"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/endpoint"
	"github.com/zenoss/zenoss-go-sdk/log"
	zstats "github.com/zenoss/zenoss-go-sdk/stats"
)

var (
	// Ensure Proxy implements DataReceiverServiceServer interface.
	_ data_receiver.DataReceiverServiceServer = (*Proxy)(nil)

	// Ensure Proxy implements log.Logger interface.
	_ log.Logger = (*Proxy)(nil)
)

// Config specifies a Proxy's configuration.
type Config struct {
	// Name is an arbitrary name for the endpoint for disambiguation olf multiple proxies.
	// Optional.
	Name string `yaml:"name"`

	// AllowedAPIKeys is a list of acceptable API keys.
	// A nil or empty list means that API keys will not be enforced at all.
	// Default: nil
	AllowedAPIKeys []string `yaml:"allowedAPIKeys"`

	// Output is a client to which the proxy will send all received data.
	// Typical output types are endpoint.Endpoint, splitter.Splitter, and processor.Processor.
	// Required.
	Output data_receiver.DataReceiverServiceClient

	// LoggerConfig specifies the logging configuration.
	// Optional.
	LoggerConfig log.LoggerConfig `yaml:"logger"`
}

// Proxy is a proxy for Zenoss' DataReceiverService API.
type Proxy struct {
	config         Config
	allowedAPIKeys map[string]interface{}
	output         data_receiver.DataReceiverServiceClient
	tagMutators    []tag.Mutator
}

// New returns a new Proxy with specified configuration.
func New(config Config) (*Proxy, error) {
	p := &Proxy{}
	err := p.SetConfig(config)
	if err != nil {
		return nil, err
	}

	return p, nil
}

// SetConfig allows the proxy's configuration to be changed while running.
func (p *Proxy) SetConfig(config Config) error {
	if config.Output == nil {
		return fmt.Errorf("Config.Output is nil")
	}

	if config.Name == "" {
		config.Name = "default"
	}

	if config.LoggerConfig.Fields == nil {
		config.LoggerConfig.Fields = log.Fields{"proxy": config.Name}
	}

	allowedAPIKeys := make(map[string]interface{}, len(config.AllowedAPIKeys))
	for _, apiKey := range config.AllowedAPIKeys {
		allowedAPIKeys[apiKey] = nil
	}

	p.config = config
	p.allowedAPIKeys = allowedAPIKeys
	p.output = config.Output
	p.tagMutators = []tag.Mutator{
		tag.Upsert(zstats.KeyModuleType, "proxy"),
		tag.Upsert(zstats.KeyModuleName, p.config.Name),
	}

	return nil
}

// GetLoggerConfig returns logging configuration.
// Satisfies log.Logger interface.
func (p *Proxy) GetLoggerConfig() log.LoggerConfig {
	return p.config.LoggerConfig
}

// PutEvents implements DataReceiverService PutEvents unary RPC.
func (p *Proxy) PutEvents(ctx context.Context, events *data_receiver.Events) (*data_receiver.EventStatusResult, error) {
	err := p.checkAPIKey(ctx)
	if err != nil {
		return nil, err
	}
	var (
		failedEventsCount    int32
		succeededEventsCount int32
		failedEvents         = make([]*data_receiver.EventError, 0)
	)
	if len(events.Events) > 0 {
		if r, err := p.output.PutEvents(ctx, &data_receiver.Events{
			DetailedResponse: events.DetailedResponse,
			Events:           events.Events,
		}); err != nil {
			// TODO: capture failed events count
			failedEventsCount += int32(len(events.Events))

			if events.DetailedResponse {
				for _, e := range events.Events {
					failedEvents = append(failedEvents, &data_receiver.EventError{
						Error: err.Error(),
						Event: e,
					})
				}
			}
		} else {
			// TODO: capture failed events count
			failedEventsCount += r.GetFailed()
			// TODO: capture succeeded events count
			succeededEventsCount += r.GetSucceeded()

			if events.DetailedResponse {
				failedEvents = append(failedEvents, r.GetFailedEvents()...)
			}

		}
	}

	_ = stats.RecordWithTags(ctx, p.tagMutators,
		zstats.ReceivedEvents.M(int64(len(events.Events))),
		zstats.SentEvents.M(int64(succeededEventsCount)),
		zstats.FailedEvents.M(int64(failedEventsCount)),
	)

	return &data_receiver.EventStatusResult{
		Failed:       failedEventsCount,
		Succeeded:    succeededEventsCount,
		FailedEvents: failedEvents,
	}, nil
}

// PutEvent implements DataReceiverService PutEvent streaming RPC.
func (p *Proxy) PutEvent(data_receiver.DataReceiverService_PutEventServer) error {
	return status.Error(codes.Unimplemented, "PutEvent is not supported")

}

// PutMetric is not supported.
// Satisfies data_receiver.DataReceiverServiceServer interface.
func (p *Proxy) PutMetric(data_receiver.DataReceiverService_PutMetricServer) error {
	return status.Error(codes.Unimplemented, "PutMetric is not supported")
}

// PutMetrics receives metrics of all kinds via gRPC.
// Satisfies data_receiver.DataReceiverServiceServer interface.
func (p *Proxy) PutMetrics(ctx context.Context, metrics *data_receiver.Metrics) (*data_receiver.StatusResult, error) {
	err := p.checkAPIKey(ctx)
	if err != nil {
		return nil, err
	}

	var failedCompactMetricsCount, succeededCompactMetricsCount int32
	var failedCompactMetrics []*data_receiver.CompactMetricError
	var failedTaggedMetricsCount, succeededTaggedMetricsCount int32
	var failedTaggedMetrics []*data_receiver.TaggedMetricError
	var failedMetricsCount, succeededMetricsCount int32
	var failedMetrics []*data_receiver.MetricError
	var r *data_receiver.StatusResult

	if metrics.DetailedResponse {
		failedCompactMetrics = make([]*data_receiver.CompactMetricError, 0, len(metrics.CompactMetrics))
		failedTaggedMetrics = make([]*data_receiver.TaggedMetricError, 0, len(metrics.TaggedMetrics))
		failedMetrics = make([]*data_receiver.MetricError, 0, len(metrics.Metrics))
	}

	if len(metrics.CompactMetrics) > 0 {
		if r, err = p.output.PutMetrics(ctx, &data_receiver.Metrics{
			DetailedResponse: metrics.DetailedResponse,
			CompactMetrics:   metrics.CompactMetrics,
		}); err != nil {
			failedCompactMetricsCount += int32(len(metrics.CompactMetrics))

			if metrics.DetailedResponse {
				for _, m := range metrics.CompactMetrics {
					failedCompactMetrics = append(failedCompactMetrics, &data_receiver.CompactMetricError{
						Error:  err.Error(),
						Metric: m,
					})
				}
			}
		} else {
			failedCompactMetricsCount += r.GetFailed()
			succeededCompactMetricsCount += r.GetSucceeded()

			if metrics.DetailedResponse {
				failedCompactMetrics = append(failedCompactMetrics, r.GetFailedCompactMetrics()...)
			}
		}
	}

	if len(metrics.TaggedMetrics) > 0 {
		if r, err = p.output.PutMetrics(ctx, &data_receiver.Metrics{
			DetailedResponse: metrics.DetailedResponse,
			TaggedMetrics:    metrics.TaggedMetrics,
		}); err != nil {
			failedTaggedMetricsCount += int32(len(metrics.TaggedMetrics))

			if metrics.DetailedResponse {
				for _, m := range metrics.TaggedMetrics {
					failedTaggedMetrics = append(failedTaggedMetrics, &data_receiver.TaggedMetricError{
						Error:  err.Error(),
						Metric: m,
					})
				}
			}
		} else {
			failedTaggedMetricsCount += r.GetFailed()
			succeededTaggedMetricsCount += r.GetSucceeded()

			if metrics.DetailedResponse {
				failedTaggedMetrics = append(failedTaggedMetrics, r.GetFailedTaggedMetrics()...)
			}
		}
	}

	if len(metrics.Metrics) > 0 {
		if r, err = p.output.PutMetrics(ctx, &data_receiver.Metrics{
			DetailedResponse: metrics.DetailedResponse,
			Metrics:          metrics.Metrics,
		}); err != nil {
			failedMetricsCount += int32(len(metrics.Metrics))

			if metrics.DetailedResponse {
				for _, m := range metrics.Metrics {
					failedMetrics = append(failedMetrics, &data_receiver.MetricError{
						Error:  err.Error(),
						Metric: m,
					})
				}
			}
		} else {
			failedMetricsCount += r.GetFailed()
			succeededMetricsCount += r.GetSucceeded()

			if metrics.DetailedResponse {
				failedMetrics = append(failedMetrics, r.GetFailedMetrics()...)
			}
		}
	}

	_ = stats.RecordWithTags(ctx, p.tagMutators,
		zstats.ReceivedCompactMetrics.M(int64(len(metrics.CompactMetrics))),
		zstats.SentCompactMetrics.M(int64(succeededCompactMetricsCount)),
		zstats.FailedCompactMetrics.M(int64(failedCompactMetricsCount)),
		zstats.ReceivedTaggedMetrics.M(int64(len(metrics.TaggedMetrics))),
		zstats.SentTaggedMetrics.M(int64(succeededTaggedMetricsCount)),
		zstats.FailedTaggedMetrics.M(int64(failedTaggedMetricsCount)),
		zstats.ReceivedMetrics.M(int64(len(metrics.Metrics))),
		zstats.SentMetrics.M(int64(succeededMetricsCount)),
		zstats.FailedMetrics.M(int64(failedMetricsCount)),
	)

	return &data_receiver.StatusResult{
		Failed:               failedCompactMetricsCount + failedTaggedMetricsCount + failedMetricsCount,
		Succeeded:            succeededCompactMetricsCount + succeededTaggedMetricsCount + succeededMetricsCount,
		FailedCompactMetrics: failedCompactMetrics,
		FailedTaggedMetrics:  failedTaggedMetrics,
		FailedMetrics:        failedMetrics,
	}, nil
}

// PutModels receives models via gRPC.
// Satisfies data_receiver.DataReceiverServiceServer interface.
func (p *Proxy) PutModels(ctx context.Context, models *data_receiver.Models) (*data_receiver.ModelStatusResult, error) {
	err := p.checkAPIKey(ctx)
	if err != nil {
		return nil, err
	}

	var failedModelsCount, succeededModelsCount int32
	var failedModels []*data_receiver.ModelError
	var r *data_receiver.ModelStatusResult

	if models.DetailedResponse {
		failedModels = make([]*data_receiver.ModelError, 0, len(models.Models))
	}

	if len(models.Models) > 0 {
		if r, err = p.output.PutModels(ctx, models); err != nil {
			failedModelsCount += int32(len(models.Models))

			if models.DetailedResponse {
				for _, m := range models.Models {
					failedModels = append(failedModels, &data_receiver.ModelError{
						Error: err.Error(),
						Model: m,
					})
				}
			}
		} else {
			failedModelsCount += r.GetFailed()
			succeededModelsCount += r.GetSucceeded()

			if models.DetailedResponse {
				failedModels = append(failedModels, r.GetFailedModels()...)
			}
		}
	}

	_ = stats.RecordWithTags(ctx, p.tagMutators,
		zstats.ReceivedModels.M(int64(len(models.Models))),
		zstats.SentModels.M(int64(succeededModelsCount)),
		zstats.FailedModels.M(int64(failedModelsCount)),
	)

	return &data_receiver.ModelStatusResult{
		Failed:       failedModelsCount,
		Succeeded:    succeededModelsCount,
		FailedModels: failedModels,
	}, nil
}

func (p *Proxy) checkAPIKey(ctx context.Context) error {
	if len(p.allowedAPIKeys) == 0 {
		// Specifying no AllowedAPIKeys means that no API key is required.
		return nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(
			codes.Unauthenticated,
			fmt.Sprintf("no metadata in context"))
	}

	apiKeys := md.Get(endpoint.APIKeyHeader)
	if len(apiKeys) == 0 {
		return status.Error(
			codes.Unauthenticated,
			fmt.Sprintf("no %q in context", endpoint.APIKeyHeader))
	}

	for _, apiKey := range apiKeys {
		if _, ok := p.allowedAPIKeys[apiKey]; ok {
			return nil
		}
	}

	return status.Error(codes.PermissionDenied, "unauthorized API key")
}

package endpoint

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"

	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	"google.golang.org/api/support/bundler"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	data_registry "github.com/zenoss/zenoss-protobufs/go/cloud/data-registry"
	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/cenkalti/backoff/v5"
	"github.com/jellydator/ttlcache/v3"
	"github.com/twmb/murmur3"
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

	// DefaultMinTTL is the default minimum TTL for local cache.
	// Overridden by Config.MinTTL.
	DefaultMinTTL = 6 * time.Hour

	// DefaultMaxTTL is the default maximum TTL for local cache.
	// Overridden by Config.MaxTTL.
	DefaultMaxTTL = 12 * time.Hour

	// DefaultInitialBackoff is the default initial backoff while retrying to register metrics.
	// Overriden by Config.InitialBackoff.
	DefaultInitialBackoff = 3 * time.Second

	// DefaultNumberOfRetries is the default number of retry attempts for the backoff strategy.
	// Overriden by Config.NumberOfRetries.
	DefaultNumberOfRetries = 3

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

	// TestRegClient is for testing only. Do not set for normal use.
	TestRegClient data_registry.DataRegistryServiceClient

	// Min TTL for local cache
	//
	// Default: 6 hour
	MinTTL int `yaml:"minTTL"`

	// Max TTL for local cache
	//
	// Default: 12 hour
	MaxTTL int `yaml:"maxTTL"`

	// CacheSizeLimit for local cache
	CacheSizeLimit int `yaml:"cacheSizeLimit"`

	// InitialBackoff for register retries.
	// Default: 3 seconds
	InitialBackoff time.Duration `yaml:"initialBackoff"`

	// NumberOfRetries for registering.
	// Default: 3 times
	NumberOfRetries uint `yaml:"NumberOfRetries"`

	// ExcludedTags are tags to exclude when generating hash
	ExcludedTags []string
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
	cache        *ttlcache.Cache[string, MetricIDNameAndHash]

	bundlers struct {
		events         *bundler.Bundler
		models         *bundler.Bundler
		metrics        *bundler.Bundler
		taggedMetrics  *bundler.Bundler
		compactMetrics *bundler.Bundler
	}
}

// MetricIDNameAndHash gives a metrics id, name, and hash
type MetricIDNameAndHash struct {
	hash, id, name string
}

type FailedRegistrationContext struct {
	Metric *data_receiver.Metric
	Index  int // an index of metric in the batch to preserve the order
	Error  string
}

// initCache initialises the ttl cache used to store metric ids
func initCache(cacheSizeLimit, minTTL, maxTTL int) *ttlcache.Cache[string, MetricIDNameAndHash] {
	return ttlcache.New[string, MetricIDNameAndHash](
		ttlcache.WithTTL[string, MetricIDNameAndHash](getCacheItemTTL(minTTL, maxTTL)),
		ttlcache.WithCapacity[string, MetricIDNameAndHash](uint64(cacheSizeLimit)),
		ttlcache.WithDisableTouchOnHit[string, MetricIDNameAndHash](),
	)
}

// getCacheItemTTL gets a ttl selected at random within max and min ttl limits
func getCacheItemTTL(minTTL int, maxTTL int) time.Duration {
	return time.Duration(getRandInRange(minTTL, maxTTL)) * time.Second
}

func getRandInRange(min, max int) int {
	return rand.Intn(max-min) + min
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

	if config.InitialBackoff == 0 {
		config.InitialBackoff = DefaultInitialBackoff
	}

	if config.NumberOfRetries == 0 {
		config.NumberOfRetries = DefaultNumberOfRetries
	}

	if config.LoggerConfig.Fields == nil {
		config.LoggerConfig.Fields = log.Fields{"endpoint": config.Name}
	}

	var client data_receiver.DataReceiverServiceClient
	var regclient data_registry.DataRegistryServiceClient

	if config.TestClient != nil {
		client = config.TestClient
		if config.TestRegClient != nil {
			regclient = config.TestRegClient
		}
	} else {
		dialOptions := make([]grpc.DialOption, 4)

		if config.DisableTLS {
			dialOptions[0] = grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			)
		} else {
			dialOptions[0] = grpc.WithTransportCredentials(
				credentials.NewTLS(
					&tls.Config{
						InsecureSkipVerify: config.InsecureTLS,
					},
				),
			)
		}

		// Enable gRPC retry middleware.
		retryOptions := []grpcretry.CallOption{
			grpcretry.WithMax(5),
			grpcretry.WithPerRetryTimeout(config.Timeout / 3),
			grpcretry.WithBackoff(grpcretry.BackoffLinear(200 * time.Millisecond)),
		}

		dialOptions[1] = grpc.WithUnaryInterceptor(grpcretry.UnaryClientInterceptor(retryOptions...))
		dialOptions[2] = grpc.WithStreamInterceptor(grpcretry.StreamClientInterceptor(retryOptions...))

		// Enable OpenCensus gRPC client stats.
		dialOptions[3] = grpc.WithStatsHandler(&ocgrpc.ClientHandler{})

		// Dial doesn't block by default. So no error can actually occur.
		conn, _ := grpc.Dial(config.Address, dialOptions...)
		client = data_receiver.NewDataReceiverServiceClient(conn)
		regclient = data_registry.NewDataRegistryServiceClient(conn)
	}

	var cache *ttlcache.Cache[string, MetricIDNameAndHash]
	if config.MinTTL <= 0 {
		config.MinTTL = int(DefaultMinTTL.Seconds())
	}
	if config.MaxTTL <= 0 {
		config.MaxTTL = int(DefaultMaxTTL.Seconds())
	}
	cache = initCache(config.CacheSizeLimit, config.MinTTL, config.MaxTTL)

	e := &Endpoint{
		config:       config,
		client:       client,
		regclient:    regclient,
		cache:        cache,
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
	e.bundlers.events = e.createBundler((*data_receiver.Event)(nil), func(bundle any) {
		e.putEvents(&data_receiver.Events{Events: bundle.([]*data_receiver.Event)})
	})

	e.bundlers.models = e.createBundler((*data_receiver.Model)(nil), func(bundle any) {
		e.putModels(&data_receiver.Models{Models: bundle.([]*data_receiver.Model)})
	})

	e.bundlers.metrics = e.createBundler((*data_receiver.Metric)(nil), func(bundle any) {
		e.putMetrics(&data_receiver.Metrics{Metrics: bundle.([]*data_receiver.Metric)})
	})

	e.bundlers.taggedMetrics = e.createBundler((*data_receiver.TaggedMetric)(nil), func(bundle any) {
		e.putMetrics(&data_receiver.Metrics{TaggedMetrics: bundle.([]*data_receiver.TaggedMetric)})
	})

	e.bundlers.compactMetrics = e.createBundler((*data_receiver.CompactMetric)(nil), func(bundle any) {
		e.putMetrics(&data_receiver.Metrics{CompactMetrics: bundle.([]*data_receiver.CompactMetric)})
	})
}

func (e *Endpoint) createBundler(itemExample any, handler func(any)) *bundler.Bundler {
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

// GetMetricCacheKey generates key to use for metric whose id is going to be cached
func (*Endpoint) GetMetricCacheKey(metric *data_receiver.Metric) string {
	dims := metric.Dimensions
	keys := make([]string, 0, len(dims))
	for k := range dims {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var flatDims strings.Builder
	for _, k := range keys {
		_, _ = fmt.Fprintf(&flatDims, "%s=%s,", k, dims[k])
	}

	localKey := []byte(fmt.Sprintf("%s:%s", metric.Metric, flatDims.String()))
	hash := murmur3.New128()
	hash.Write(localKey)
	v1, v2 := hash.Sum128()
	return strconv.FormatUint(v1, 10) + strconv.FormatUint(v2, 10)
}

// tagExcluded is called by GetMetadataHash
func tagExcluded(tag string, excludedTags []string) bool {
	if excludedTags == nil {
		return false
	}
	for _, s := range excludedTags {
		if s == tag {
			return true
		}
	}
	return false
}

// GetMetadataHash generates hash of metadata fields of the metric whose id is going to be cached
func (e *Endpoint) GetMetadataHash(metric *data_receiver.Metric) string {
	hasher := murmur3.New64()

	metadatafieldmap := metric.MetadataFields.GetFields()
	keys := make([]string, 0, len(metadatafieldmap))
	for k := range metadatafieldmap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		if tagExcluded(k, e.config.ExcludedTags) {
			continue
		}

		_, _ = fmt.Fprintf(&buf, "%s:%s", k, metadatafieldmap[k])
	}

	hasher.Write(buf.Bytes())

	return strconv.FormatUint(hasher.Sum64(), 10)
}

// checkCache checks the cache to see if metruc is already registered or needs update
func (e *Endpoint) checkCache(allmetrics []*data_receiver.Metric) ([]*data_receiver.CompactMetric, []*data_receiver.Metric, []string) {
	var (
		cachedCompctMetrics = make([]*data_receiver.CompactMetric, 0, len(allmetrics))
		misses              = make([]*data_receiver.Metric, 0, len(allmetrics))
		missKeys            = make([]string, 0, len(allmetrics))
	)
	for _, metric := range allmetrics {
		currentMetricCacheKey := e.GetMetricCacheKey(metric)

		cacheentry := e.cache.Get(currentMetricCacheKey)
		if cacheentry != nil {
			// cache hit, return metric with its id and name if metadata not changed
			currentMetadataHash := e.GetMetadataHash(metric)
			value := cacheentry.Value()
			if currentMetadataHash == value.hash {
				cachedCompctMetric := &data_receiver.CompactMetric{
					Id:        value.id,
					Timestamp: metric.Timestamp,
					Value:     metric.Value,
				}
				cachedCompctMetrics = append(cachedCompctMetrics, cachedCompctMetric)
			} else {
				log.Log(e, log.LevelInfo, log.Fields{"metric": metric}, "Metadata change on metric requires re-registry")
				misses = append(misses, metric)
				missKeys = append(missKeys, currentMetricCacheKey)
			}
		} else {
			misses = append(misses, metric)
			missKeys = append(missKeys, currentMetricCacheKey)
		}
	}
	return cachedCompctMetrics, misses, missKeys
}

// ConvertMetrics will convert canonical metrics to compact
func (e *Endpoint) ConvertMetrics(ctx context.Context, batch []*data_receiver.Metric) ([]*data_receiver.CompactMetric, []*data_receiver.Metric) {
	// cachedCompctMetrics - compact metrics with ids from cache
	// misses - canonical metrics which do not have an id in cache Or that need re-registration due to metadta chenge
	// missKeys - keys for caching the canonical metrics that are not currently in cache or need re-registration
	cachedCompctMetrics, misses, missKeys := e.checkCache(batch)
	failedMetrics := make([]*data_receiver.Metric, 0)
	log.Log(e, log.LevelInfo, log.Fields{"number of misses": len(misses)}, "Number of metrics misses in cache ")
	registeredcompactmetrics := make([]*data_receiver.CompactMetric, 0, len(misses))
	if len(misses) > 0 {
		successesAndFailures, metricIdsNamesAndHashes := e.registerMetrics(ctx, misses)
		if len(metricIdsNamesAndHashes) > 0 { // it is empty if the register call completely failed
			for i, ok := range successesAndFailures {
				if ok {
					metricIDNameAndHash := metricIdsNamesAndHashes[i]
					metric := misses[i]
					metricCacheKey := missKeys[i]
					zcm := &data_receiver.CompactMetric{
						Id:        metricIDNameAndHash.id,
						Timestamp: metric.Timestamp,
						Value:     metric.Value,
					}
					registeredcompactmetrics = append(registeredcompactmetrics, zcm)
					e.cache.Set(metricCacheKey, metricIDNameAndHash, getCacheItemTTL(e.config.MinTTL, e.config.MaxTTL))
				} else {
					failedMetrics = append(failedMetrics, misses[i])
				}
			}
		} else {
			failedMetrics = misses
		}
		log.Log(e, log.LevelInfo, log.Fields{"len failed metrics": len(failedMetrics)}, "Number of metrics failed to convert ")
	}
	if len(registeredcompactmetrics) > 0 {
		cachedCompctMetrics = append(cachedCompctMetrics, registeredcompactmetrics...)
		log.Log(e, log.LevelInfo, log.Fields{"len converted metrics": len(cachedCompctMetrics)}, "Number of metrics converted ")
	}

	return cachedCompctMetrics, failedMetrics
}

// registerMetrics will register metric using dataRegistry
func (e *Endpoint) registerMetrics(ctx context.Context, metrics []*data_receiver.Metric) ([]bool, []MetricIDNameAndHash) {
	successes := make([]bool, len(metrics))
	metricIDsNamesAndHashes := make([]MetricIDNameAndHash, len(metrics))

	expBoff := backoff.NewExponentialBackOff()
	expBoff.InitialInterval = e.config.InitialBackoff
	registerMetricsresponse, err := backoff.Retry(
		ctx,
		func() (*data_registry.RegisterMetricsResponse, error) {
			return e.CreateOrUpdateMetrics(ctx, metrics)
		},
		backoff.WithBackOff(expBoff),
		backoff.WithMaxTries(e.config.NumberOfRetries),
	)
	if err != nil {
		log.Log(e, log.LevelError, log.Fields{"error": err}, "Unable to register metrics")
		return successes, []MetricIDNameAndHash{}
	}

	failedMetrics := make([]*FailedRegistrationContext, 0)

	for i, response := range registerMetricsresponse.Responses {
		if response.Error != "" {
			failedMetrics = append(failedMetrics, &FailedRegistrationContext{
				Metric: metrics[i],
				Index:  i,
				Error:  response.Error,
			})
			continue
		}
		metricMetadataHash := e.GetMetadataHash(metrics[i])
		metricIDsNamesAndHashes[i] = MetricIDNameAndHash{
			id:   response.Response.InstanceId,
			name: response.Response.Name,
			hash: metricMetadataHash,
		}
		successes[i] = true
	}

	if len(failedMetrics) > 0 {
		// retrying to register failed metrics from batch
		expBoff.Reset()
		metricsToRetry := extractMetrics(failedMetrics)
		retryResponse, err := backoff.Retry(
			ctx,
			func() (*data_registry.RegisterMetricsResponse, error) {
				return e.CreateOrUpdateMetrics(ctx, metricsToRetry)
			},
			backoff.WithBackOff(expBoff),
			backoff.WithMaxTries(e.config.NumberOfRetries),
		)
		if err != nil {
			for _, failedMetric := range failedMetrics {
				log.Log(e, log.LevelError, log.Fields{"error": failedMetric.Error}, "Metric from batch was not registered")
			}
			return successes, metricIDsNamesAndHashes
		}
		for j, retryResp := range retryResponse.Responses {
			originalIndex := failedMetrics[j].Index
			if retryResp.Error == "" {
				successes[originalIndex] = true

				metricMetadataHash := e.GetMetadataHash(failedMetrics[j].Metric)
				metricIDsNamesAndHashes[originalIndex] = MetricIDNameAndHash{
					id:   retryResp.Response.InstanceId,
					name: retryResp.Response.Name,
					hash: metricMetadataHash,
				}
			} else {
				log.Log(e, log.LevelError, log.Fields{"error": retryResp.Error}, "Metric from batch was not registered")
			}
		}
	}
	return successes, metricIDsNamesAndHashes
}

// CreateOrUpdateMetrics uses DataRegistryService CreateOrUpdateMetrics to create or update metrics
func (e *Endpoint) CreateOrUpdateMetrics(ctx context.Context, metrics []*data_receiver.Metric) (*data_registry.RegisterMetricsResponse, error) {
	var failedRegistrationCount, succeededRegistrationsCount int32
	if e.config.APIKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, APIKeyHeader, e.config.APIKey)
	}
	stream, err := e.regclient.CreateOrUpdateMetrics(ctx, grpcretry.Disable())
	if err != nil {
		return nil, err
	}
	for _, metric := range metrics {
		metricWrapper := &data_receiver.MetricWrapper{
			MetricType: &data_receiver.MetricWrapper_Canonical{Canonical: metric},
		}
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
				zstats.FailedRegistrations.M(int64(len(metrics))),
			)
			return nil, err
		}
	}
	registerResponse, err := stream.CloseAndRecv()
	if err != nil {
		log.Log(e, log.LevelError, log.Fields{}, "Error closing stream")
		_ = stats.RecordWithTags(ctx, e.tagMutators,
			zstats.SucceededRegistrations.M(int64(succeededRegistrationsCount)),
			zstats.FailedRegistrations.M(int64(len(metrics))),
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
func (e *Endpoint) PutEvents(ctx context.Context, events *data_receiver.Events, _ ...grpc.CallOption) (*data_receiver.EventStatusResult, error) {
	var failedEventsCount, succeededEventsCount int32
	var failedEvents []*data_receiver.EventError

	if events.DetailedResponse {
		failedEvents = make([]*data_receiver.EventError, 0, len(events.Events))
	}

	for _, event := range events.Events {
		err := e.bundlers.events.Add(event, 1)
		if err != nil {
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
func (*Endpoint) PutEvent(_ context.Context, _ ...grpc.CallOption) (data_receiver.DataReceiverService_PutEventClient, error) {
	return nil, status.Error(codes.Unimplemented, "PutEvent is not supported")
}

// PutMetric implements DataReceiverService PutMetric streaming RPC.
func (*Endpoint) PutMetric(context.Context, ...grpc.CallOption) (data_receiver.DataReceiverService_PutMetricClient, error) {
	return nil, status.Error(codes.Unimplemented, "PutMetric is not supported")
}

// PutMetrics implements DataReceiverService PutMetrics unary RPC.
//
//revive:disable:cognitive-complexity
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

//revive:enable:cognitive-complexity

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

func extractMetrics(failedMetrics []*FailedRegistrationContext) []*data_receiver.Metric {
	metrics := make([]*data_receiver.Metric, len(failedMetrics))
	for i, failedMetric := range failedMetrics {
		metrics[i] = failedMetric.Metric
	}
	return metrics
}

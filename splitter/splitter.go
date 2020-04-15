package splitter

import (
	"context"
	"fmt"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/log"
	zstats "github.com/zenoss/zenoss-go-sdk/stats"
)

var (
	// Ensure Splitter implements DataReceiverServiceClient interface.
	_ data_receiver.DataReceiverServiceClient = (*Splitter)(nil)

	// Ensure Splitter implements log.Logger interface.
	_ log.Logger = (*Splitter)(nil)
)

// Config specifies a splitter's configuration.
type Config struct {
	// Name is an arbitrary name for the splitter for disambiguation of multiple splitters.
	// Optional.
	Name string `yaml:"name"`

	// Outputs are clients to which data received by the splitter will be sent.
	// Typical output types are *endpoint.Endpoint and *processor.Processor.
	// Required.
	Outputs []data_receiver.DataReceiverServiceClient

	// LoggerConfig specifies the logging configuration.
	// Optional.
	LoggerConfig log.LoggerConfig `yaml:"logger"`
}

// Splitter accepts Zenoss data and sends it to multiple Zenoss API endpoints.
type Splitter struct {
	config      Config
	outputs     []data_receiver.DataReceiverServiceClient
	tagMutators []tag.Mutator
}

// New returns a new Splitter with specified configuration.
func New(config Config) (*Splitter, error) {
	if config.Name == "" {
		config.Name = "default"
	}

	if config.Outputs == nil || len(config.Outputs) == 0 {
		return nil, fmt.Errorf("Config.Outputs is empty")
	}

	if config.LoggerConfig.Fields == nil {
		config.LoggerConfig.Fields = log.Fields{"splitter": config.Name}
	}

	s := &Splitter{
		config:  config,
		outputs: config.Outputs,
		tagMutators: []tag.Mutator{
			tag.Upsert(zstats.KeyModuleType, "splitter"),
			tag.Upsert(zstats.KeyModuleName, config.Name),
		},
	}

	return s, nil
}

// GetLoggerConfig returns logging configuration.
// Satisfies log.Logger interface.
func (s *Splitter) GetLoggerConfig() log.LoggerConfig {
	return s.config.LoggerConfig
}

// PutMetric implements DataReceiverService PutMetric streaming RPC.
func (s *Splitter) PutMetric(context.Context, ...grpc.CallOption) (data_receiver.DataReceiverService_PutMetricClient, error) {
	return nil, status.Error(codes.Unimplemented, "PutMetric is not supported")
}

// PutMetrics implements DataReceiverService PutMetrics unary RPC.
func (s *Splitter) PutMetrics(ctx context.Context, metrics *data_receiver.Metrics, opts ...grpc.CallOption) (*data_receiver.StatusResult, error) {
	var failedCompactMetricsCount, succeededCompactMetricsCount int32
	var failedCompactMetrics []*data_receiver.CompactMetricError
	var failedTaggedMetricsCount, succeededTaggedMetricsCount int32
	var failedTaggedMetrics []*data_receiver.TaggedMetricError
	var failedMetricsCount, succeededMetricsCount int32
	var failedMetrics []*data_receiver.MetricError
	var err error
	var r *data_receiver.StatusResult

	if metrics.DetailedResponse {
		failedCompactMetrics = make([]*data_receiver.CompactMetricError, 0, len(metrics.CompactMetrics)*len(s.outputs))
		failedTaggedMetrics = make([]*data_receiver.TaggedMetricError, 0, len(metrics.TaggedMetrics)*len(s.outputs))
		failedMetrics = make([]*data_receiver.MetricError, 0, len(metrics.Metrics)*len(s.outputs))
	}

	for _, output := range s.outputs {
		if len(metrics.CompactMetrics) > 0 {
			if r, err = output.PutMetrics(ctx, &data_receiver.Metrics{
				DetailedResponse: metrics.DetailedResponse,
				CompactMetrics:   metrics.CompactMetrics,
			}, opts...); err != nil {
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
			if r, err = output.PutMetrics(ctx, &data_receiver.Metrics{
				DetailedResponse: metrics.DetailedResponse,
				TaggedMetrics:    metrics.TaggedMetrics,
			}, opts...); err != nil {
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
			if r, err = output.PutMetrics(ctx, &data_receiver.Metrics{
				DetailedResponse: metrics.DetailedResponse,
				Metrics:          metrics.Metrics,
			}, opts...); err != nil {
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
	}

	_ = stats.RecordWithTags(ctx, s.tagMutators,
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

// PutModels implements DataReceiverService PutModels unary RPC.
func (s *Splitter) PutModels(ctx context.Context, models *data_receiver.Models, opts ...grpc.CallOption) (*data_receiver.ModelStatusResult, error) {
	var failedModelsCount, succeededModelsCount int32
	var failedModels []*data_receiver.ModelError
	var err error
	var r *data_receiver.ModelStatusResult

	if models.DetailedResponse {
		failedModels = make([]*data_receiver.ModelError, 0, len(models.Models)*len(s.outputs))
	}

	for _, output := range s.outputs {
		if len(models.Models) > 0 {
			if r, err = output.PutModels(ctx, models, opts...); err != nil {
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
	}

	_ = stats.RecordWithTags(ctx, s.tagMutators,
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

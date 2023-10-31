package processor

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
	// Ensure Processor implements DataReceiverServiceClient interface.
	_ data_receiver.DataReceiverServiceClient = (*Processor)(nil)

	// Ensure Processor implements log.Logger interface.
	_ log.Logger = (*Processor)(nil)

	// Ensure signal types implement error interface.
	_ error = (*SignalDrop)(nil)
	_ error = (*SignalSend)(nil)
)

// Config specifies a processor's configuration.
type Config struct {
	// Name is an arbitrary name for the endpoint for disambiguation of multiple processors.
	// Optional.
	Name string `yaml:"name"`

	// MetricRules are processing rules for metrics.
	MetricRules []MetricRuleConfig `yaml:"metricRules"`

	// TaggedMetricRules are processing rules for tagged metrics.
	TaggedMetricRules []TaggedMetricRuleConfig `yaml:"taggedMetricRules"`

	// ModelRules are processing rules for models.
	ModelRules []ModelRuleConfig `yaml:"modelRules"`

	// LoggerConfig specifies the logging configuration.
	// Optional.
	LoggerConfig log.LoggerConfig `yaml:"logger"`

	// Output is a client to which the processor will send processed data.
	// Typical output types are *endpoint.Endpoint and *processor.Processor.
	// Required.
	Output data_receiver.DataReceiverServiceClient
}

// Processor represents a data (metric, model) processing engine.
type Processor struct {
	config      Config
	output      data_receiver.DataReceiverServiceClient
	tagMutators []tag.Mutator
}

// New returns new processor with specified configuration.
func New(config Config) (*Processor, error) {
	if config.Name == "" {
		config.Name = "default"
	}

	if config.Output == nil {
		return nil, fmt.Errorf("Config.Output is nil")
	}

	if config.LoggerConfig.Fields == nil {
		config.LoggerConfig.Fields = log.Fields{"processor": config.Name}
	}

	p := &Processor{
		config: config,
		output: config.Output,
		tagMutators: []tag.Mutator{
			tag.Upsert(zstats.KeyModuleType, "processor"),
			tag.Upsert(zstats.KeyModuleName, config.Name),
		},
	}

	return p, nil
}

// GetLoggerConfig returns logging configuration.
// Satisfies log.Logger interface.
func (p *Processor) GetLoggerConfig() log.LoggerConfig {
	return p.config.LoggerConfig
}

// PutEvent implements DataReceiverService PutEvent streaming RPC.
func (*Processor) PutEvent(_ context.Context, _ ...grpc.CallOption) (data_receiver.DataReceiverService_PutEventClient, error) {
	return nil, status.Error(codes.Unimplemented, "PutEvent is not supported")
}

// PutEvents implements DataReceiverService PutEvents unary RPC.
func (*Processor) PutEvents(_ context.Context, _ *data_receiver.Events, _ ...grpc.CallOption) (*data_receiver.EventStatusResult, error) {
	// TODO: impelement event processor rules
	return nil, status.Error(codes.Unimplemented, "PutEvents is not supported")
}

// PutMetric implements DataReceiverService PutMetric streaming RPC.
func (*Processor) PutMetric(context.Context, ...grpc.CallOption) (data_receiver.DataReceiverService_PutMetricClient, error) {
	return nil, status.Error(codes.Unimplemented, "PutMetric is not supported")
}

// PutMetrics implements DataReceiverService PutMetrics unary RPC.
//
//revive:disable:cognitive-complexity
func (p *Processor) PutMetrics(ctx context.Context, metrics *data_receiver.Metrics, _ ...grpc.CallOption) (*data_receiver.StatusResult, error) {
	var failedCompactMetricsCount, succeededCompactMetricsCount int32
	var failedCompactMetrics []*data_receiver.CompactMetricError
	var failedTaggedMetricsCount, succeededTaggedMetricsCount int32
	var failedTaggedMetrics []*data_receiver.TaggedMetricError
	var failedMetricsCount, succeededMetricsCount int32
	var failedMetrics []*data_receiver.MetricError
	var err error
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

	for _, taggedMetric := range metrics.TaggedMetrics {
		if err = p.processTaggedMetric(ctx, taggedMetric); err != nil {
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

	for _, metric := range metrics.Metrics {
		if err = p.processMetric(ctx, metric); err != nil {
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

//revive:enable:cognitive-complexity

// PutModels implements DataReceiverService PutModels unary RPC.
func (p *Processor) PutModels(ctx context.Context, models *data_receiver.Models, _ ...grpc.CallOption) (*data_receiver.ModelStatusResult, error) {
	var failedModelsCount, succeededModelsCount int32
	var failedModels []*data_receiver.ModelError
	var err error

	if models.DetailedResponse {
		failedModels = make([]*data_receiver.ModelError, 0, len(models.Models))
	}

	for _, model := range models.Models {
		if err = p.processModel(ctx, model); err != nil {
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

func (p *Processor) processMetric(ctx context.Context, metric *data_receiver.Metric) error {
loop:
	for _, r := range p.config.MetricRules {
		switch err := r.Apply(ctx, p, metric); err.(type) {
		case *SignalDrop:
			return nil
		case *SignalSend:
			break loop
		case nil:
			continue loop
		default:
			log.Error(p, log.Fields{"error": err}, "metric error")
			return err
		}
	}

	_, err := p.output.PutMetrics(ctx, &data_receiver.Metrics{
		Metrics: []*data_receiver.Metric{metric},
	})

	return err
}

func (p *Processor) processTaggedMetric(ctx context.Context, taggedMetric *data_receiver.TaggedMetric) error {
loop:
	for _, r := range p.config.TaggedMetricRules {
		switch err := r.Apply(ctx, p, taggedMetric); err.(type) {
		case *SignalDrop:
			return nil
		case *SignalSend:
			break loop
		case nil:
			continue loop
		default:
			log.Error(p, log.Fields{"error": err}, "tagged metric error")
			return err
		}
	}

	_, err := p.output.PutMetrics(ctx, &data_receiver.Metrics{
		TaggedMetrics: []*data_receiver.TaggedMetric{taggedMetric},
	})

	return err
}

func (p *Processor) processModel(ctx context.Context, model *data_receiver.Model) error {
loop:
	for _, r := range p.config.ModelRules {
		switch err := r.Apply(ctx, p, model); err.(type) {
		case *SignalDrop:
			return nil
		case *SignalSend:
			break loop
		case nil:
			continue loop
		default:
			log.Error(p, log.Fields{"error": err}, "model error")
			return err
		}
	}

	_, err := p.output.PutModels(ctx, &data_receiver.Models{
		Models: []*data_receiver.Model{model},
	})

	return err
}

// SignalDrop is returned by an action type when the data should be dropped.
type SignalDrop struct{}

// Error exists solely to satisfy the error interface.
func (*SignalDrop) Error() string { return "drop" }

// SignalSend is returned by an action type when the data should be immediately sent.
type SignalSend struct{}

// Error exists solely to satisfy the error interface.
func (*SignalSend) Error() string { return "send" }

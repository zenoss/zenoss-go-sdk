package writer

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/zenoss/zenoss-go-sdk/endpoint"
	"github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
	endpointLog "github.com/zenoss/zenoss-go-sdk/log"
	sdk_utils "github.com/zenoss/zenoss-go-sdk/utils"
	zpb "github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"
	"google.golang.org/protobuf/types/known/structpb"
)

// NewZCDestination creates a new ZCDestination instance
// any config should be handled through ZCDestinationConfig parameter
func NewZCDestination(config *ZCDestinationConfig) (*ZCDestination, error) {
	epConf := config.EndpointConfig

	epConf.LoggerConfig.Func = logFn()

	ep, err := endpoint.New(*epConf)
	if err != nil {
		log.GetLogger().Error().Msgf("zenoss endpoint creation error %+v", err)
		return nil, err
	}

	if config.UseCompact &&
		(config.EndpointConfig.MinTTL == 0 || config.EndpointConfig.MaxTTL == 0) {
		log.GetLogger().Error().Msg("not all cache configs set. Disabling compact metrics")
		config.UseCompact = false
	}

	return &ZCDestination{Endpoint: ep, Config: config}, nil
}

// ZCDestinationConfig keeps required configuration for ZC destination
// It keeps endpoint config with your API key, endpoint address, etc.
// It also keeps some data related configs like additional metadata
type ZCDestinationConfig struct {
	// EndpointConfig have all required data that we need to use ZC cloud endpoint.
	// Follow endpoint.Config docstrings for additional details
	EndpointConfig *endpoint.Config

	// UseCompact defines whether to send metrics in compact or canonical format
	UseCompact bool

	// SourceType should allows us to categorize different types of monitored staff
	// We also can define different ways to pre-process health data for different source types in future
	SourceType string
	// SourceName allows us to split your monitored staff as unique entities if you have a few
	SourceName string
	// Metadata that will be added to you datapoints.
	// There is also some metadata that will be present in datapoint by default: source-type
	Metadata map[string]string
}

// ZCDestination outputs health data to zenoss cloud endpoint
type ZCDestination struct {
	Endpoint *endpoint.Endpoint
	Config   *ZCDestinationConfig
}

// Register takes target, builds model and pushes it to preconfigured ZC endpoint
func (d *ZCDestination) Register(ctx context.Context, target *target.Target) error {
	model := &zpb.Model{}
	model.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	model.Dimensions = d.buildTargetDimensions(target.ID)
	model.MetadataFields = d.buildTargetMetadata()
	model.MetadataFields.Fields[utils.ZenossNameField] = sdk_utils.StrToStructValue(target.ID)

	result, err := d.Endpoint.PutModels(ctx, &zpb.Models{Models: []*zpb.Model{model}})
	if err != nil {
		return err
	}

	log.GetLogger().Debug().Msgf("Models push statistic: succeed: %d, failed: %d", result.Succeeded, result.Failed)
	return nil
}

// Push takes health, builds metrics and pushes them to preconfigured ZC endpoint
func (d *ZCDestination) Push(ctx context.Context, health *target.Health) error {
	canonicalMetrics := d.buildCanonicalMetrics(health)

	var cmpMetrics []*zpb.CompactMetric
	if d.Config.UseCompact {
		compactMetrics, failedMetrics := d.Endpoint.ConvertMetrics(ctx, canonicalMetrics)
		cmpMetrics = compactMetrics
		if len(failedMetrics) > 0 {
			log.GetLogger().Debug().Msgf("Failed to register canonical metrics: %v", failedMetrics)
		}
		canonicalMetrics = failedMetrics
	}

	result, err := d.Endpoint.PutMetrics(ctx, &zpb.Metrics{
		Metrics:        canonicalMetrics,
		CompactMetrics: cmpMetrics,
	})
	if err != nil {
		return err
	}
	log.GetLogger().Debug().Msgf("Metrics push statistic: succeed: %d, failed: %d", result.Succeeded, result.Failed)
	return nil
}

func (d *ZCDestination) buildCanonicalMetrics(health *target.Health) []*zpb.Metric {
	metrics := make([]*zpb.Metric, 0)
	for mID, mValue := range health.Metrics {
		metrics = append(metrics, d.buildMetric(
			health.TargetID, mID, mValue,
		))
	}

	for cID, cValue := range health.Counters {
		metrics = append(metrics, d.buildMetric(
			health.TargetID, cID, float64(cValue),
		))
	}

	if health.Heartbeat.Enabled {
		value := float64(0)
		if health.Heartbeat.Beats {
			value = 1
		}
		metrics = append(metrics, d.buildMetric(health.TargetID, utils.HeartBeatMetricName, value))
	}

	return metrics
}

func (d *ZCDestination) buildMetric(targetID, metricID string, value float64) *zpb.Metric {
	metric := &zpb.Metric{}
	metric.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
	metric.Metric = metricID
	metric.Value = value
	metric.Dimensions = d.buildTargetDimensions(targetID)
	metric.MetadataFields = d.buildTargetMetadata()
	return metric
}

func (d *ZCDestination) buildTargetDimensions(targetID string) map[string]string {
	dims := make(map[string]string)
	dims[utils.TargetKey] = targetID
	if d.Config.SourceType == "" {
		dims[utils.SourceTypeKey] = utils.DefaultSourceType
	} else {
		dims[utils.SourceTypeKey] = d.Config.SourceType
	}
	dims[utils.SourceKey] = d.Config.SourceName
	return dims
}

func (d *ZCDestination) buildTargetMetadata() *structpb.Struct {
	metadata := make(map[string]*structpb.Value)
	for key, value := range d.Config.Metadata {
		metadata[key] = sdk_utils.StrToStructValue(value)
	}
	return &structpb.Struct{Fields: metadata}
}

func logFn() endpointLog.Func {
	return func(level endpointLog.Level, fields endpointLog.Fields, format string, args ...any) {
		l := log.GetLogger()
		var e *zerolog.Event

		switch level {
		case endpointLog.LevelDebug:
			e = l.Debug()
		case endpointLog.LevelInfo:
			e = l.Info()
		case endpointLog.LevelWarning:
			e = l.Warn()
		case endpointLog.LevelError:
			e = l.Error()
		default:
			e = l.Log()
		}
		e.Fields(fields)
		e.Msgf(format, args...)
	}
}

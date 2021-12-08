package processor

import (
	"context"
	"fmt"

	_struct "google.golang.org/protobuf/types/known/structpb"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/log"
)

const (
	// ActionTypeNoop is the "noop" action type.
	// It does nothing to the matched item. It is the same as not having the action.
	ActionTypeNoop = "noop"

	// ActionTypeLog is the "log" action type.
	// It logs the matched item.
	ActionTypeLog = "log"

	// ActionTypeDrop is the "drop" action type.
	// It stops processing the matched item, and prevents it from being sent.
	ActionTypeDrop = "drop"

	// ActionTypeSend is the "send" action type.
	// It immediately sends the matched item, and prevents any further processing.
	ActionTypeSend = "send"

	// ActionTypeAppendToMetadataField is the "append-to-metadata-field" action type.
	// It appends a value to a list-type metadata field on a metric or model.
	ActionTypeAppendToMetadataField = "append-to-metadata-field"

	// ActionTypeCreateModel is the "create-model" action type.
	// It creates a new model based on the matched metric.
	// The new model is then processed according to the model rules.
	ActionTypeCreateModel = "create-model"

	// ActionTypeCopyToMetric is the "copy-to-metric" action type.
	// It copies the contents of a tagged metric to a new metric.
	// The new metric is then processed according to the metric rules.
	ActionTypeCopyToMetric = "copy-to-metric"
)

// ActionOptions is a container for ad hoc options for any action type.
type ActionOptions map[string]interface{}

// MetricActionConfig specifies a metric action.
type MetricActionConfig struct {
	Name    string
	Type    string
	Options ActionOptions
}

// GetName returns name for the MetricActionConfig.
func (a *MetricActionConfig) GetName() string {
	if a.Name == "" {
		return "default"
	}

	return a.Name
}

// String returns a string representation of the MetricActionConfig.
// Satisfies fmt.Stringer interface.
func (a *MetricActionConfig) String() string {
	return fmt.Sprintf("%s (%s)", a.GetName(), a.Type)
}

// Apply applies the MetricActionConfig to a metric.
func (a *MetricActionConfig) Apply(ctx context.Context, p *Processor, metric *data_receiver.Metric) error {
	switch a.Type {
	case ActionTypeNoop:
		return nil
	case ActionTypeLog:
		log.Log(p, log.LevelInfo, log.Fields{"action": a.GetName()}, metricString(metric))
	case ActionTypeDrop:
		return &SignalDrop{}
	case ActionTypeSend:
		return &SignalSend{}
	case ActionTypeAppendToMetadataField:
		action, err := NewActionAppendToMetadataField(a.Options)
		if err != nil {
			return err
		}

		if err := action.ApplyToMetric(metric); err != nil {
			return err
		}
	case ActionTypeCreateModel:
		action, err := NewActionCreateModel(a.Options)
		if err != nil {
			return err
		}

		model, err := action.ApplyToMetric(metric)
		if err != nil {
			return err
		}

		return p.processModel(ctx, model)
	default:
		return fmt.Errorf("unknown metric action type: %s", a)
	}

	return nil
}

// TaggedMetricActionConfig specifies a tagged metric action.
type TaggedMetricActionConfig struct {
	Name    string
	Type    string
	Options map[string]interface{}
}

// GetName returns name for the TaggedMetricActionConfig.
func (a *TaggedMetricActionConfig) GetName() string {
	if a.Name == "" {
		return "default"
	}

	return a.Name
}

// String returns a string representation of the TaggedMetricActionConfig.
// Satisfies the fmt.Stringer interface.
func (a *TaggedMetricActionConfig) String() string {
	return fmt.Sprintf("%s (%s)", a.GetName(), a.Type)
}

// Apply applies the TaggedMetricActionConfig to a tagged metric.
func (a *TaggedMetricActionConfig) Apply(ctx context.Context, p *Processor, taggedMetric *data_receiver.TaggedMetric) error {
	switch a.Type {
	case ActionTypeNoop:
		return nil
	case ActionTypeLog:
		log.Log(p, log.LevelInfo, log.Fields{"action": a.GetName()}, taggedMetricString(taggedMetric))
	case ActionTypeDrop:
		return &SignalDrop{}
	case ActionTypeSend:
		return &SignalSend{}
	case ActionTypeCopyToMetric:
		action, err := NewActionCopyToMetric(a.Options)
		if err != nil {
			return err
		}

		return p.processMetric(ctx, action.ApplyToTaggedMetric(taggedMetric))
	default:
		return fmt.Errorf("unknown tagged metric action type: %s", a)
	}

	return nil
}

// ModelActionConfig specifies a model action.
type ModelActionConfig struct {
	Name    string
	Type    string
	Options map[string]interface{}
}

// GetName returns name for the ModelActionConfig.
func (a *ModelActionConfig) GetName() string {
	if a.Name == "" {
		return "default"
	}

	return a.Name
}

// String returns a string representation of the ModelActionConfig.
// Satisfies the fmt.Stringer interface.
func (a *ModelActionConfig) String() string {
	return fmt.Sprintf("%s (%s)", a.GetName(), a.Type)
}

// Apply applies the ModelActionConfig to a model.
func (a *ModelActionConfig) Apply(_ context.Context, p *Processor, model *data_receiver.Model) error {
	switch a.Type {
	case ActionTypeNoop:
		return nil
	case ActionTypeLog:
		log.Log(p, log.LevelInfo, log.Fields{"action": a.GetName()}, modelString(model))
	case ActionTypeDrop:
		return &SignalDrop{}
	case ActionTypeSend:
		return &SignalSend{}
	case ActionTypeAppendToMetadataField:
		action, err := NewActionAppendToMetadataField(a.Options)
		if err != nil {
			return err
		}

		if err := action.ApplyToModel(model); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown model action type: %s", a)
	}

	return nil
}

func metricString(metric *data_receiver.Metric) string {
	return fmt.Sprintf(
		"Metric{Metric: %q, Dimensions: %v, MetadataFields: %v}",
		metric.Metric,
		metric.Dimensions,
		metadataFieldsString(metric.MetadataFields))
}

func taggedMetricString(taggedMetric *data_receiver.TaggedMetric) string {
	return fmt.Sprintf(
		"TaggedMetric{Metric: %q, Tags: %v}",
		taggedMetric.Metric,
		taggedMetric.Tags)
}

func modelString(model *data_receiver.Model) string {
	return fmt.Sprintf(
		"Model{Dimensions: %v, MetadataFields: %v}",
		model.Dimensions,
		metadataFieldsString(model.MetadataFields))
}

func metadataFieldsString(metadataFields *_struct.Struct) string {
	m := make(map[string]interface{}, len(metadataFields.GetFields()))
	for k, v := range metadataFields.GetFields() {
		m[k] = unrollValue(v)
	}

	return fmt.Sprintf("%v", m)
}

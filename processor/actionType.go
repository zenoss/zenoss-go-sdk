package processor

import (
	"fmt"
	"strings"

	"github.com/cbroglie/mustache"
	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/mitchellh/mapstructure"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/metadata"
)

// ActionCreateModel specifies the options for a "create-model" action.
type ActionCreateModel struct {
	NameTemplate       string
	ParsedNameTemplate *mustache.Template
	DropMetadataKeys   []string
}

// NewActionCreateModel returns a new "create-model" action.
// It also performs static validation of the options.
func NewActionCreateModel(options ActionOptions) (*ActionCreateModel, error) {
	var a ActionCreateModel
	if err := mapstructure.Decode(options, &a); err != nil {
		return nil, err
	}

	if a.NameTemplate != "" {
		r, err := mustache.ParseString(a.NameTemplate)
		if err != nil {
			return nil, err
		}

		a.ParsedNameTemplate = r
	}

	return &a, nil
}

// ApplyToMetric applies a "create-model" action to a metric.
func (a *ActionCreateModel) ApplyToMetric(metric *data_receiver.Metric) (*data_receiver.Model, error) {
	dropMetadataKeysMap := make(map[string]interface{}, len(a.DropMetadataKeys))
	for _, key := range a.DropMetadataKeys {
		dropMetadataKeysMap[key] = nil
	}

	dimensions := make(map[string]string)
	for k, v := range metric.Dimensions {
		dimensions[k] = v
	}

	metadataFields := metadata.FromStringMap(nil)
	for k, v := range metric.GetMetadataFields().GetFields() {
		if _, ok := dropMetadataKeysMap[k]; !ok {
			metadataFields.Fields[k] = v
		}
	}

	if a.ParsedNameTemplate != nil {
		mustache.AllowMissingVariables = false
		r, err := a.ParsedNameTemplate.Render(getTemplateContext(metric))
		if err != nil {
			return nil, err
		}

		metadataFields.Fields["name"] = metadata.FromString(r)
	}

	return &data_receiver.Model{
		Timestamp:      metric.Timestamp,
		Dimensions:     dimensions,
		MetadataFields: metadataFields,
	}, nil
}

// ActionAppendToMetadataField specifies the options for an "append-to-metadata-field" action.
type ActionAppendToMetadataField struct {
	Key                 string
	Value               string
	ValueTemplate       string
	ParsedValueTemplate *mustache.Template
}

// NewActionAppendToMetadataField returns a new "append-to-metadata-field" action.
// It also performs static validation of the options.
func NewActionAppendToMetadataField(options ActionOptions) (*ActionAppendToMetadataField, error) {
	var a ActionAppendToMetadataField
	if err := mapstructure.Decode(options, &a); err != nil {
		return nil, err
	}

	if a.Key == "" {
		return nil, fmt.Errorf(`missing "key"`)
	}

	if a.ValueTemplate != "" {
		parsed, err := mustache.ParseString(a.ValueTemplate)
		if err != nil {
			return nil, err
		}

		a.ParsedValueTemplate = parsed
	}

	if a.Value == "" && a.ParsedValueTemplate == nil {
		return nil, fmt.Errorf(`missing "value" or "valueTemplate"`)
	}

	if a.Value != "" && a.ParsedValueTemplate != nil {
		return nil, fmt.Errorf(`setting "value" and "valueTemplate" is ambiguous`)
	}

	return &a, nil
}

// ApplyToMetric applies a "append-to-metadata-field" action to a metric.
func (a *ActionAppendToMetadataField) ApplyToMetric(metric *data_receiver.Metric) error {
	metadataFields, err := a.applyToMetadataFields(
		metric.GetMetadataFields(),
		getTemplateContext(metric))

	metric.MetadataFields = metadataFields

	return err
}

// ApplyToModel applies a "append-to-metadata-field" action to a model.
func (a *ActionAppendToMetadataField) ApplyToModel(model *data_receiver.Model) error {
	metadataFields, err := a.applyToMetadataFields(
		model.GetMetadataFields(),
		getTemplateContext(model))

	model.MetadataFields = metadataFields

	return err
}

func (a *ActionAppendToMetadataField) applyToMetadataFields(metadataFields *_struct.Struct, templateContext interface{}) (*_struct.Struct, error) {
	if metadataFields == nil {
		metadataFields = metadata.FromStringMap(nil)
	}

	var value string
	if a.Value != "" {
		value = a.Value
	} else if a.ParsedValueTemplate != nil {
		mustache.AllowMissingVariables = false
		r, err := a.ParsedValueTemplate.Render(templateContext)
		if err != nil {
			return metadataFields, err
		}

		value = r
	}

	if pValue, ok := metadataFields.GetFields()[a.Key]; ok {
		switch v := pValue.Kind.(type) {
		case *_struct.Value_ListValue:
			v.ListValue.Values = append(v.ListValue.Values, metadata.FromString(value))
		default:
			return metadataFields, fmt.Errorf(
				"unable to append %q field: field is not a list",
				a.Key)
		}
	} else {
		metadataFields.GetFields()[a.Key] = metadata.FromStringSlice([]string{value})
	}

	return metadataFields, nil
}

// ActionCopyToMetric specifies the options for a "copy-to-metric" action.
type ActionCopyToMetric struct {
	MetadataKeys   []string
	metadataKeyMap map[string]interface{}
}

// NewActionCopyToMetric returns a new "copy-to-metric" action.
// It also performs static validation of the options.
func NewActionCopyToMetric(options ActionOptions) (*ActionCopyToMetric, error) {
	var a ActionCopyToMetric
	if err := mapstructure.Decode(options, &a); err != nil {
		return nil, err
	}

	// Make a map from the list for constant-time lookups.
	metadataKeyMap := make(map[string]interface{})
	for _, k := range a.MetadataKeys {
		metadataKeyMap[k] = nil
	}

	a.metadataKeyMap = metadataKeyMap

	return &a, nil
}

// ApplyToTaggedMetric applies a "copy-to-metric" action to a tagged metric.
func (a *ActionCopyToMetric) ApplyToTaggedMetric(taggedMetric *data_receiver.TaggedMetric) *data_receiver.Metric {
	dimensions := make(map[string]string)
	metadataFields := metadata.FromStringMap(nil)

	// Sort tags into dimensions and metadata based on metadataKeys.
	for tagKey, tagValue := range taggedMetric.Tags {
		if _, ok := a.metadataKeyMap[tagKey]; ok {
			metadataFields.Fields[tagKey] = metadata.FromString(tagValue)
		} else {
			dimensions[tagKey] = tagValue
		}
	}

	return &data_receiver.Metric{
		Metric:         taggedMetric.Metric,
		Dimensions:     dimensions,
		MetadataFields: metadataFields,
		Timestamp:      taggedMetric.Timestamp,
		Value:          taggedMetric.Value,
	}
}

type dimensionsAndMetadataFields interface {
	GetDimensions() map[string]string
	GetMetadataFields() *_struct.Struct
}

func getTemplateContext(o dimensionsAndMetadataFields) map[string]interface{} {
	templateContext := make(map[string]interface{})

	sanitize := func(s string) string {
		// Mustache can't access context keys with dots in their name.
		return strings.ReplaceAll(s, ".", "_")
	}

	for k, v := range o.GetDimensions() {
		templateContext[sanitize(k)] = v
	}

	if o.GetMetadataFields() != nil {
		for k, v := range o.GetMetadataFields().GetFields() {
			templateContext[sanitize(k)] = unrollValue(v)
		}
	}
	return templateContext
}

func unrollValue(v *_struct.Value) interface{} {
	switch v.Kind.(type) {
	case *_struct.Value_StringValue:
		return v.GetStringValue()
	case *_struct.Value_NumberValue:
		return v.GetNumberValue()
	case *_struct.Value_BoolValue:
		return v.GetBoolValue()
	case *_struct.Value_ListValue:
		l := make([]interface{}, len(v.GetListValue().GetValues()))
		for i, value := range v.GetListValue().GetValues() {
			l[i] = unrollValue(value)
		}
		return l
	}

	return nil
}

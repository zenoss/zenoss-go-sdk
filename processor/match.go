package processor

import (
	_struct "google.golang.org/protobuf/types/known/structpb"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"
)

// MetricMatchConfig specifies metric matching criteria.
type MetricMatchConfig struct {
	Name           string            `yaml:"name"`
	Metric         string            `yaml:"metric"`
	DimensionKeys  []string          `yaml:"dimensionKeys"`
	Dimensions     map[string]string `yaml:"dimensions"`
	MetadataKeys   []string          `yaml:"metadataKeys"`
	MetadataFields map[string]string `yaml:"metadataFields"`
}

// GetName returns name for the metric match criteria.
func (m *MetricMatchConfig) GetName() string {
	if m.Name != "" {
		return m.Name
	}

	return "default"
}

// MatchesMetric returns true if metric matches the criteria.
func (m *MetricMatchConfig) MatchesMetric(metric *data_receiver.Metric) bool {
	return all(
		stringFieldMatches(m.Metric, metric.Metric),
		keysMatch(m.DimensionKeys, metric.Dimensions),
		fieldsFieldMatches(m.Dimensions, metric.Dimensions),
		metadataKeysMatch(m.MetadataKeys, metric.MetadataFields),
		metadataFieldsFieldMatches(m.MetadataFields, metric.MetadataFields))
}

// TaggedMetricMatchConfig specifies tagged metric matching criteria.
type TaggedMetricMatchConfig struct {
	Name    string            `yaml:"name"`
	Metric  string            `yaml:"metric"`
	TagKeys []string          `yaml:"tagKeys"`
	Tags    map[string]string `yaml:"tags"`
}

// GetName returns name for the tagged metric match criteria.
func (m *TaggedMetricMatchConfig) GetName() string {
	if m.Name != "" {
		return m.Name
	}

	return "default"
}

// MatchesTaggedMetric returns true if tagged metric matches the criteria.
func (m *TaggedMetricMatchConfig) MatchesTaggedMetric(taggedMetric *data_receiver.TaggedMetric) bool {
	return all(
		stringFieldMatches(m.Metric, taggedMetric.Metric),
		keysMatch(m.TagKeys, taggedMetric.Tags),
		fieldsFieldMatches(m.Tags, taggedMetric.Tags))
}

// ModelMatchConfig specifies model matching criteria.
type ModelMatchConfig struct {
	Name           string            `yaml:"name"`
	DimensionKeys  []string          `yaml:"dimensionKeys"`
	Dimensions     map[string]string `yaml:"dimensions"`
	MetadataKeys   []string          `yaml:"metadataKeys"`
	MetadataFields map[string]string `yaml:"metadataFields"`
}

// GetName returns name for the model match criteria.
func (m *ModelMatchConfig) GetName() string {
	if m.Name != "" {
		return m.Name
	}

	return "default"
}

// MatchesModel returns true if model matches the criteria.
func (m *ModelMatchConfig) MatchesModel(model *data_receiver.Model) bool {
	return all(
		keysMatch(m.DimensionKeys, model.Dimensions),
		fieldsFieldMatches(m.Dimensions, model.Dimensions),
		metadataKeysMatch(m.MetadataKeys, model.MetadataFields),
		metadataFieldsFieldMatches(m.MetadataFields, model.MetadataFields))
}

// all returns true if all args are true.
func all(args ...bool) bool {
	for _, arg := range args {
		if !arg {
			return false
		}
	}

	return true
}

// stringFieldMatches returns true if pattern matches value exactly.
func stringFieldMatches(pattern string, value string) bool {
	if pattern == "" {
		return true
	}

	return pattern == value
}

// keysMatch returns true if all keys are found in fields.
func keysMatch(keys []string, fields map[string]string) bool {
	if len(keys) == 0 {
		return true
	}

	if len(fields) == 0 {
		return false
	}

	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return false
		}
	}

	return true
}

// fieldsFieldMatches returns true if all fieldPatterns pairs exist in fields.
func fieldsFieldMatches(fieldPatterns map[string]string, fields map[string]string) bool {
	for patternKey, patternValue := range fieldPatterns {
		patternMatches := false
		for key, value := range fields {
			if patternKey == key && patternValue == value {
				patternMatches = true
				break
			}
		}
		if !patternMatches {
			return false
		}
	}

	return true
}

// metadataKeysMatch returns true if all metadataKeys are found in metadataFields.
func metadataKeysMatch(metadataKeys []string, metadataFields *_struct.Struct) bool {
	if len(metadataKeys) == 0 {
		return true
	}

	for _, metadataKey := range metadataKeys {
		if _, ok := metadataFields.GetFields()[metadataKey]; !ok {
			return false
		}
	}

	return true
}

// metadataFieldsFieldMatches returns true if all fieldPatterns pairs exist in metadataFields.
func metadataFieldsFieldMatches(fieldPatterns map[string]string, metadataFields *_struct.Struct) bool {
	if len(fieldPatterns) == 0 {
		return true
	}

	fields := make(map[string]string, len(metadataFields.GetFields()))
	for key, value := range metadataFields.GetFields() {
		fields[key] = value.GetStringValue()
	}

	return fieldsFieldMatches(fieldPatterns, fields)
}

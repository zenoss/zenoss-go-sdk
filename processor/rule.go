package processor

import (
	"context"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"
)

// MetricRuleConfig specifies a metric processing rule.
type MetricRuleConfig struct {
	Name        string               `yaml:"name"`
	Matches     []MetricMatchConfig  `yaml:"matches"`
	Actions     []MetricActionConfig `yaml:"actions"`
	MetricRules []MetricRuleConfig   `yaml:"metricRules"`
}

// GetName returns rule's name.
func (r *MetricRuleConfig) GetName() string {
	if r.Name == "" {
		return "default"
	}

	return r.Name
}

// MatchesMetric returns true if rule matches metric.
func (r *MetricRuleConfig) MatchesMetric(metric *data_receiver.Metric) bool {
	if r.Matches == nil {
		return true
	}

	for _, m := range r.Matches {
		if m.MatchesMetric(metric) {
			return true
		}
	}

	return false
}

// Apply will apply rule's actions (and sub-rules recursively) if it matches metric.
func (r *MetricRuleConfig) Apply(ctx context.Context, p *Processor, metric *data_receiver.Metric) (err error) {
	if r.MatchesMetric(metric) {
		for _, a := range r.Actions {
			if err = a.Apply(ctx, p, metric); err != nil {
				return
			}
		}

		for _, subrule := range r.MetricRules {
			if err = subrule.Apply(ctx, p, metric); err != nil {
				return
			}
		}
	}

	return
}

// TaggedMetricRuleConfig specifies a tagged metric processing rule.
type TaggedMetricRuleConfig struct {
	Name              string                     `yaml:"name"`
	Matches           []TaggedMetricMatchConfig  `yaml:"matches"`
	Actions           []TaggedMetricActionConfig `yaml:"actions"`
	TaggedMetricRules []TaggedMetricRuleConfig   `yaml:"taggedMetricRules"`
}

// GetName returns rule's name.
func (r *TaggedMetricRuleConfig) GetName() string {
	if r.Name == "" {
		return "default"
	}

	return r.Name
}

// MatchesTaggedMetric returns true if rule matches tagged metric.
func (r *TaggedMetricRuleConfig) MatchesTaggedMetric(taggedMetric *data_receiver.TaggedMetric) bool {
	if r.Matches == nil {
		return true
	}

	for _, m := range r.Matches {
		if m.MatchesTaggedMetric(taggedMetric) {
			return true
		}
	}

	return false
}

// Apply will apply rule's actions (and sub-rules recursively) if it matches tagged metric.
func (r *TaggedMetricRuleConfig) Apply(ctx context.Context, p *Processor, taggedMetric *data_receiver.TaggedMetric) (err error) {
	if r.MatchesTaggedMetric(taggedMetric) {
		for _, a := range r.Actions {
			if err = a.Apply(ctx, p, taggedMetric); err != nil {
				return
			}
		}

		for _, subrule := range r.TaggedMetricRules {
			if err = subrule.Apply(ctx, p, taggedMetric); err != nil {
				return
			}
		}
	}

	return
}

// ModelRuleConfig specifies a model processing rule.
type ModelRuleConfig struct {
	Name       string              `yaml:"name"`
	Matches    []ModelMatchConfig  `yaml:"matches"`
	Actions    []ModelActionConfig `yaml:"actions"`
	ModelRules []ModelRuleConfig   `yaml:"modelRules"`
}

// GetName returns rule's name.
func (r *ModelRuleConfig) GetName() string {
	if r.Name == "" {
		return "default"
	}

	return r.Name
}

// MatchesModel returns true if rule matches model.
func (r *ModelRuleConfig) MatchesModel(model *data_receiver.Model) bool {
	if r.Matches == nil {
		return true
	}

	for _, m := range r.Matches {
		if m.MatchesModel(model) {
			return true
		}
	}

	return false
}

// Apply will apply rule's actions (and sub-rules recursively) if it matches model.
func (r *ModelRuleConfig) Apply(ctx context.Context, p *Processor, model *data_receiver.Model) (err error) {
	if r.MatchesModel(model) {
		for _, a := range r.Actions {
			if err = a.Apply(ctx, p, model); err != nil {
				return
			}
		}

		for _, subrule := range r.ModelRules {
			if err = subrule.Apply(ctx, p, model); err != nil {
				return
			}
		}
	}

	return
}

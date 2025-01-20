// Package component provides exported structs that you should use during
// health monitoring initialization and collection.
package component

import (
	"slices"

	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

// New creates a new Component object
// If you set cType an empty string it will have "default" value assigned
// Empty targetID means the Component does not impact the health of any target Component
//
//revive:disable:argument-limit
func New(
	id, cType, targetID string, enableHeartbeat bool, metrics map[string]Aggregator, counterIDs, totalCounterIDs []string,
) (*Component, error) {
	set := make(map[string]struct{})
	for val := range metrics {
		set[val] = struct{}{}
	}
	for _, val := range counterIDs {
		set[val] = struct{}{}
	}
	for _, val := range totalCounterIDs {
		set[val] = struct{}{}
	}
	if len(set) < len(metrics)+len(counterIDs)+len(totalCounterIDs) {
		return nil, utils.ErrMeasureIDTaken
	}
	if len(set) > utils.ComponentMeasuresLimit {
		return nil, utils.ErrComponentMeasuresLimitExceeded
	}
	if cType == "" {
		cType = utils.DefaultComponentType
	}
	return &Component{
		ID:              id,
		Type:            cType,
		TargetID:        targetID,
		EnableHeartbeat: enableHeartbeat,
		Metrics:         metrics,
		CounterIDs:      counterIDs,
		TotalCounterIDs: totalCounterIDs,
	}, nil
}

//revive:enable:argument-limit

// Component keeps component data (such as component ID, metric IDs, etc.) and
// configuration (such as whether to enable heartbeat)
type Component struct {
	ID              string
	Metrics         map[string]Aggregator
	CounterIDs      []string
	TotalCounterIDs []string

	// Type is just a string that helps you to categorize your components
	// In future it will also help us to define priority of different type of components
	Type string
	// TargetID defines a higher level target component that is impacted by the health of the current component
	TargetID string

	// whether you want the heartbeat to affect your components health
	EnableHeartbeat bool
}

type Aggregator int

const (
	AggregatorMean Aggregator = iota
	AggregatorMin
	AggregatorMax
	AggregatorSum
	AggregatorCount

	DefaultAggregator = AggregatorMean
)

// IsMeasureIDUnique searches through all metric and counter IDs and
// returns false if such ID is already taken
func (c *Component) IsMeasureIDUnique(id string) bool {
	_, metricOk := c.Metrics[id]
	return !(metricOk ||
		slices.Contains(c.CounterIDs, id) ||
		slices.Contains(c.TotalCounterIDs, id))
}

func (c *Component) IsMeasuresLimitExceeded(newMeasures int) bool {
	return len(c.Metrics)+len(c.CounterIDs)+len(c.TotalCounterIDs)+
		newMeasures > utils.ComponentMeasuresLimit
}

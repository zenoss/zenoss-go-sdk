// Package component provides exported structs that you should use during
// health monitoring initialization and collection.
package component

import (
	"github.com/zenoss/zenoss-go-sdk/health/utils"
	sdk_utils "github.com/zenoss/zenoss-go-sdk/utils"
)

// New creates a new Component object
// If you set tType an empty string it will have "default" value assgined
//
//revive:disable:argument-limit
func New(
	id, tType string, enableHeartbeat bool, metricIDs, counterIDs, totalCounterIDs []string,
) (*Component, error) {
	set := make(map[string]struct{})
	for _, val := range metricIDs {
		set[val] = struct{}{}
	}
	for _, val := range counterIDs {
		set[val] = struct{}{}
	}
	for _, val := range totalCounterIDs {
		set[val] = struct{}{}
	}
	if len(set) < len(metricIDs)+len(counterIDs)+len(totalCounterIDs) {
		return nil, utils.ErrMeasureIDTaken
	}
	if tType == "" {
		tType = utils.DefaultComponentType
	}
	return &Component{
		ID:              id,
		Type:            tType,
		EnableHeartbeat: enableHeartbeat,
		MetricIDs:       metricIDs,
		CounterIDs:      counterIDs,
		TotalCounterIDs: totalCounterIDs,
	}, nil
}

//revive:enable:argument-limit

// Component keeps component data (such as component ID, metric IDs, etc.) and
// configuration (such as whether to enable heartbeat)
type Component struct {
	ID              string
	MetricIDs       []string
	CounterIDs      []string
	TotalCounterIDs []string

	// Type is just a string that helps you to categorize your components
	// In future it will also help us to define proirity of different type of components
	Type string

	// whether you want the heartbeat to affect your components health
	EnableHeartbeat bool
}

// IsMeasureIDUnique searches through all metric and counter IDs and
// returns false if such ID is already taken
func (t *Component) IsMeasureIDUnique(id string) bool {
	return !(sdk_utils.ListContainsString(t.MetricIDs, id) ||
		sdk_utils.ListContainsString(t.CounterIDs, id) ||
		sdk_utils.ListContainsString(t.TotalCounterIDs, id))
}

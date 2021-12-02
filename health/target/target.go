// Package target provides exported structs that you should use during
// health monitoring initialization and collection.
package target

import (
	"github.com/zenoss/zenoss-go-sdk/health/utils"
	sdk_utils "github.com/zenoss/zenoss-go-sdk/utils"
)

// New creates a new Target object
// If you set tType an empty string it will have "default" value assgined
func New(
	id, tType string, enableHeartbeat bool, metricIDs, counterIDs, totalCounterIDs []string,
) (*Target, error) {
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
		tType = utils.DefaultTargetType
	}
	return &Target{
		ID:              id,
		Type:            tType,
		EnableHeartbeat: enableHeartbeat,
		MetricIDs:       metricIDs,
		CounterIDs:      counterIDs,
		TotalCounterIDs: totalCounterIDs,
	}, nil
}

// Target keeps target data (such as target ID, metric IDs, etc.) and
// configuration (such as whether to enable heartbeat)
type Target struct {
	ID              string
	MetricIDs       []string
	CounterIDs      []string
	TotalCounterIDs []string

	// Type is just a string that helps you to categorize your targets
	// In future it will also help us to define proirity of different type of targets
	Type string

	// whether you want the heartbeat to affect your targets health
	EnableHeartbeat bool
}

// IsMeasureIDUnique searches through all metric and counter IDs and
// returns false if such ID is already taken
func (t *Target) IsMeasureIDUnique(ID string) bool {
	return !(sdk_utils.ListContainsString(t.MetricIDs, ID) ||
		sdk_utils.ListContainsString(t.CounterIDs, ID) ||
		sdk_utils.ListContainsString(t.TotalCounterIDs, ID))
}

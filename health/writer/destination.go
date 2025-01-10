package writer

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/zenoss/zenoss-go-sdk/health/component"
)

// Destination is a simple interface that is used by writer to output health data
type Destination interface {
	// Register takes a component object and do registration on destination side if needed
	Register(ctx context.Context, component *component.Component) error
	// Push takes a component health object and
	// does final calls to send data outside of health monitoring tool
	Push(ctx context.Context, health *component.Health) error
}

// NewLogDestination creates a new LogDestination instance
// Logger should be created earlier and passed as a parameter
func NewLogDestination(logger *zerolog.Logger) *LogDestination {
	return &LogDestination{log: logger}
}

// LogDestination outputs health data as a log message using configured logger
type LogDestination struct {
	log *zerolog.Logger
}

// Push takes health data and builds a nice log message with it on info level
func (l *LogDestination) Push(_ context.Context, health *component.Health) error {
	l.logComponentHealth(health)
	return nil
}

// Register takes component data and builds a nice log message with it on info level
func (l *LogDestination) Register(_ context.Context, component *component.Component) error {
	l.logComponentInfo(component)
	return nil
}

func (l *LogDestination) logComponentInfo(component *component.Component) {
	l.log.Info().Msgf("Got component update "+
		"ComponentID: %s, TargetID: %s, Heartbeat Enabled: %t, CounterIDs=%v, TotalCounterIDs=%v MetricIDs=%v",
		component.ID, component.TargetID, component.EnableHeartbeat,
		component.CounterIDs, component.TotalCounterIDs, component.MetricIDs,
	)
}

func (l *LogDestination) logComponentHealth(health *component.Health) {
	messageSums := make([]string, len(health.Messages))
	for i, message := range health.Messages {
		messageSums[i] = message.Summary
	}

	if health.Heartbeat.Enabled {
		l.log.Info().Msgf(
			"ComponentID: %s, TargetID: %s, Status=%v, Heartbeat=%t, Counters=%v, Metrics=%v, Messages=%v",
			health.ComponentID, health.TargetID, health.Status, health.Heartbeat.Beats, health.Counters,
			health.Metrics, messageSums,
		)
	} else {
		l.log.Info().Msgf(
			"ComponentID: %s, TargetID: %s, Status=%v, Counters=%v, Metrics=%v, Messages=%v",
			health.ComponentID, health.TargetID, health.Status, health.Counters, health.Metrics, messageSums,
		)
	}
}

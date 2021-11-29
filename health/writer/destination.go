package writer

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

// Destination is a simple interface that is used by writer to output health data
type Destination interface {
	// Register takes a target object and do registration on destination side if needed
	Register(ctx context.Context, target *target.Target) error
	// Push takes a target health object and
	// does final calls to send data outside of health monitoring tool
	Push(ctx context.Context, health *target.Health) error
}

// NewLogDestination creates a new LogDestination instance
// Logger should be created earlier and passed as a parameter
func NewLogDestination(logger *zerolog.Logger) Destination {
	return &LogDestination{log: logger}
}

// LogDestination outputs health data as a log message using configured logger
type LogDestination struct {
	log *zerolog.Logger
}

// Push takes health data and builds a nice log message with it on info level
func (l *LogDestination) Push(ctx context.Context, health *target.Health) error {
	l.logTargetHealth(health)
	return nil
}

func (l *LogDestination) Register(ctx context.Context, target *target.Target) error {
	l.logTargetInfo(target)
	return nil
}

func (l *LogDestination) logTargetInfo(target *target.Target) {
	l.log.Info().Msgf("Got target update "+
		"TargetID: %s, Heartbeat Enabled: %t, CounterIDs=%v, TotalCounterIDs=%v MetricIDs=%v",
		target.ID, target.EnableHeartbeat,
		target.CounterIDs, target.TotalCounterIDs, target.MetricIDs,
	)
}

func (l *LogDestination) logTargetHealth(health *target.Health) {
	messageSums := make([]string, len(health.Messages))
	for i, message := range health.Messages {
		messageSums[i] = message.Summary
	}

	if health.Heartbeat.Enabled {
		l.log.Info().Msgf(
			"TargetID: %s, Status=%v, Heartbeat=%t, Counters=%v, Metrics=%v, Messages=%v",
			health.TargetID, health.Status, health.Heartbeat.Beats, health.Counters,
			health.Metrics, messageSums,
		)
	} else {
		l.log.Info().Msgf(
			"TargetID: %s, Status=%v, Counters=%v, Metrics=%v, Messages=%v",
			health.TargetID, health.Status, health.Counters, health.Metrics, messageSums,
		)
	}
}

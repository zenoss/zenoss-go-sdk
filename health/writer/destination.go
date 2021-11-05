package writer

import (
	"github.com/rs/zerolog"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

// Destination is a simple interface that is used by writer to output health data
type Destination interface {
	// Push takes a target health object and
	// does final calls to send data outside of health monitoring tool
	Push(health *target.Health) error
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
func (l *LogDestination) Push(health *target.Health) error {
	l.logTargetHealth(health)
	return nil
}

func (l *LogDestination) logTargetHealth(health *target.Health) {
	messageSums := make([]string, len(health.Messages))
	for i, message := range health.Messages {
		messageSums[i] = message.Summary
	}

	if health.Heartbeat.Enabled {
		l.log.Info().Msgf(
			"TargetID: %s, Healthy=%t, Heartbeat=%t, Counters=%v, Metrics=%v, Messages=%v",
			health.ID, health.Healthy, health.Heartbeat.Beats, health.Counters,
			health.Metrics, messageSums,
		)
	} else {
		l.log.Info().Msgf(
			"TargetID: %s, Healthy=%t, Counters=%v, Metrics=%v, Messages=%v",
			health.ID, health.Healthy, health.Counters, health.Metrics, messageSums,
		)
	}
}

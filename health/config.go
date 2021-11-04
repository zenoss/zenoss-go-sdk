package health

import (
	"time"
)

// Config is a struct that we pass to health manager. It defines some basic configuration
type Config struct {
	// CollectionCycle defines its duration ie how often health manager will calculate raw metrics
	// and send them to a health writer.
	CollectionCycle time.Duration

	// RegistrationOnCollect shows if we want to allow to collect data for not registered targets
	// Targets will be created automatically. Not recommended to use
	// Note: all counters will be added as default counters
	RegistrationOnCollect bool

	// LogLevel is applied for default health logger
	// Possible values: trace, debug, info, warn, error, fatal, panic
	LogLevel string
}

// NewConfig is a constructor for health.Config. It also defines some default values
func NewConfig() *Config {
	return &Config{CollectionCycle: 30 * time.Second}
}

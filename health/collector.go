package health

import (
	"time"

	"github.com/zenoss/zenoss-go-sdk/health/errors"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

var (
	collector *healthCollector
)

// Collector provides different methods to send health information per target
type Collector interface {
	// HeartBeat should be run in goroutine. It will send heartbeat data once per collection cycle
	HeartBeat(targetID string) error
	// AddToCounter updates counter with provided value (can be negative).
	// Used for both TotalCounters and Counters
	AddToCounter(targetID, counterID string, value int32) error
	// AddMetricValue stores one more metric for id
	// In the end of the cycle the manager will count an average value
	AddMetricValue(targetID, metricID string, value float64) error
	// HealthMessage sends a message for target. More info in target.Message struct
	HealthMessage(targetID string, msg *target.Message) error
	// ChangeHealth marks your target as healthy (true) or unhealthy (false). Target health will not
	// be restored by itself, you need to call this method if you want to restore healthy status
	ChangeHealth(targetID string, healthStatus target.HealthStatus) error
}

func InitCollector(cycleDuration time.Duration, metricsIn chan<- *targetMeasurement) {
	collector = &healthCollector{
		cycleDuration: cycleDuration,
		metricsIn:     metricsIn,
		isRunning:     true,
	}
}

func ShutDownCollector() {
	collector.isRunning = false
	close(collector.metricsIn)
}

// GetCollector returns a health collector instance if it was already initialized by manager
func GetCollector() (Collector, error) {
	if collector == nil {
		return nil, errors.ErrDeadCollector
	}
	return collector, nil
}

type healthCollector struct {
	cycleDuration time.Duration
	metricsIn     chan<- *targetMeasurement
	isRunning     bool
}

func (hc *healthCollector) HeartBeat(targetID string) error {
	ticker := time.NewTicker(hc.cycleDuration)
	for {
		select {
		case <-ticker.C:
			if !hc.isRunning {
				return errors.ErrDeadCollector
			}
			measure := &targetMeasurement{
				targetID:    targetID,
				measureType: heartbeat,
			}
			hc.metricsIn <- measure
		}
	}
}

func (hc *healthCollector) AddToCounter(targetID, counterID string, value int32) error {
	if !hc.isRunning {
		return errors.ErrDeadCollector
	}
	measure := &targetMeasurement{
		targetID:      targetID,
		measureType:   counterChange,
		measureID:     counterID,
		counterChange: value,
	}
	hc.metricsIn <- measure
	return nil
}

func (hc *healthCollector) AddMetricValue(targetID, metricID string, value float64) error {
	if !hc.isRunning {
		return errors.ErrDeadCollector
	}
	measure := &targetMeasurement{
		targetID:    targetID,
		measureType: metric,
		measureID:   metricID,
		metricValue: value,
	}
	hc.metricsIn <- measure
	return nil
}

func (hc *healthCollector) HealthMessage(targetID string, msg *target.Message) error {
	if !hc.isRunning {
		return errors.ErrDeadCollector
	}
	measure := &targetMeasurement{
		targetID:    targetID,
		measureType: message,
		message:     msg,
	}
	hc.metricsIn <- measure
	return nil
}

func (hc *healthCollector) ChangeHealth(targetID string, status target.HealthStatus) error {
	if !hc.isRunning {
		return errors.ErrDeadCollector
	}
	measure := &targetMeasurement{
		targetID:     targetID,
		measureType:  healthStatus,
		healthStatus: status,
	}
	hc.metricsIn <- measure
	return nil
}

package health

import (
	"context"
	"time"

	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

var (
	collector *healthCollector
)

// Collector provides different methods to send health information per target
type Collector interface {
	// HeartBeat runs a new goroutine that sends heartbeat data once per collection cycle.
	// It returns the cancel that will stop the heartbeat goroutine.
	HeartBeat(targetID string) (context.CancelFunc, error)
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

// NewCollector creates a new healthCollector instance.
func NewCollector(cycleDuration time.Duration, metricsIn chan<- *TargetMeasurement) Collector {
	return &healthCollector{
		cycleDuration: cycleDuration,
		metricsIn:     metricsIn,
		isRunning:     true,
	}
}

// SetCollectorSingleton saves your collector instance as a var here.
// You can get it by GetCollectorSingleton whether you need it.
func SetCollectorSingleton(c Collector) {
	collector = c.(*healthCollector)
}

// StopCollectorSingleton turns off collector and closes input channel
func StopCollectorSingleton() {
	if collector != nil {
		collector.isRunning = false
		close(collector.metricsIn)
	}
}

// GetCollectorSingleton returns a health collector instance if it was already initialized
func GetCollectorSingleton() (Collector, error) {
	if collector == nil {
		return nil, utils.ErrDeadCollector
	}
	return collector, nil
}

// ResetCollectorSingleton sets collector var to nil
func ResetCollectorSingleton() {
	collector = nil
}

type healthCollector struct {
	cycleDuration time.Duration
	metricsIn     chan<- *TargetMeasurement
	isRunning     bool
}

func (hc *healthCollector) HeartBeat(targetID string) (context.CancelFunc, error) {
	if !hc.isRunning {
		return nil, utils.ErrDeadCollector
	}
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ticker := time.NewTicker(hc.cycleDuration)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !hc.isRunning {
					return
				}
				measure := &TargetMeasurement{
					TargetID:    targetID,
					MeasureType: Heartbeat,
				}
				hc.metricsIn <- measure
			}
		}
	}()

	return cancel, nil
}

func (hc *healthCollector) AddToCounter(targetID, counterID string, value int32) error {
	if !hc.isRunning {
		return utils.ErrDeadCollector
	}
	measure := &TargetMeasurement{
		TargetID:      targetID,
		MeasureType:   CounterChange,
		MeasureID:     counterID,
		CounterChange: value,
	}
	hc.metricsIn <- measure
	return nil
}

func (hc *healthCollector) AddMetricValue(targetID, metricID string, value float64) error {
	if !hc.isRunning {
		return utils.ErrDeadCollector
	}
	measure := &TargetMeasurement{
		TargetID:    targetID,
		MeasureType: Metric,
		MeasureID:   metricID,
		MetricValue: value,
	}
	hc.metricsIn <- measure
	return nil
}

func (hc *healthCollector) HealthMessage(targetID string, msg *target.Message) error {
	if !hc.isRunning {
		return utils.ErrDeadCollector
	}
	measure := &TargetMeasurement{
		TargetID:    targetID,
		MeasureType: Message,
		Message:     msg,
	}
	hc.metricsIn <- measure
	return nil
}

func (hc *healthCollector) ChangeHealth(targetID string, status target.HealthStatus) error {
	if !hc.isRunning {
		return utils.ErrDeadCollector
	}
	measure := &TargetMeasurement{
		TargetID:     targetID,
		MeasureType:  HealthStatus,
		HealthStatus: status,
	}
	hc.metricsIn <- measure
	return nil
}

// MeasureType lists possible measurements types that collector can work with
type MeasureType int

// list of measure types
const (
	_ MeasureType = iota
	HealthStatus
	Heartbeat
	CounterChange
	Metric
	Message
)

// TargetMeasurement is used by collector to send data.
// You should define a measureType so manager will know what field it should looking for
// Can be splitted into different structs and wrapped with the one interface in case
// if we will have too many different measure types (should save us some ram)
type TargetMeasurement struct {
	TargetID    string
	MeasureType MeasureType

	MeasureID string // used for both metrics and counters

	// actual measurement fields
	HealthStatus  target.HealthStatus
	Message       *target.Message
	CounterChange int32
	MetricValue   float64
}

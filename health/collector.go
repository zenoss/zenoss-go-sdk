package health

import (
	"context"
	"fmt"
	"sync"
	"time"

	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

var (
	collector     *healthCollector
	collectorLock sync.Mutex
)

// Collector provides different methods to send health information per target
type Collector interface {
	// HeartBeat runs a new goroutine that sends heartbeat data once per collection cycle.
	// It returns the cancel that will stop the heartbeat goroutine.
	HeartBeat(targetID string) (context.CancelFunc, error)
	// UpdateCycleDuration updates heartbeat cycle duration with provided value for all active targets
	UpdateCycleDuration(d time.Duration) error
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
		done:          make(chan struct{}),
	}
}

// SetCollectorSingleton saves your collector instance as a var here.
// You can get it by GetCollectorSingleton whether you need it.
func SetCollectorSingleton(c Collector) {
	collectorLock.Lock()
	defer collectorLock.Unlock()
	collector = c.(*healthCollector)
}

// StopCollectorSingleton turns off collector and closes input channel
func StopCollectorSingleton() {
	collectorLock.Lock()
	defer collectorLock.Unlock()
	if collector != nil {
		collector.doneOnce.Do(func() {
			close(collector.done)
		})
	}
}

// GetCollectorSingleton returns a health collector instance if it was already initialized
func GetCollectorSingleton() (Collector, error) {
	collectorLock.Lock()
	defer collectorLock.Unlock()
	if collector == nil {
		return nil, utils.ErrDeadCollector
	}
	return collector, nil
}

// ResetCollectorSingleton sets collector var to nil
func ResetCollectorSingleton() {
	collectorLock.Lock()
	defer collectorLock.Unlock()
	collector = nil
}

type healthCollector struct {
	cycleDuration time.Duration
	heartbeats    sync.Map
	mu            sync.RWMutex
	metricsIn     chan<- *TargetMeasurement
	done          chan struct{}
	doneOnce      sync.Once
}

type heartbeatTracker struct {
	cancel context.CancelFunc
	ticker *time.Ticker
}

func (hc *healthCollector) HeartBeat(targetID string) (context.CancelFunc, error) {
	select {
	case <-hc.done:
		return nil, utils.ErrDeadCollector
	default:
	}

	if hb, exists := hc.heartbeats.Load(targetID); exists {
		heartbeat := hb.(*heartbeatTracker)
		heartbeat.ticker.Stop()
		heartbeat.cancel()
		hc.heartbeats.Delete(targetID)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		hc.mu.RLock()
		ticker := time.NewTicker(hc.cycleDuration)
		hc.mu.RUnlock()
		hc.heartbeats.Store(targetID, &heartbeatTracker{
			cancel: cancel,
			ticker: ticker,
		})
		defer func() {
			ticker.Stop()
			hc.heartbeats.Delete(targetID)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-hc.done:
				return
			case <-ticker.C:
				measure := &TargetMeasurement{
					TargetID:    targetID,
					MeasureType: Heartbeat,
				}

				select {
				case hc.metricsIn <- measure:
				case <-hc.done:
					return
				}
			}
		}
	}()

	return cancel, nil
}

func (hc *healthCollector) UpdateCycleDuration(newDuration time.Duration) error {
	if newDuration <= 0 {
		return fmt.Errorf("cycle duration must be positive")
	}

	hc.mu.Lock()
	hc.cycleDuration = newDuration
	hc.mu.Unlock()

	targets := 0
	hc.heartbeats.Range(func(_, hb any) bool {
		heartbeat := hb.(*heartbeatTracker)
		heartbeat.ticker.Reset(newDuration)
		targets++
		return true
	})
	if targets > 0 {
		logging.GetLogger().Info().Msgf("Updated heartbeat interval to %v for %d targets",
			newDuration, targets,
		)
	}
	return nil
}

func (hc *healthCollector) AddToCounter(targetID, counterID string, value int32) error {
	select {
	case hc.metricsIn <- &TargetMeasurement{
		TargetID:      targetID,
		MeasureType:   CounterChange,
		MeasureID:     counterID,
		CounterChange: value,
	}:
		return nil
	case <-hc.done:
		return utils.ErrDeadCollector
	}
}

func (hc *healthCollector) AddMetricValue(targetID, metricID string, value float64) error {
	select {
	case hc.metricsIn <- &TargetMeasurement{
		TargetID:    targetID,
		MeasureType: Metric,
		MeasureID:   metricID,
		MetricValue: value,
	}:
		return nil
	case <-hc.done:
		return utils.ErrDeadCollector
	}
}

func (hc *healthCollector) HealthMessage(targetID string, msg *target.Message) error {
	select {
	case hc.metricsIn <- &TargetMeasurement{
		TargetID:    targetID,
		MeasureType: Message,
		Message:     msg,
	}:
		return nil
	case <-hc.done:
		return utils.ErrDeadCollector
	}
}

func (hc *healthCollector) ChangeHealth(targetID string, status target.HealthStatus) error {
	select {
	case hc.metricsIn <- &TargetMeasurement{
		TargetID:     targetID,
		MeasureType:  HealthStatus,
		HealthStatus: status,
	}:
		return nil
	case <-hc.done:
		return utils.ErrDeadCollector
	}
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

package health

import (
	"sync"

	t "github.com/zenoss/zenoss-go-sdk/health/target"
)

func newRawHealth(target *t.Target, metricIDs []string) *rawHealth {
	tHealth := &rawHealth{
		target:        target,
		status:        t.Healthy,
		heartBeat:     false,
		counters:      make(map[string]int32),
		totalCounters: make(map[string]int32),
		rawMetrics:    make(map[string][]float64),
		messages:      make([]*t.Message, 0),
	}
	for _, metricID := range metricIDs {
		tHealth.rawMetrics[metricID] = make([]float64, 0)
	}
	return tHealth
}

// rawHealth is a struct that keeps target information together with raw target health
type rawHealth struct {
	target        *t.Target
	status        t.HealthStatus
	heartBeat     bool
	counters      map[string]int32
	totalCounters map[string]int32
	rawMetrics    map[string][]float64
	messages      []*t.Message
}

// healthRegistry keeps raw health data and provides an interface to get and update it
// It is a caller responsibility to lock and unlock registry before usage
type healthRegistry interface {
	lock()
	unlock()
	getRawHealthForTarget(targetID string) (*rawHealth, bool)
	setRawHealthForTarget(rawTargetHealth *rawHealth)
	getRawHealthMap() map[string]*rawHealth
}

func newHealthRegistry() healthRegistry {
	return &memRegistry{
		mutex:     &sync.Mutex{},
		rawHealth: make(map[string]*rawHealth),
	}
}

// memRegistry is a healthRegistry implementation that keeps all raw health in RAM
type memRegistry struct {
	mutex     *sync.Mutex
	rawHealth map[string]*rawHealth
}

func (reg *memRegistry) lock() {
	reg.mutex.Lock()
}

func (reg *memRegistry) unlock() {
	reg.mutex.Unlock()
}

func (reg *memRegistry) getRawHealthForTarget(targetID string) (*rawHealth, bool) {
	health, ok := reg.rawHealth[targetID]
	return health, ok
}

func (reg *memRegistry) setRawHealthForTarget(rawTargetHealth *rawHealth) {
	reg.rawHealth[rawTargetHealth.target.ID] = rawTargetHealth
}

func (reg *memRegistry) getRawHealthMap() map[string]*rawHealth {
	return reg.rawHealth
}

type measureType int

const (
	_ measureType = iota
	healthStatus
	heartbeat
	counterChange
	metric
	message
)

// targetMeasurement is used by collector to send data.
// You should define a measureType so manager will know what field it should looking for
// Can be splitted into different structs and wrapped with the one interface in case
// if we will have to many different measure types (should save us some ram)
type targetMeasurement struct {
	targetID    string
	measureType measureType

	// actual measurement fields
	healthStatus  t.HealthStatus
	message       *t.Message
	counterChange int32
	metricValue   float64
	measureID     string // used for both metrics and counters
}

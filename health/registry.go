package health

import (
	"sync"

	"github.com/zenoss/zenoss-go-sdk/health/target"
)

func newRawHealth(t *target.Target) *rawHealth {
	tHealth := &rawHealth{
		target:        t,
		status:        target.Healthy,
		heartBeat:     false,
		counters:      make(map[string]int32),
		totalCounters: make(map[string]int32),
		rawMetrics:    make(map[string][]float64),
		messages:      make([]*target.Message, 0),
	}
	for _, metricID := range t.MetricIDs {
		tHealth.rawMetrics[metricID] = make([]float64, 0)
	}
	return tHealth
}

// rawHealth is a struct that keeps target information together with raw target health
type rawHealth struct {
	target        *target.Target
	status        target.HealthStatus
	heartBeat     bool
	counters      map[string]int32
	totalCounters map[string]int32
	rawMetrics    map[string][]float64
	messages      []*target.Message
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

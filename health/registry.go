package health

import (
	"sync"

	"github.com/zenoss/zenoss-go-sdk/health/component"
)

func newRawHealth(t *component.Component) *rawHealth {
	tHealth := &rawHealth{
		component:     t,
		status:        component.Healthy,
		heartBeat:     false,
		counters:      make(map[string]int32),
		totalCounters: make(map[string]int32),
		rawMetrics:    make(map[string][]float64),
		messages:      make([]*component.Message, 0),
	}
	for _, metricID := range t.MetricIDs {
		tHealth.rawMetrics[metricID] = make([]float64, 0)
	}
	return tHealth
}

// rawHealth is a struct that keeps component information together with raw component health
type rawHealth struct {
	component     *component.Component
	status        component.HealthStatus
	heartBeat     bool
	counters      map[string]int32
	totalCounters map[string]int32
	rawMetrics    map[string][]float64
	messages      []*component.Message
}

// healthRegistry keeps raw health data and provides an interface to get and update it
// It is a caller responsibility to lock and unlock registry before usage
type healthRegistry interface {
	lock()
	unlock()
	getRawHealthForComponent(componentID string) (*rawHealth, bool)
	setRawHealthForComponent(rawComponentHealth *rawHealth)
	getRawHealthMap() map[string]*rawHealth
	checkTarget(componentID string) (registered bool, hbEnabled bool)
	setTarget(componentID string, hbEnabled bool)
	getTargetsMap() map[string]bool
}

func newHealthRegistry() healthRegistry {
	return &memRegistry{
		mutex:            &sync.Mutex{},
		rawHealth:        make(map[string]*rawHealth),
		targetComponents: make(map[string]bool),
	}
}

// memRegistry is a healthRegistry implementation that keeps all raw health in RAM
type memRegistry struct {
	mutex            *sync.Mutex
	rawHealth        map[string]*rawHealth
	targetComponents map[string]bool
}

func (reg *memRegistry) lock() {
	reg.mutex.Lock()
}

func (reg *memRegistry) unlock() {
	reg.mutex.Unlock()
}

func (reg *memRegistry) getRawHealthForComponent(componentID string) (*rawHealth, bool) {
	health, ok := reg.rawHealth[componentID]
	return health, ok
}

func (reg *memRegistry) setRawHealthForComponent(rawComponentHealth *rawHealth) {
	reg.rawHealth[rawComponentHealth.component.ID] = rawComponentHealth
}

func (reg *memRegistry) getRawHealthMap() map[string]*rawHealth {
	return reg.rawHealth
}

func (reg *memRegistry) checkTarget(componentID string) (registered bool, hbEnabled bool) {
	hbEnabled, registered = reg.targetComponents[componentID]
	return
}

func (reg *memRegistry) setTarget(componentID string, hbEnabled bool) {
	reg.targetComponents[componentID] = hbEnabled
}

func (reg *memRegistry) getTargetsMap() map[string]bool {
	return reg.targetComponents
}

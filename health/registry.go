package health

import (
	"sync"

	"github.com/zenoss/zenoss-go-sdk/health/component"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
)

func newRawHealth(c *component.Component) *rawHealth {
	cHealth := &rawHealth{
		component:     c,
		status:        component.Healthy,
		heartBeat:     false,
		counters:      make(map[string]int32),
		totalCounters: make(map[string]int32),
		metrics:       make(map[string]*metric),
		messages:      make([]*component.Message, 0),
	}
	for metricID, ag := range c.Metrics {
		cHealth.metrics[metricID] = &metric{
			values:     make([]float64, 0),
			aggregator: ag,
		}
	}
	return cHealth
}

// rawHealth is a struct that keeps component information together with raw component health
type (
	rawHealth struct {
		component     *component.Component
		status        component.HealthStatus
		heartBeat     bool
		counters      map[string]int32
		totalCounters map[string]int32
		metrics       map[string]*metric
		messages      []*component.Message
	}

	metric struct {
		values     []float64
		aggregator component.Aggregator
	}
)

// healthRegistry keeps raw health data and provides an interface to get and update it
// It is a caller responsibility to lock and unlock registry before usage
type healthRegistry interface {
	lock()
	unlock()
	getRawHealthForComponent(componentID string) (*rawHealth, bool)
	setRawHealthForComponent(rawComponentHealth *rawHealth)
	removeRawHealthForComponent(componentID string)
	getRawHealthMap() map[string]*rawHealth

	checkTarget(componentID string) (registered bool, hbEnabled bool)
	setTarget(componentID string, hbEnabled bool)
	removeTarget(componentID string)
	getTargetsMap() map[string]bool

	getPriorityForType(componentType string) component.Priority
	setPriorityForType(componentType string, priority component.Priority)
}

func newHealthRegistry() healthRegistry {
	return &memRegistry{
		mutex:            &sync.Mutex{},
		rawHealth:        make(map[string]*rawHealth),
		targetComponents: make(map[string]bool),
		typePriorities:   make(map[string]component.Priority),
	}
}

// memRegistry is a healthRegistry implementation that keeps all raw health in RAM
type memRegistry struct {
	mutex            *sync.Mutex
	rawHealth        map[string]*rawHealth
	targetComponents map[string]bool
	typePriorities   map[string]component.Priority
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

func (reg *memRegistry) removeRawHealthForComponent(componentID string) {
	delete(reg.rawHealth, componentID)
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

func (reg *memRegistry) removeTarget(componentID string) {
	delete(reg.targetComponents, componentID)
}

func (reg *memRegistry) getTargetsMap() map[string]bool {
	return reg.targetComponents
}

func (reg *memRegistry) getPriorityForType(componentType string) component.Priority {
	if componentType == "" {
		componentType = utils.DefaultComponentType
	}
	if priority, ok := reg.typePriorities[componentType]; ok {
		return priority
	}
	return component.DefaultPriority
}

func (reg *memRegistry) setPriorityForType(componentType string, priority component.Priority) {
	if componentType == "" {
		componentType = utils.DefaultComponentType
	}
	reg.typePriorities[componentType] = priority
}

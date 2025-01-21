/*
Package health implements a simple tool for health data + metrics collection.

We defines three main parts of the health monitorin tool: manager, collector and writer.
- Writer description is avilable in its own package.
- Collector is stored in collector.go file. It provides a simple interface that allows you to collect
different type of health data per component. All methods should have componentID as a parameter. Collector
will automatically send data to health manager.
- Manger is a heart of the health collection tool. It initialize all comunication channels and has
a control over collector and writer. Manager keeps all data collected by collector, calculates every
component health once per cycle and sends calculated data to writer.
*/
package health

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/exp/maps"

	"github.com/zenoss/zenoss-go-sdk/health/component"
	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/utils"
	w "github.com/zenoss/zenoss-go-sdk/health/writer"
	sdk_utils "github.com/zenoss/zenoss-go-sdk/utils"
)

// FrameworkStop synchronously terminates all framework subprocesses
type FrameworkStop func()

// FrameworkStart initializes dependencies and starts health monitoring
// If you call it in goroutine, you need to wait a little bit for collector to initialize
func FrameworkStart(ctx context.Context, cfg *Config, m Manager, writer w.HealthWriter) FrameworkStop {
	if cfg.LogLevel != "" {
		logging.SetLogLevel(cfg.LogLevel)
	}

	measurementsCh := make(chan *ComponentMeasurement)
	healthCh := make(chan *component.Health)
	componentCh := make(chan *component.Component)

	c := NewCollector(cfg.CollectionCycle, measurementsCh)
	SetCollectorSingleton(c)

	ctx, cancel := context.WithCancel(ctx)
	var doneWg sync.WaitGroup

	doneWg.Add(1)
	go func() {
		defer doneWg.Done()
		defer cancel()
		<-ctx.Done()

		StopCollectorSingleton()
	}()

	doneWg.Add(1)
	go func() {
		defer doneWg.Done()
		defer cancel()

		m.Start(ctx, measurementsCh, healthCh, componentCh)
		<-m.Done()
	}()

	doneWg.Add(1)
	go func() {
		defer doneWg.Done()
		defer cancel()

		writer.Start(ctx, healthCh, componentCh)
	}()

	return func() {
		cancel()

		StopCollectorSingleton()
		m.Shutdown()
		writer.Shutdown()
		doneWg.Wait()
	}
}

// Manager keeps all information about components and provides a functionality
// to initialize collector, writer and communication channels
type Manager interface {
	// Start method should initialize collector with it's configuration and
	// define a method to send data to a writer. Remember that healthIn and componentIn channels
	// require readers and measureOut requires a writer.
	// Make sure to close measureOut before manager shutdown to not lose any data.
	Start(
		ctx context.Context, measureOut <-chan *ComponentMeasurement,
		healthIn chan<- *component.Health, componentIn chan<- *component.Component,
	)
	// UpdateConfig applies the new configuration for manager and collector
	UpdateConfig(config *Config) error
	// Shutdown method closes manager's channels and terminates goroutines
	Shutdown()
	// IsStarted return the status of the manager
	IsStarted() bool
	// AddComponents provides a simple interface to register monitored components
	AddComponents(components []*component.Component) error
	// DeleteComponents removes components with the IDs provided from monitoring
	DeleteComponents(componentIDs []string)
	// SetPriority defines the priority of the impact of components of a given type
	SetPriority(componentType string, priority component.Priority)

	Done() <-chan struct{}
}

// NewManager creates a new Manager instance with internal component registry.
// You should init configuration and writer before and pass it here
func NewManager(_ context.Context, config *Config) Manager {
	healthReg := newHealthRegistry()
	return &healthManager{
		registry: healthReg,
		config:   config,
		wg:       &sync.WaitGroup{},
		stopWait: make(chan struct{}),
	}
}

// healthManager implements Manager interface
type healthManager struct {
	registry    healthRegistry
	config      *Config
	configIn    chan<- *Config
	healthIn    chan<- *component.Health
	componentIn chan<- *component.Component

	// channel that block shutdown function from return until manager is stopped
	stopWait chan struct{}
	// channel that used by shutdown call to stop the manager
	stopSig chan struct{}
	// used to wait for manager processes to stop so we can mark started as false in a right time
	wg *sync.WaitGroup

	mu       sync.Mutex
	configMu sync.RWMutex
	started  atomic.Bool
}

func (hm *healthManager) Start(
	ctx context.Context, measureOut <-chan *ComponentMeasurement,
	healthIn chan<- *component.Health, componentIn chan<- *component.Component,
) {
	configCh := make(chan *Config)

	hm.mu.Lock()
	hm.configIn = configCh
	hm.componentIn = componentIn
	hm.healthIn = healthIn
	hm.stopSig = make(chan struct{})
	hm.mu.Unlock()

	hm.wg.Add(1)
	go func() {
		defer hm.wg.Done()
		hm.listenMeasurements(ctx, measureOut)
	}()

	hm.wg.Add(1)
	go func() {
		defer hm.wg.Done()
		hm.healthForwarder(ctx, configCh, healthIn)
	}()

	hm.started.Store(true)

	go func() {
		hm.wg.Wait()
		hm.started.Store(false)
		close(hm.stopWait)
	}()

	hm.sendComponentsInfo() // if some components were added before start we need to register them
}

func (hm *healthManager) UpdateConfig(newConfig *Config) error {
	if newConfig == nil {
		return fmt.Errorf("config should not be nil")
	}
	if newConfig.CollectionCycle <= 0 {
		return fmt.Errorf("collection cycle must be positive")
	}
	logging.SetLogLevel(newConfig.LogLevel)

	var cycleDurUpdated bool
	hm.configMu.Lock()
	cycleDurUpdated = hm.config.CollectionCycle != newConfig.CollectionCycle
	hm.config = newConfig
	hm.configMu.Unlock()

	if cycleDurUpdated {
		coll, err := GetCollectorSingleton()
		if err != nil {
			return err
		}
		err = coll.UpdateCycleDuration(newConfig.CollectionCycle)
		if err != nil {
			return err
		}
		go func() {
			hm.configIn <- newConfig
		}()
	}
	return nil
}

func (hm *healthManager) collectionCycle() time.Duration {
	hm.configMu.RLock()
	defer hm.configMu.RUnlock()
	return hm.config.CollectionCycle
}

func (hm *healthManager) registrationOnCollect() bool {
	hm.configMu.RLock()
	defer hm.configMu.RUnlock()
	return hm.config.RegistrationOnCollect
}

func (hm *healthManager) targetHealthFn() func(map[component.HealthStatus]map[component.Priority]int) component.HealthStatus {
	hm.configMu.RLock()
	defer hm.configMu.RUnlock()
	if hm.config.TargetHealthFn == nil {
		return defaultTargetHealthFn
	}
	return hm.config.TargetHealthFn
}

func (hm *healthManager) Shutdown() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	close(hm.stopSig)
	<-hm.stopWait
	close(hm.componentIn)
	close(hm.configIn)
}

func (hm *healthManager) Done() <-chan struct{} {
	return hm.stopWait
}

func (hm *healthManager) IsStarted() bool {
	return hm.started.Load()
}

func (hm *healthManager) AddComponents(components []*component.Component) error {
	hm.registry.lock()
	defer hm.registry.unlock()

	uniqueComponents := make(map[string]struct{}, len(components))
	for _, c := range components {
		uniqueComponents[c.ID] = struct{}{}
		if c.TargetID != "" {
			uniqueComponents[c.TargetID] = struct{}{}
		}
	}
	if len(uniqueComponents)+len(hm.registry.getRawHealthMap()) > utils.ComponentsLimit {
		return utils.ErrComponentsLimitExceeded
	}

	for _, newComponent := range components {
		hm.registry.setRawHealthForComponent(newRawHealth(newComponent))
		if hm.IsStarted() {
			hm.componentIn <- newComponent
		}
		hm.updateTargetsRegistry(newComponent)
	}

	for targetID, hbEnabled := range hm.registry.getTargetsMap() {
		// raw health must be set for target component if it has not been added explicitly
		if _, ok := hm.registry.getRawHealthForComponent(targetID); !ok {
			targetComponent := &component.Component{
				ID:              targetID,
				Type:            utils.DefaultHealthTarget,
				EnableHeartbeat: hbEnabled,
			}
			hm.registry.setRawHealthForComponent(newRawHealth(targetComponent))
			if hm.IsStarted() {
				hm.componentIn <- targetComponent
			}
		}
	}

	return nil
}

func (hm *healthManager) DeleteComponents(componentIDs []string) {
	hm.registry.lock()
	defer hm.registry.unlock()

	targetsToRemove := map[string]struct{}{}
	for _, cID := range componentIDs {
		rHealth, ok := hm.registry.getRawHealthForComponent(cID)
		if !ok {
			continue
		}
		if targetID := rHealth.component.TargetID; targetID != "" {
			targetsToRemove[targetID] = struct{}{}
		}
		hm.registry.removeRawHealthForComponent(cID)
	}

	for _, rHealth := range hm.registry.getRawHealthMap() {
		// targets that have impacting components left should not be deleted
		delete(targetsToRemove, rHealth.component.TargetID)
	}

	for tcID := range targetsToRemove {
		hm.registry.removeRawHealthForComponent(tcID)
		hm.registry.removeTarget(tcID)
	}
}

func (hm *healthManager) SetPriority(componentType string, priority component.Priority) {
	hm.registry.lock()
	defer hm.registry.unlock()
	hm.registry.setPriorityForType(componentType, priority)
}

func (hm *healthManager) updateTargetsRegistry(c *component.Component) {
	if c == nil || c.TargetID == "" {
		return
	}
	tRegistered, tHeartbeatEnabled := hm.registry.checkTarget(c.TargetID)
	if !tRegistered {
		hm.registry.setTarget(c.TargetID, c.EnableHeartbeat)
		// If a target component is automatically created, whether its heartbeat is enabled will be determined
		// by the enabled heartbeat of at least one impacting component.
		// If the target component is added manually, the enabled heartbeat must be set explicitly
	} else if !tHeartbeatEnabled && c.EnableHeartbeat {
		hm.registry.setTarget(c.TargetID, true)
	}
}

func (hm *healthManager) sendComponentsInfo() {
	hm.registry.lock()
	defer hm.registry.unlock()

	for _, rawHealth := range hm.registry.getRawHealthMap() {
		hm.componentIn <- rawHealth.component
	}
}

// Health Manager listens for data from collector and stores it in a registry
func (hm *healthManager) listenMeasurements(ctx context.Context, measurements <-chan *ComponentMeasurement) {
	log := logging.GetLogger()
	log.Info().Msg("Start to listen for collected measurements")
	defer func() { log.Info().Msg("Finish listening for measurements from collector") }()
	for {
		select {
		case measurement, more := <-measurements:
			if !more {
				log.Warn().Msg("Measurements channel closed. Stop listening")
				return
			}

			hm.mu.Lock()

			err := hm.updateComponentHealthData(measurement)
			if err != nil {
				log.Error().AnErr("error", err).Msgf("Unable to update component with id %s", measurement.ComponentID)
			}

			hm.mu.Unlock()
		case <-hm.stopSig:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (hm *healthManager) updateComponentHealthData(measure *ComponentMeasurement) error {
	var err error

	hm.registry.lock()
	defer hm.registry.unlock()

	componentHealth, ok := hm.registry.getRawHealthForComponent(measure.ComponentID)
	if !ok {
		if !hm.registrationOnCollect() {
			return utils.ErrComponentNotRegistered
		}
		if len(hm.registry.getRawHealthMap())+1 > utils.ComponentsLimit {
			return utils.ErrComponentsLimitExceeded
		}
		componentHealth, err = hm.buildComponentFromMeasure(measure)
		if err != nil {
			return fmt.Errorf("unable to register component automatically: %w", err)
		}
		hm.registry.setRawHealthForComponent(componentHealth)
	}

	switch measure.MeasureType {
	case Heartbeat:
		componentHealth.heartBeat = true
	case Metric:
		err := hm.updateComponentsMetric(componentHealth, measure)
		if err != nil {
			return fmt.Errorf("unable to update component metric: %w", err)
		}
	case CounterChange:
		err := hm.updateComponentsCounter(componentHealth, measure)
		if err != nil {
			return fmt.Errorf("unable to update component counter: %w", err)
		}
	case Message:
		if measure.Message.AffectHealth {
			componentHealth.status = measure.Message.HealthStatus
		}
		componentHealth.messages = append(componentHealth.messages, measure.Message)
	case HealthStatus:
		componentHealth.status = measure.HealthStatus
	}

	debugComponentMeasure(measure)
	return nil
}

func (hm *healthManager) updateComponentsMetric(cHealth *rawHealth, measure *ComponentMeasurement) error {
	if _, metricOk := cHealth.component.Metrics[measure.MeasureID]; !metricOk {
		if !hm.registrationOnCollect() {
			return utils.ErrMetricNotRegistered
		}
		if cHealth.component.IsMeasuresLimitExceeded(1) {
			return utils.ErrComponentMeasuresLimitExceeded
		}
		if !cHealth.component.IsMeasureIDUnique(measure.MeasureID) {
			return utils.ErrMeasureIDTaken
		}
		cHealth.component.Metrics[measure.MeasureID] = component.DefaultAggregator
		cHealth.metrics[measure.MeasureID] = &metric{values: []float64{measure.MetricValue}}
	} else {
		cHealth.metrics[measure.MeasureID].values = append(
			cHealth.metrics[measure.MeasureID].values,
			measure.MetricValue,
		)
	}
	return nil
}

func (hm *healthManager) updateComponentsCounter(cHealth *rawHealth, measure *ComponentMeasurement) error {
	if slices.Contains(cHealth.component.TotalCounterIDs, measure.MeasureID) {
		cHealth.totalCounters[measure.MeasureID] += measure.CounterChange
	} else {
		if !slices.Contains(cHealth.component.CounterIDs, measure.MeasureID) {
			if !hm.registrationOnCollect() {
				return utils.ErrCounterNotRegistered
			}
			if cHealth.component.IsMeasuresLimitExceeded(1) {
				return utils.ErrComponentMeasuresLimitExceeded
			}
			if !cHealth.component.IsMeasureIDUnique(measure.MeasureID) {
				return utils.ErrMeasureIDTaken
			}
			cHealth.component.CounterIDs = append(cHealth.component.CounterIDs, measure.MeasureID)
		}
		cHealth.counters[measure.MeasureID] += measure.CounterChange
	}
	return nil
}

func (*healthManager) buildComponentFromMeasure(measure *ComponentMeasurement) (*rawHealth, error) {
	var enableHeartbeat bool
	metrics := make(map[string]component.Aggregator)
	counterIDs := make([]string, 0)
	totalCounterIDs := make([]string, 0)
	switch measure.MeasureType {
	case Heartbeat:
		enableHeartbeat = true
	case Metric:
		metrics[measure.MeasureID] = component.DefaultAggregator
	case CounterChange:
		counterIDs = []string{measure.MeasureID}
	}
	component, err := component.New(
		measure.ComponentID, utils.DefaultComponentType, "", enableHeartbeat,
		metrics, counterIDs, totalCounterIDs,
	)
	if err != nil { // shouldn't ever happen here
		return nil, err
	}
	return newRawHealth(component), nil
}

// Calculates raw health data from the registry and forwards all health data to the writer once per cycle
func (hm *healthManager) healthForwarder(ctx context.Context, configUpd <-chan *Config, healthIn chan<- *component.Health) {
	log := logging.GetLogger()
	log.Info().Msgf("Start to send health data to a writer with cycle %v", hm.collectionCycle())
	defer func() { log.Info().Msg("Finish to send health data to a writer") }()
	ticker := time.NewTicker(hm.collectionCycle())
	for {
		select {
		case <-ticker.C:
			hm.writeHealthResult(healthIn)
		case cfg, updated := <-configUpd:
			if updated {
				ticker.Reset(cfg.CollectionCycle)
				logging.GetLogger().Info().Msgf("Updated collection interval to %v", cfg.CollectionCycle)
			}
		case <-hm.stopSig:
			hm.writeHealthResult(healthIn)
			close(healthIn)
			return
		case <-ctx.Done():
			hm.writeHealthResult(healthIn)
			close(healthIn)
			return
		}
	}
}

type targetInfo struct {
	componentID string

	impactCounts map[component.HealthStatus]map[component.Priority]int
	impactHB     *component.HeartBeat
	impactMsgs   []*component.Message
	ownHealth    *component.Health
}

func newTarget(componentID string) *targetInfo {
	return &targetInfo{
		componentID: componentID,
		impactCounts: map[component.HealthStatus]map[component.Priority]int{
			component.Healthy:   make(map[component.Priority]int),
			component.Degrade:   make(map[component.Priority]int),
			component.Unhealthy: make(map[component.Priority]int),
		},
	}
}

func (hm *healthManager) writeHealthResult(healthIn chan<- *component.Health) {
	hm.registry.lock()
	defer hm.registry.unlock()

	targetsInfo := map[string]*targetInfo{}
	for _, rawHealth := range hm.registry.getRawHealthMap() {
		debugRawHealthStats(rawHealth)
		cHealth := hm.calculateComponentHealth(rawHealth)

		if isTargetComponent, hbEnabled := hm.registry.checkTarget(cHealth.ComponentID); isTargetComponent {
			if _, ok := targetsInfo[cHealth.ComponentID]; !ok {
				targetsInfo[cHealth.ComponentID] = newTarget(cHealth.ComponentID)
				targetsInfo[cHealth.ComponentID].impactHB = &component.HeartBeat{Enabled: hbEnabled}
			}
			targetsInfo[cHealth.ComponentID].ownHealth = cHealth
		} else {
			healthIn <- cHealth

			// current component impacts some target component and is not impacted by any other component
			if cHealth.TargetID != "" {
				if _, ok := targetsInfo[cHealth.TargetID]; !ok {
					targetsInfo[cHealth.TargetID] = newTarget(cHealth.TargetID)
				}
				hm.updateImpactedTargetInfo(targetsInfo[cHealth.TargetID], cHealth)
			}
		}

		hm.cleanHealthValues(rawHealth)
	}

	for tID, tInfo := range targetsInfo {
		// build and push health for target components starting with the highest level ones
		if tInfo.ownHealth.TargetID == "" {
			hm.buildAndPushHealthForTarget(tID, targetsInfo, healthIn)
		}
	}
}

func (*healthManager) calculateComponentHealth(rawHealth *rawHealth) *component.Health {
	health := component.NewHealth(rawHealth.component.ID, rawHealth.component.Type, rawHealth.component.TargetID)
	health.Status = rawHealth.status
	health.Heartbeat.Enabled = rawHealth.component.EnableHeartbeat
	health.Heartbeat.Beats = rawHealth.heartBeat
	health.Messages = rawHealth.messages
	for counterID, counter := range rawHealth.counters {
		health.Counters[counterID] = counter
	}
	for counterID, counter := range rawHealth.totalCounters {
		health.Counters[counterID] = counter
	}
	for metricID, m := range rawHealth.metrics {
		if len(m.values) > 0 {
			switch m.aggregator {
			case component.AggregatorMean:
				health.Metrics[metricID] = sdk_utils.Sum(m.values) / float64(len(m.values))
			case component.AggregatorMin:
				health.Metrics[metricID] = slices.Min(m.values)
			case component.AggregatorMax:
				health.Metrics[metricID] = slices.Max(m.values)
			case component.AggregatorSum:
				health.Metrics[metricID] = sdk_utils.Sum(m.values)
			case component.AggregatorCount:
				health.Metrics[metricID] = float64(len(m.values))
			}
		}
	}
	return health
}

func (*healthManager) cleanHealthValues(rawHealth *rawHealth) {
	rawHealth.heartBeat = false
	rawHealth.messages = []*component.Message{}
	for counterID := range rawHealth.counters {
		rawHealth.counters[counterID] = 0
	}
	for _, m := range rawHealth.metrics {
		if len(m.values) > 0 {
			m.values = []float64{}
		}
	}
}

func (hm *healthManager) buildAndPushHealthForTarget(
	currentTargetID string, targetsInfo map[string]*targetInfo, healthIn chan<- *component.Health,
) *component.Health {
	currentTargetInfo := targetsInfo[currentTargetID]

	for otherTargetID, otherTargetInfo := range targetsInfo {
		// if current target component is affected by a lower level target component
		// its health should be processed first to account for its impact
		if otherTargetInfo.ownHealth.TargetID == currentTargetID {
			impactingTC := hm.buildAndPushHealthForTarget(otherTargetID, targetsInfo, healthIn)
			hm.updateImpactedTargetInfo(currentTargetInfo, impactingTC)
		}
	}

	hm.resolveTargetHealthStatus(currentTargetInfo)
	hm.resolveTargetHeartbeat(currentTargetInfo)
	currentTargetInfo.ownHealth.Messages = append(currentTargetInfo.ownHealth.Messages, currentTargetInfo.impactMsgs...)
	healthIn <- currentTargetInfo.ownHealth
	return currentTargetInfo.ownHealth
}

func (hm *healthManager) updateImpactedTargetInfo(targetInfo *targetInfo, impactedBy *component.Health) {
	priority := hm.registry.getPriorityForType(impactedBy.ComponentType)
	targetInfo.impactCounts[impactedBy.Status][priority]++
	switch impactedBy.Status {
	case component.Degrade:
		targetInfo.impactMsgs = append(targetInfo.impactMsgs, &component.Message{
			Summary:      fmt.Sprintf("%s degraded", impactedBy.ComponentID),
			AffectHealth: true,
			HealthStatus: impactedBy.Status,
		})
	case component.Unhealthy:
		targetInfo.impactMsgs = append(targetInfo.impactMsgs, &component.Message{
			Summary:      fmt.Sprintf("%s unhealthy", impactedBy.ComponentID),
			AffectHealth: true,
			HealthStatus: impactedBy.Status,
		})
	default:
		// do nothing
	}

	if targetInfo.impactHB == nil {
		_, hbEnabled := hm.registry.checkTarget(targetInfo.componentID)
		targetInfo.impactHB = &component.HeartBeat{Enabled: hbEnabled}
	}
	if impactedBy.Heartbeat.Beats {
		targetInfo.impactHB.Beats = true
	}
}

func (hm *healthManager) resolveTargetHealthStatus(target *targetInfo) {
	target.ownHealth.Status = hm.targetHealthFn()(target.impactCounts)
}

func (*healthManager) resolveTargetHeartbeat(target *targetInfo) {
	if target.impactHB == nil {
		target.impactHB = &component.HeartBeat{}
	}

	target.ownHealth.Heartbeat = &component.HeartBeat{
		Enabled: target.ownHealth.Heartbeat.Enabled || target.impactHB.Enabled,
		Beats:   target.ownHealth.Heartbeat.Beats || target.impactHB.Beats,
	}
}

func defaultTargetHealthFn(impactCounts map[component.HealthStatus]map[component.Priority]int) component.HealthStatus {
	var healthyCount, unhealthyCount, totalCount int
	for healthStatus, impactsByHealth := range impactCounts {
		countByHealth := sdk_utils.Sum(maps.Values(impactsByHealth))
		switch healthStatus {
		case component.Healthy:
			healthyCount = countByHealth
		case component.Unhealthy:
			unhealthyCount = countByHealth
		}
		totalCount += countByHealth
	}

	r := float64(totalCount-healthyCount) / float64(totalCount)
	switch {
	case unhealthyCount >= 1 || (totalCount > 2 && r >= 0.5):
		// at least one component is unhealthy or half or more are degraded -> target is unhealthy
		return component.Unhealthy
	case r > 0:
		// one or less than half of the components are degraded -> target is degraded
		return component.Degrade
	default:
		return component.Healthy
	}
}

// methods that help with debug logging functionality
func debugComponentMeasure(measure *ComponentMeasurement) {
	log := logging.GetLogger()
	switch measure.MeasureType {
	case Heartbeat:
		log.Debug().Msgf("Got heartbeat from %s component", measure.ComponentID)
	case Metric:
		log.Debug().Msgf("Got metric %s with %f value from %s component",
			measure.MeasureID, measure.MetricValue, measure.ComponentID)
	case CounterChange:
		log.Debug().Msgf("Got update for %s counter with %d value from %s component",
			measure.MeasureID, measure.CounterChange, measure.ComponentID)
	case Message:
		if measure.Message.AffectHealth {
			log.Debug().Msgf("Got message that affects health from %s component: status=%s, msg=%s, error.msg=%s",
				measure.MeasureID, measure.Message.HealthStatus, measure.Message.Summary, measure.Message.Error)
		} else {
			log.Debug().Msgf("Got message that doesn't affect health from %s component: msg=%s, error.msg=%s",
				measure.MeasureID, measure.Message.Summary, measure.Message.Error)
		}
	case HealthStatus:
		log.Debug().Msgf("Got health status update from %s component. %s", measure.ComponentID, measure.HealthStatus)
	}
}

func debugRawHealthStats(rawHealth *rawHealth) {
	log := logging.GetLogger()
	messageSums := make([]string, len(rawHealth.messages))
	for i, message := range rawHealth.messages {
		messageSums[i] = message.Summary
	}

	log.Debug().Msgf(
		"ComponentID: %s, Status=%v, Heartbeat=%t, Counters=%v, Metrics=%v, Messages=%v",
		rawHealth.component.ID, rawHealth.status, rawHealth.heartBeat, rawHealth.counters,
		rawHealth.metrics, messageSums,
	)
}

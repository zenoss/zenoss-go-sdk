/*
Package health implements a simple tool for health data + metrics collection.

We defines three main parts of the health monitorin tool: manager, collector and writer.
- Writer description is avilable in its own package.
- Collector is stored in collector.go file. It provides a simple interface that allows you to collect
different type of health data per target. All methods should have targetID as a parameter. Collector
will automatically send data to health manager.
- Manger is a heart of the health collection tool. It initialize all comunication channels and has
a control over collector and writer. Manager keeps all data collected by collector, calculates every
target health once per cycle and sends calculated data to writer.
*/
package health

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
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

	measurementsCh := make(chan *TargetMeasurement)
	healthCh := make(chan *target.Health)
	targetCh := make(chan *target.Target)

	c := NewCollector(cfg.CollectionCycle, measurementsCh)
	SetCollectorSingleton(c)

	go func() {
		<-ctx.Done()
		StopCollectorSingleton()
	}()

	go m.Start(ctx, measurementsCh, healthCh, targetCh)

	writer.Addwg()
	go writer.Start(ctx, healthCh, targetCh)

	return func() {
		StopCollectorSingleton()
		m.Shutdown()
		writer.Shutdown()
	}
}

// Manager keeps all information about targets and provides a functionality
// to initialize collector, writer and communication channels
type Manager interface {
	// Start method should initialize collector with it's configuration and
	// define a method to send data to a writer. Remember that healthIn and targetIn channels
	// require readers and measureOut requires a writer.
	// Make sure to close measureOut before manager shutdown to not lose any data.
	Start(
		ctx context.Context, measureOut <-chan *TargetMeasurement,
		healthIn chan<- *target.Health, targetIn chan<- *target.Target,
	)
	// Shutdown method closes manager's channels and terminates goroutines
	Shutdown()
	// IsStarted return the status of the manager
	IsStarted() bool
	// AddTargets provides a simple interface to register monitored targets
	AddTargets(targets []*target.Target)
}

// NewManager creates a new Manager instance with internal target registry.
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
	registry healthRegistry
	config   *Config
	healthIn chan<- *target.Health
	targetIn chan<- *target.Target

	// channel that block shutdown function from return until manager is stopped
	stopWait chan struct{}
	// channel that used by shutdown call to stop the manager
	stopSig chan struct{}
	// used to wait for manager processes to stop so we can mark started as false in a right time
	wg *sync.WaitGroup

	mu      sync.Mutex
	started atomic.Bool
}

func (hm *healthManager) Start(
	ctx context.Context,
	measureOut <-chan *TargetMeasurement,
	healthIn chan<- *target.Health,
	targetIn chan<- *target.Target,
) {
	hm.mu.Lock()
	hm.targetIn = targetIn
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
		hm.healthForwarder(ctx, healthIn)
	}()

	hm.started.Store(true)
	go func() {
		hm.wg.Wait()
		hm.started.Store(false)
		hm.stopWait <- struct{}{}
	}()

	hm.sendTargetsInfo() // if some targets where added before start we need to register them
}

func (hm *healthManager) Shutdown() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	close(hm.stopSig)
	<-hm.stopWait
	close(hm.targetIn)
}

func (hm *healthManager) IsStarted() bool {
	return hm.started.Load()
}

func (hm *healthManager) AddTargets(targets []*target.Target) {
	hm.registry.lock()
	defer hm.registry.unlock()

	for _, newTarget := range targets {
		hm.registry.setRawHealthForTarget(newRawHealth(newTarget))
		if hm.IsStarted() {
			hm.targetIn <- newTarget
		}
	}
}

func (hm *healthManager) sendTargetsInfo() {
	hm.registry.lock()
	defer hm.registry.unlock()

	for _, rawHealth := range hm.registry.getRawHealthMap() {
		hm.targetIn <- rawHealth.target
	}
}

// Health Manager listens for data from collector and stores it in a registry
func (hm *healthManager) listenMeasurements(ctx context.Context, measurements <-chan *TargetMeasurement) {
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

			err := hm.updateTargetHealthData(measurement)
			if err != nil {
				log.Error().AnErr("error", err).Msgf("Unable to update target with id %s", measurement.TargetID)
			}

			hm.mu.Unlock()
		case <-hm.stopSig:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (hm *healthManager) updateTargetHealthData(measure *TargetMeasurement) error {
	var err error

	hm.registry.lock()
	defer hm.registry.unlock()

	targetHealth, ok := hm.registry.getRawHealthForTarget(measure.TargetID)
	if !ok {
		if !hm.config.RegistrationOnCollect {
			return utils.ErrTargetNotRegistered
		}
		targetHealth, err = hm.buildTargetFromMeasure(measure)
		if err != nil {
			return fmt.Errorf("Unable to register target automatically: %w", err)
		}
		hm.registry.setRawHealthForTarget(targetHealth)
	}

	switch measure.MeasureType {
	case Heartbeat:
		targetHealth.heartBeat = true
	case Metric:
		err := hm.updateTargetsMetric(targetHealth, measure)
		if err != nil {
			return fmt.Errorf("Unable to update target metric: %w", err)
		}
	case CounterChange:
		err := hm.updateTargetsCounter(targetHealth, measure)
		if err != nil {
			return fmt.Errorf("Unable to update target counter: %w", err)
		}
	case Message:
		if measure.Message.AffectHealth {
			targetHealth.status = measure.Message.HealthStatus
		}
		targetHealth.messages = append(targetHealth.messages, measure.Message)
	case HealthStatus:
		targetHealth.status = measure.HealthStatus
	}

	debugTargetMeasure(measure)
	return nil
}

func (hm *healthManager) updateTargetsMetric(tHealth *rawHealth, measure *TargetMeasurement) error {
	if !sdk_utils.ListContainsString(tHealth.target.MetricIDs, measure.MeasureID) {
		if !hm.config.RegistrationOnCollect {
			return utils.ErrMetricNotRegistered
		}
		if !tHealth.target.IsMeasureIDUnique(measure.MeasureID) {
			return utils.ErrMeasureIDTaken
		}
		tHealth.target.MetricIDs = append(tHealth.target.MetricIDs, measure.MeasureID)
		tHealth.rawMetrics[measure.MeasureID] = []float64{measure.MetricValue}
	} else {
		tHealth.rawMetrics[measure.MeasureID] = append(
			tHealth.rawMetrics[measure.MeasureID],
			measure.MetricValue,
		)
	}
	return nil
}

func (hm *healthManager) updateTargetsCounter(tHealth *rawHealth, measure *TargetMeasurement) error {
	if sdk_utils.ListContainsString(tHealth.target.TotalCounterIDs, measure.MeasureID) {
		tHealth.totalCounters[measure.MeasureID] += measure.CounterChange
	} else {
		if !sdk_utils.ListContainsString(tHealth.target.CounterIDs, measure.MeasureID) {
			if !hm.config.RegistrationOnCollect {
				return utils.ErrCounterNotRegistered
			}
			if !tHealth.target.IsMeasureIDUnique(measure.MeasureID) {
				return utils.ErrMeasureIDTaken
			}
			tHealth.target.CounterIDs = append(tHealth.target.CounterIDs, measure.MeasureID)
		}
		tHealth.counters[measure.MeasureID] += measure.CounterChange
	}
	return nil
}

func (*healthManager) buildTargetFromMeasure(measure *TargetMeasurement) (*rawHealth, error) {
	var enableHeartbeat bool
	metricIDs := make([]string, 0)
	counterIDs := make([]string, 0)
	totalCounterIDs := make([]string, 0)
	switch measure.MeasureType {
	case Heartbeat:
		enableHeartbeat = true
	case Metric:
		metricIDs = []string{measure.MeasureID}
	case CounterChange:
		counterIDs = []string{measure.MeasureID}
	}
	target, err := target.New(
		measure.TargetID, utils.DefaultTargetType, enableHeartbeat,
		metricIDs, counterIDs, totalCounterIDs,
	)
	if err != nil { // shouldn't ever happen here
		return nil, err
	}
	return newRawHealth(target), nil
}

// Calculates raw health data from the registry and forwards all health data to the writer once per cycle
func (hm *healthManager) healthForwarder(ctx context.Context, healthIn chan<- *target.Health) {
	log := logging.GetLogger()
	log.Info().Msgf("Start to send health data to a writer with cycle %v", hm.config.CollectionCycle)
	defer func() { log.Info().Msg("Finish to send health data to a writer") }()
	ticker := time.NewTicker(hm.config.CollectionCycle)
	for {
		select {
		case <-ticker.C:
			hm.writeHealthResult(healthIn)
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

func (hm *healthManager) writeHealthResult(healthIn chan<- *target.Health) {
	hm.registry.lock()
	defer hm.registry.unlock()

	for _, rawHealth := range hm.registry.getRawHealthMap() {
		debugRawHealthStats(rawHealth)
		tHealth := hm.calculateTargetHealth(rawHealth)
		healthIn <- tHealth
		hm.cleanHealthValues(rawHealth)
	}
}

func (*healthManager) calculateTargetHealth(rawHealth *rawHealth) *target.Health {
	health := target.NewHealth(rawHealth.target.ID, rawHealth.target.Type)
	health.Status = rawHealth.status
	health.Heartbeat.Enabled = rawHealth.target.EnableHeartbeat
	health.Heartbeat.Beats = rawHealth.heartBeat
	health.Messages = rawHealth.messages
	for counterID, counter := range rawHealth.counters {
		health.Counters[counterID] = counter
	}
	for counterID, counter := range rawHealth.totalCounters {
		health.Counters[counterID] = counter
	}
	for metricID, mValues := range rawHealth.rawMetrics {
		if len(mValues) > 0 {
			var sum float64
			for _, value := range mValues {
				sum += value
			}
			health.Metrics[metricID] = sum / float64(len(mValues))
		}
	}
	return health
}

func (*healthManager) cleanHealthValues(rawHealth *rawHealth) {
	rawHealth.heartBeat = false
	rawHealth.messages = []*target.Message{}
	for counterID := range rawHealth.counters {
		rawHealth.counters[counterID] = 0
	}
	for metricID, mValues := range rawHealth.rawMetrics {
		if len(mValues) > 0 {
			rawHealth.rawMetrics[metricID] = []float64{}
		}
	}
}

// methods that help with debug logging functionality
func debugTargetMeasure(measure *TargetMeasurement) {
	log := logging.GetLogger()
	switch measure.MeasureType {
	case Heartbeat:
		log.Debug().Msgf("Got heartbeat from %s target", measure.TargetID)
	case Metric:
		log.Debug().Msgf("Got metric %s with %f value from %s target",
			measure.MeasureID, measure.MetricValue, measure.TargetID)
	case CounterChange:
		log.Debug().Msgf("Got update for %s counter with %d value from %s target",
			measure.MeasureID, measure.CounterChange, measure.TargetID)
	case Message:
		if measure.Message.AffectHealth {
			log.Debug().Msgf("Got message that affects health from %s target: status=%s, msg=%s, error.msg=%s",
				measure.MeasureID, measure.Message.HealthStatus, measure.Message.Summary, measure.Message.Error)
		} else {
			log.Debug().Msgf("Got message that doesn't affect health from %s target: msg=%s, error.msg=%s",
				measure.MeasureID, measure.Message.Summary, measure.Message.Error)
		}
	case HealthStatus:
		log.Debug().Msgf("Got health status update from %s target. %s", measure.TargetID, measure.HealthStatus)
	}
}

func debugRawHealthStats(rawHealth *rawHealth) {
	log := logging.GetLogger()
	messageSums := make([]string, len(rawHealth.messages))
	for i, message := range rawHealth.messages {
		messageSums[i] = message.Summary
	}

	log.Debug().Msgf(
		"TargetID: %s, Status=%v, Heartbeat=%t, Counters=%v, Metrics=%v, Messages=%v",
		rawHealth.target.ID, rawHealth.status, rawHealth.heartBeat, rawHealth.counters,
		rawHealth.rawMetrics, messageSums,
	)
}

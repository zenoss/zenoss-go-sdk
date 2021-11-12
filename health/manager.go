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
	"time"

	logging "github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	w "github.com/zenoss/zenoss-go-sdk/health/writer"
	"github.com/zenoss/zenoss-go-sdk/utils"
)

// Manager keeps all information about targets and provides a functionality
// to initialize collector, writer and communication channels
type Manager interface {
	// Start method should initialize collector with it's configuration and
	// define a method to send data to a writer
	Start(ctx context.Context)
	// AddTargets provides a simple interface to register monitored targets
	AddTargets(targets []*target.Target)
}

// NewManager creates a new Manager instance with internal target registry.
// You should init configuration and writer before and pass it here
func NewManager(ctx context.Context, config *Config, writer w.HealthWriter) Manager {
	healthReg := newHealthRegistry()

	return &healthManager{
		registry: healthReg,
		config:   config,
		writer:   writer,
	}
}

// healthManager implements Manager interface
type healthManager struct {
	registry healthRegistry
	config   *Config
	writer   w.HealthWriter
}

func (hm *healthManager) Start(ctx context.Context) {
	logging.SetLogLevel(hm.config.LogLevel)
	measurements := make(chan *targetMeasurement)
	initCollector(hm.config.CollectionCycle, measurements)
	go hm.listenMeasurements(ctx, measurements)

	healthIn := make(chan *target.Health)
	go hm.healthForwarder(ctx, healthIn)
	go hm.writer.Start(healthIn)
}

func (hm *healthManager) AddTargets(targets []*target.Target) {
	hm.registry.lock()
	defer hm.registry.unlock()

	for _, target := range targets {
		hm.registry.setRawHealthForTarget(newRawHealth(target, target.MetricIDs))
	}
}

// Health Manager listens for data from collector and stores it in a registry
func (hm *healthManager) listenMeasurements(ctx context.Context, measurements <-chan *targetMeasurement) {
	log := logging.GetLogger()
	log.Info().Msg("Start to listen for collected measurements")
	defer log.Info().Msg("Finish listening for measurements from collector")
	for {
		select {
		case measurement := <-measurements:
			err := hm.updateTargetHealthData(measurement)
			if err != nil {
				log.Error().AnErr("error", err).Msgf("Unable to update target with id %s", measurement.targetID)
			}
		case <-ctx.Done():
			shutDownCollector()
			return
		}
	}
}

func (hm *healthManager) updateTargetHealthData(measure *targetMeasurement) error {
	var err error

	hm.registry.lock()
	defer hm.registry.unlock()

	targetHealth, ok := hm.registry.getRawHealthForTarget(measure.targetID)
	if !ok {
		if !hm.config.RegistrationOnCollect {
			return errTargetNotRegistered
		}
		targetHealth, err = hm.buildTargetFromMeasure(measure)
		if err != nil {
			return fmt.Errorf("Unable to register target automatically: %w", err)
		}
	}

	switch measure.measureType {
	case heartbeat:
		targetHealth.heartBeat = true
	case metric:
		err := hm.updateTargetsMetric(targetHealth, measure)
		if err != nil {
			return fmt.Errorf("Unable to update target metric: %w", err)
		}
	case counterChange:
		err := hm.updateTargetsCounter(targetHealth, measure)
		if err != nil {
			return fmt.Errorf("Unable to update target counter: %w", err)
		}
	case message:
		if measure.message.AffectHealth {
			targetHealth.status = measure.message.HealthStatus
		}
		targetHealth.messages = append(targetHealth.messages, measure.message)
	case healthStatus:
		targetHealth.status = measure.healthStatus
	}

	debugTargetMeasure(measure)
	return nil
}

func (hm *healthManager) updateTargetsMetric(tHealth *rawHealth, measure *targetMeasurement) error {
	if !utils.ListContainsString(tHealth.target.MetricIDs, measure.measureID) {
		if !hm.config.RegistrationOnCollect {
			return errMetricNotRegistered
		}
		if !tHealth.target.IsMeasureIDUnique(measure.measureID) {
			return errMeasureIDTaken
		}
		tHealth.target.MetricIDs = append(tHealth.target.MetricIDs, measure.measureID)
		tHealth.rawMetrics[measure.measureID] = []float64{measure.metricValue}
	} else {
		tHealth.rawMetrics[measure.measureID] = append(
			tHealth.rawMetrics[measure.measureID],
			measure.metricValue,
		)
	}
	return nil
}

func (hm *healthManager) updateTargetsCounter(tHealth *rawHealth, measure *targetMeasurement) error {
	if utils.ListContainsString(tHealth.target.TotalCounterIDs, measure.measureID) {
		tHealth.totalCounters[measure.measureID] += measure.counterChange
	} else {
		if !utils.ListContainsString(tHealth.target.CounterIDs, measure.measureID) {
			if !hm.config.RegistrationOnCollect {
				return errCounterNotRegistered
			}
			if !tHealth.target.IsMeasureIDUnique(measure.measureID) {
				return errMeasureIDTaken
			}
			tHealth.target.CounterIDs = append(tHealth.target.CounterIDs, measure.measureID)
		}
		tHealth.counters[measure.measureID] += measure.counterChange
	}
	return nil
}

func (hm *healthManager) buildTargetFromMeasure(measure *targetMeasurement) (*rawHealth, error) {
	var enableHeartbeat bool
	metricIDs := make([]string, 0)
	counterIDs := make([]string, 0)
	totalCounterIDs := make([]string, 0)
	switch measure.measureType {
	case heartbeat:
		enableHeartbeat = true
	case metric:
		metricIDs = []string{measure.measureID}
	case counterChange:
		counterIDs = []string{measure.measureID}
	}
	target, err := target.New(measure.targetID, enableHeartbeat, metricIDs, counterIDs, totalCounterIDs)
	if err != nil {
		return nil, err
	}
	return newRawHealth(target, metricIDs), nil
}

// Calculates raw health data from the registry and forwards all health data to the writer once per cycle
func (hm *healthManager) healthForwarder(ctx context.Context, healthIn chan<- *target.Health) {
	log := logging.GetLogger()
	log.Info().Msgf("Start to send health data to a writer with cycle %v", hm.config.CollectionCycle)
	defer log.Info().Msg("Finish to send health data to a writer")

	ticker := time.NewTicker(hm.config.CollectionCycle)
	for {
		select {
		case <-ticker.C:
			hm.writeHealthResult(healthIn)
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

func (hm *healthManager) calculateTargetHealth(rawHealth *rawHealth) *target.Health {
	health := target.NewHealth(rawHealth.target.ID)
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

func (hm *healthManager) cleanHealthValues(rawHealth *rawHealth) {
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
func debugTargetMeasure(measure *targetMeasurement) {
	log := logging.GetLogger()
	switch measure.measureType {
	case heartbeat:
		log.Debug().Msgf("Got heartbeat from %s target", measure.targetID)
	case metric:
		log.Debug().Msgf("Got metric %s with %f value from %s target",
			measure.measureID, measure.metricValue, measure.targetID)
	case counterChange:
		log.Debug().Msgf("Got update for %s counter with %d value from %s target",
			measure.measureID, measure.counterChange, measure.targetID)
	case message:
		if measure.message.AffectHealth {
			log.Debug().Msgf("Got message that affects health from %s target: status=%s, msg=%s, error.msg=%s",
				measure.measureID, measure.message.HealthStatus, measure.message.Summary, measure.message.Error)
		} else {
			log.Debug().Msgf("Got message that doesn't affect health from %s target: msg=%s, error.msg=%s",
				measure.measureID, measure.message.Summary, measure.message.Error)
		}
	case healthStatus:
		log.Debug().Msgf("Got health status update from %s target. %s", measure.targetID, measure.healthStatus)
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

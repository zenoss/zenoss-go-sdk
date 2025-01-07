package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/component"
	"github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

const (
	mercedesComponent = "mercedes.citaro"
	bogdanComponent   = "bogdan.A091"

	speedMetricID       = "speed"
	stationsCounterID   = "stations"
	passengersCounterID = "passengers"
)

func main() {
	ctx := context.Background()

	// Define health tool configuration
	config := health.NewConfig()
	config.CollectionCycle = 2 * time.Second

	// define monitored components
	busComponent, err := component.New(
		mercedesComponent, "", true,
		[]string{speedMetricID},
		[]string{stationsCounterID},
		[]string{passengersCounterID},
	)
	if err != nil {
		panic(err)
	}
	components := []*component.Component{
		busComponent,
	}

	// Define writer and its destination
	logDestination := writer.NewLogDestination(log.GetLogger())
	writer := writer.New([]writer.Destination{logDestination})

	// init health manager
	manager := health.NewManager(ctx, config)
	manager.AddComponents(components)

	// start health monitoring framework
	// after this you are safe to call collector in any part of your program
	frameworkStop := health.FrameworkStart(ctx, config, manager, writer)

	time.Sleep(1 * time.Second)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go bus(manager, wg)
	wg.Wait()

	frameworkStop()
}

func bus(manager health.Manager, wg *sync.WaitGroup) {
	log := log.GetLogger()
	defer wg.Done()

	log.Info().Msg("Bus is started")
	collector, err := health.GetCollectorSingleton()
	if err != nil {
		panic(err)
	}
	sleeps := 2 * time.Second

	hbCancel, err := collector.HeartBeat(mercedesComponent)
	if err != nil {
		panic(err)
	}
	defer hbCancel()

	// just started
	collector.AddMetricValue(mercedesComponent, speedMetricID, 0)

	time.Sleep(sleeps)
	log.Info().Msg("Bus is keep moving")

	collector.AddMetricValue(mercedesComponent, speedMetricID, 35.4)

	time.Sleep(sleeps)
	log.Info().Msg("Need to stop on the station")

	collector.AddMetricValue(mercedesComponent, speedMetricID, 0)
	collector.AddToCounter(mercedesComponent, stationsCounterID, 1)
	collector.AddToCounter(mercedesComponent, passengersCounterID, 8)

	time.Sleep(sleeps)
	log.Info().Msg("And we move again")

	collector.AddMetricValue(mercedesComponent, speedMetricID, 8.9)

	time.Sleep(sleeps)
	log.Info().Msg("OH no. Somethign happened")

	msg := component.NewMessage(
		"The engine stalled",
		errors.New("engine stopped working"),
		true, component.Unhealthy)
	collector.HealthMessage(mercedesComponent, msg)
	collector.AddMetricValue(mercedesComponent, speedMetricID, 0)

	time.Sleep(sleeps)
	log.Info().Msg("Two passengers where not patient and left")

	collector.AddToCounter(mercedesComponent, passengersCounterID, -2)

	time.Sleep(sleeps)
	log.Info().Msg("Congrats, we repaired an engine, we can mark as healthy again")

	collector.ChangeHealth(mercedesComponent, component.Healthy)
	collector.AddMetricValue(mercedesComponent, speedMetricID, 5.0)
	collector.AddMetricValue(mercedesComponent, speedMetricID, 25.0)

	time.Sleep(sleeps / 2)

	log.Info().Msg("An unregistered vehicle appears at a crossroads")
	collector.AddMetricValue(bogdanComponent, speedMetricID, 2)
	time.Sleep(sleeps / 2)

	log.Info().Msg("Updating the config to monitor it")
	cfg := health.NewConfig()
	cfg.RegistrationOnCollect = true
	cfg.CollectionCycle = 4 * time.Second
	manager.UpdateConfig(cfg)
	sleeps = 4 * time.Second
	time.Sleep(sleeps / 2)

	collector.AddMetricValue(bogdanComponent, speedMetricID, 4.0)
	collector.AddMetricValue(mercedesComponent, speedMetricID, 5.0)
	time.Sleep(sleeps)

	collector.AddMetricValue(bogdanComponent, speedMetricID, 7.0)
	collector.AddMetricValue(mercedesComponent, speedMetricID, 11.0)
	time.Sleep(sleeps)
	log.Info().Msg("Buses keep moving but we don't need to monitor them anymore")
}

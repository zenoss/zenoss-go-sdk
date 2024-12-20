package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zenoss/zenoss-go-sdk/health"
	"github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
	"github.com/zenoss/zenoss-go-sdk/health/writer"
)

const (
	mercedesTarget = "mercedes.citaro"
	bogdanTarget   = "bogdan.A091"

	speedMetricID       = "speed"
	stationsCounterID   = "stations"
	passengersCounterID = "passengers"
)

func main() {
	ctx := context.Background()

	// Define health tool configuration
	config := health.NewConfig()
	config.CollectionCycle = 2 * time.Second

	// define monitored targets
	busTarget, err := target.New(
		mercedesTarget, "", true,
		[]string{speedMetricID},
		[]string{stationsCounterID},
		[]string{passengersCounterID},
	)
	if err != nil {
		panic(err)
	}
	targets := []*target.Target{
		busTarget,
	}

	// Define writer and its destination
	logDestination := writer.NewLogDestination(log.GetLogger())
	writer := writer.New([]writer.Destination{logDestination})

	// init health manager
	manager := health.NewManager(ctx, config)
	manager.AddTargets(targets)

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

	hbCancel, err := collector.HeartBeat(mercedesTarget)
	if err != nil {
		panic(err)
	}
	defer hbCancel()

	// just started
	collector.AddMetricValue(mercedesTarget, speedMetricID, 0)

	time.Sleep(sleeps)
	log.Info().Msg("Bus is keep moving")

	collector.AddMetricValue(mercedesTarget, speedMetricID, 35.4)

	time.Sleep(sleeps)
	log.Info().Msg("Need to stop on the station")

	collector.AddMetricValue(mercedesTarget, speedMetricID, 0)
	collector.AddToCounter(mercedesTarget, stationsCounterID, 1)
	collector.AddToCounter(mercedesTarget, passengersCounterID, 8)

	time.Sleep(sleeps)
	log.Info().Msg("And we move again")

	collector.AddMetricValue(mercedesTarget, speedMetricID, 8.9)

	time.Sleep(sleeps)
	log.Info().Msg("OH no. Somethign happened")

	msg := target.NewMessage(
		"The engine stalled",
		errors.New("engine stopped working"),
		true, target.Unhealthy)
	collector.HealthMessage(mercedesTarget, msg)
	collector.AddMetricValue(mercedesTarget, speedMetricID, 0)

	time.Sleep(sleeps)
	log.Info().Msg("Two passengers where not patient and left")

	collector.AddToCounter(mercedesTarget, passengersCounterID, -2)

	time.Sleep(sleeps)
	log.Info().Msg("Congrats, we repaired an engine, we can mark as healthy again")

	collector.ChangeHealth(mercedesTarget, target.Healthy)
	collector.AddMetricValue(mercedesTarget, speedMetricID, 5.0)
	collector.AddMetricValue(mercedesTarget, speedMetricID, 25.0)

	time.Sleep(sleeps / 2)

	log.Info().Msg("An unregistered vehicle appears at a crossroads")
	collector.AddMetricValue(bogdanTarget, speedMetricID, 2)
	time.Sleep(sleeps / 2)

	log.Info().Msg("Updating the config to monitor it")
	cfg := health.NewConfig()
	cfg.RegistrationOnCollect = true
	cfg.CollectionCycle = 4 * time.Second
	manager.UpdateConfig(cfg)
	sleeps = 4 * time.Second
	time.Sleep(sleeps / 2)

	collector.AddMetricValue(bogdanTarget, speedMetricID, 4.0)
	collector.AddMetricValue(mercedesTarget, speedMetricID, 5.0)
	time.Sleep(sleeps)

	collector.AddMetricValue(bogdanTarget, speedMetricID, 7.0)
	collector.AddMetricValue(mercedesTarget, speedMetricID, 11.0)
	time.Sleep(sleeps)
	log.Info().Msg("Buses keep moving but we don't need to monitor them anymore")
}

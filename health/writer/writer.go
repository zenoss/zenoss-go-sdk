/*
Package writer implements health data writer

It listens channels provided by health manager and pushes data to defined destinations
*/
package writer

import (
	"context"
	"sync"

	"github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

// HealthWriter provides an ability to forward health data from manager to destinations
type HealthWriter interface {
	// Start should be run in goroutine.
	// It listens for healthIn channel and sends data to configured destinations
	Start(ctx context.Context, healthIn <-chan *target.Health, targetIn <-chan *target.Target, stopSig <-chan struct{})

	Shutdown()
}

// New creates a new HealthWriter instance.
// Destinations should be initialized and passed here as a parameter
func New(dests []Destination) HealthWriter {
	return &writer{destinations: dests, wg: &sync.WaitGroup{}}
}

type writer struct {
	destinations []Destination
	// we can also create and add some data transformers if
	// health data should be changed somehow before send
	wg *sync.WaitGroup
}

func (w *writer) Start(ctx context.Context, healthIn <-chan *target.Health, targetIn <-chan *target.Target,
	stopSig <-chan struct{},
) {
	w.wg.Add(1)
	defer w.wg.Done()

	for {
		select {
		case healthData, more := <-healthIn:
			if !more {
				return
			}
			for _, dest := range w.destinations {
				err := dest.Push(ctx, healthData)
				if err != nil {
					log.GetLogger().Error().AnErr("error", err).Msg("Unable to push health message")
				}
			}
		case targetData, more := <-targetIn:
			if !more {
				return
			}
			for _, dest := range w.destinations {
				err := dest.Register(ctx, targetData)
				if err != nil {
					log.GetLogger().Error().AnErr("error", err).Msg("Unable to register target")
				}
			}
		case <-stopSig:
			return
		}
	}
}

func (w *writer) Shutdown() {
	w.wg.Wait()
}

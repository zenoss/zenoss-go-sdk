/*
Package writer implements health data writer

It listens a channel provided by health manager and pushes data to defined destinations
*/
package writer

import (
	"github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

// HealthWriter provides an ability to forward health data from manager to destinations
type HealthWriter interface {
	// Start should be run in goroutine.
	// It listens for healthIn channel and sends data to configured destinations
	Start(healthIn <-chan *target.Health)
}

// New creates a new HealthWriter instance.
// Destinations should be initialized and passed here as a parameter
func New(dests []Destination) HealthWriter {
	return &writer{destinations: dests}
}

type writer struct {
	destinations []Destination
	// we can also create and add some data transformers if
	// health data should be changed somehow before send
}

func (w *writer) Start(healthIn <-chan *target.Health) {
	for {
		select {
		case healthData, more := <-healthIn:
			if !more {
				return
			}
			for _, dest := range w.destinations {
				err := dest.Push(healthData)
				if err != nil {
					log.GetLogger().Error().AnErr("error", err).Msg("Unable to push health message")
				}
			}
		}
	}
}

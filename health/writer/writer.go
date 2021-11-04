/*
Package writer implements health data writer

It listens a channel provided by health manager and pushes data to defined destination
*/
package writer

import (
	"github.com/zenoss/zenoss-go-sdk/health/log"
	"github.com/zenoss/zenoss-go-sdk/health/target"
)

type HealthWriter interface {
	// Start should be run in goroutine.
	// It listens for healthIn channel and sends data to configured destination
	Start(healthIn <-chan *target.Health)
}

// New creates a new HealthWriter instance.
// Destination should be initialized and passed here as a parameter
func New(dest Destination) HealthWriter {
	return &writer{destination: dest}
}

type writer struct {
	// it can be a list of Destinations in future if needed
	destination Destination
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
			err := w.destination.Push(healthData)
			if err != nil {
				log.GetLogger().Error().AnErr("error", err).Msg("Unable to push health message")
			}
		}
	}
}

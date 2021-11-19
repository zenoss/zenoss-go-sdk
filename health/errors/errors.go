package errors

import "errors"

var (
	// Collector errors
	ErrDeadCollector = errors.New("Collector is not running")

	// Manager errors
	ErrTargetNotRegistered  = errors.New("Target is not registered")
	ErrMetricNotRegistered  = errors.New("Metric is not registered on target")
	ErrCounterNotRegistered = errors.New("Counter is not registered on target")
	ErrMeasureIDTaken       = errors.New("Measure with such ID already exist")
	ErrManagerNotStarted    = errors.New("Health manager is not started")
)

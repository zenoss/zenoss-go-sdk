package health

import "errors"

var (
	// Collector errors
	DeadCollectorErr = errors.New("Collector is not running")

	// Manager errors
	TargetNotRegisteredErr  = errors.New("Target is not registered")
	MetricNotRegisteredErr  = errors.New("Metric is not registered on target")
	CounterNotRegisteredErr = errors.New("Counter is not registered on target")
	MeasureIDTakenErr       = errors.New("Measure with such ID already exist")
)

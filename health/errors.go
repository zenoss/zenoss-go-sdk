package health

import "errors"

var (
	// Collector errors
	errDeadCollector = errors.New("Collector is not running")

	// Manager errors
	errTargetNotRegistered  = errors.New("Target is not registered")
	errMetricNotRegistered  = errors.New("Metric is not registered on target")
	errCounterNotRegistered = errors.New("Counter is not registered on target")
	errMeasureIDTaken       = errors.New("Measure with such ID already exist")
)

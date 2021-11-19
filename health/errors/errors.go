package errors

import "errors"

var (
	// ErrDeadCollector defines not initialized or dead collector error
	ErrDeadCollector = errors.New("Collector is not running")

	// ErrTargetNotRegistered defines unknown monitoring target ID error
	ErrTargetNotRegistered = errors.New("Target is not registered")
	// ErrMetricNotRegistered defines unknown metric ID error
	ErrMetricNotRegistered = errors.New("Metric is not registered on target")
	// ErrCounterNotRegistered defines unknown counter ID error
	ErrCounterNotRegistered = errors.New("Counter is not registered on target")
	// ErrMeasureIDTaken defines measure ID already exists error
	ErrMeasureIDTaken = errors.New("Measure with such ID already exist")
	// ErrManagerNotStarted defines not initialized manager error
	ErrManagerNotStarted = errors.New("Health manager is not started")
)

package utils

import "errors"

var (
	// ErrDeadCollector defines not initialized or dead collector error
	ErrDeadCollector = errors.New("collector is not running")

	// ErrWrongMeasureType created for TargetMeasurements to mark that the object contains wrong type
	ErrWrongMeasureType = errors.New("measure is not the type you want to call")

	// ErrTargetNotRegistered defines unknown monitoring target ID error
	ErrTargetNotRegistered = errors.New("target is not registered")
	// ErrMetricNotRegistered defines unknown metric ID error
	ErrMetricNotRegistered = errors.New("metric is not registered on target")
	// ErrCounterNotRegistered defines unknown counter ID error
	ErrCounterNotRegistered = errors.New("counter is not registered on target")
	// ErrMeasureIDTaken defines measure ID already exists error
	ErrMeasureIDTaken = errors.New("measure with such ID already exist")
	// ErrManagerNotStarted defines not initialized manager error
	ErrManagerNotStarted = errors.New("health manager is not started")
)

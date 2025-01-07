package utils

import "errors"

var (
	// ErrDeadCollector defines not initialized or dead collector error
	ErrDeadCollector = errors.New("collector is not running")

	// ErrWrongMeasureType created for ComponentMeasurements to mark that the object contains wrong type
	ErrWrongMeasureType = errors.New("measure is not the type you want to call")

	// ErrComponentNotRegistered defines unknown monitoring component ID error
	ErrComponentNotRegistered = errors.New("component is not registered")
	// ErrMetricNotRegistered defines unknown metric ID error
	ErrMetricNotRegistered = errors.New("metric is not registered on component")
	// ErrCounterNotRegistered defines unknown counter ID error
	ErrCounterNotRegistered = errors.New("counter is not registered on component")
	// ErrMeasureIDTaken defines measure ID already exists error
	ErrMeasureIDTaken = errors.New("measure with such ID already exist")
	// ErrManagerNotStarted defines not initialized manager error
	ErrManagerNotStarted = errors.New("health manager is not started")
)

package utils

// List of constant keys and values for health framework
const (
	SourceTypeKey     = "source-type"
	DefaultSourceType = "zenoss.collection.health"

	TargetKey     = "target"
	TargetTypeKey = "target-type"
	SystemKey     = "system"
	SystemTypeKey = "system-type"

	DefaultTargetType = "default"

	HealthyStatus   = "Healthy"
	DegradeStatus   = "Degrade"
	UnhealthyStatus = "Unhealthy"
)

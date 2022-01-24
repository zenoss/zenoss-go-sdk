package utils

// List of constant keys and values for health framework
const (
	DefaultSourceType = "zenoss.collection.health"
	DefaultTargetType = "default"

	TargetKey     = "target"
	TargetTypeKey = "target-type"
	SourceKey     = "source"
	SourceTypeKey = "source-type"

	ZenossNameField = "name"

	HealthyStatus   = "Healthy"
	DegradeStatus   = "Degrade"
	UnhealthyStatus = "Unhealthy"
)

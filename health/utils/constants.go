package utils

// List of constant keys and values for health framework
const (
	DefaultSourceType    = "zenoss.collection.health"
	DefaultComponentType = "default"
	DefaultHealthTarget  = "general"

	HeartBeatMetricName = "health.heartbeat"

	ComponentKey     = "component"
	ComponentTypeKey = "component-type"
	TargetKey        = "target"
	SourceKey        = "source"
	SourceTypeKey    = "source-type"

	ZenossNameField = "name"

	HealthyStatus   = "Healthy"
	DegradeStatus   = "Degrade"
	UnhealthyStatus = "Unhealthy"

	ComponentsLimit        = 1000
	ComponentMeasuresLimit = 100
)

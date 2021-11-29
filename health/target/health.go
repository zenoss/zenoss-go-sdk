package target

import "github.com/zenoss/zenoss-go-sdk/health/utils"

// NewHealth initializes a new Health object with target ID
func NewHealth(id, tType string) *Health {
	if tType == "" {
		tType = utils.DefaultTargetType
	}
	return &Health{
		TargetID:   id,
		TargetType: tType,

		Status:    Healthy,
		Heartbeat: &HeartBeat{},
		Counters:  make(map[string]int32),
		Metrics:   make(map[string]float64),
		Messages:  make([]*Message, 0),
	}
}

// HealthStatus defines component health state
type HealthStatus int

const (
	// Healthy is the default status
	Healthy HealthStatus = iota
	// Degrade describes a scenario where the component is impaired but not out
	Degrade
	// Unhealthy is out of order component status
	Unhealthy
)

func (hs HealthStatus) String() string {
	return [...]string{utils.HealthyStatus, utils.DegradeStatus, utils.UnhealthyStatus}[hs]
}

// Health is a ready to send object that keeps all calculated health data during last cycle
type Health struct {
	TargetID   string
	TargetType string

	Status    HealthStatus
	Heartbeat *HeartBeat
	Counters  map[string]int32
	Metrics   map[string]float64
	Messages  []*Message
}

// HeartBeat is a wrapper that keeps whether heartbeat was enabled for current target and
// whether we received a heartbeat data during last cycle
type HeartBeat struct {
	Enabled, Beats bool
}

package target

// NewHealth initializes a new Health object with target ID
func NewHealth(id string) *Health {
	return &Health{
		ID: id,

		Heartbeat: &HeartBeat{},
		Counters:  make(map[string]int32),
		Metrics:   make(map[string]float64),
		Messages:  make([]*Message, 0),
	}
}

// Health is a ready to send object that keeps all calculated health data during last cycle
type Health struct {
	ID        string
	Healthy   bool
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

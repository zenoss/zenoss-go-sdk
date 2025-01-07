package component

// Message keeps the data that is related to component health.
// The message will mark the component as unhealthy if AffectHealth is true.
// You can send any info using it that can help you to keep your component healthy
type Message struct {
	Summary string
	Error   error

	AffectHealth bool
	HealthStatus HealthStatus
}

// NewMessage is just a constructor for component.Message
func NewMessage(summary string, err error, affectHealth bool, healthStatus HealthStatus) *Message {
	return &Message{Summary: summary, Error: err, AffectHealth: affectHealth, HealthStatus: healthStatus}
}

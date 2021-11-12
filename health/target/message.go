package target

// Message keeps the data that is related to target health.
// The message will mark the target as unhealthy if AffectHealth is true.
// You can send any info using it that can help you to keep your target healthy
type Message struct {
	Summary string
	Error   error

	AffectHealth bool
	HealthStatus HealthStatus
}

// NewMessage is just a constructor for target.Message
func NewMessage(summary string, err error, affectHealth bool, healthStatus HealthStatus) *Message {
	return &Message{Summary: summary, Error: err, AffectHealth: affectHealth, HealthStatus: healthStatus}
}

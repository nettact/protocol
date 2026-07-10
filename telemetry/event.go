package telemetry

import "time"

// Event is a discrete network occurrence (architecture §8.3). ID is
// agent-generated (a UUID); the server dedups on (AgentID, ID).
type Event struct {
	ID       string            `json:"id"`
	TS       time.Time         `json:"ts"`
	Type     EventType         `json:"type"`
	Layer    HealthLayer       `json:"layer,omitempty"`
	Severity Severity          `json:"severity"`
	Message  string            `json:"message,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

type EventType string

const (
	EventIfaceDown          EventType = "iface.down"
	EventIfaceUp            EventType = "iface.up"
	EventGatewayUnreachable EventType = "gateway.unreachable"
	EventWANIPChanged       EventType = "wan_ip.changed"
	EventDNSChanged         EventType = "dns.changed"
	EventDeviceNew          EventType = "device.new"
	EventRouteChanged       EventType = "route.changed"
	EventProbeFailed        EventType = "probe.failed"
	EventDataGap            EventType = "data.gap" // WAL overflow dropped samples
	EventAgentUpdated       EventType = "agent.updated"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarn     Severity = "warn"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

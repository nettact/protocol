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

	// EventProbeOverload reports that the agent could not run probes it was due to
	// run: its host-wide probe-concurrency budget (max_probe_concurrency) had no
	// slot free inside the probe's own timing budget, so the probe was skipped
	// rather than run late with a truncated measurement.
	//
	// It is a HOST-level event, not a per-monitor one, because the budget it
	// exhausted is the machine's — the same pool every monitor and every server
	// draws from — and it is rate-limited into one event per aggregation window
	// carrying the counts (see ProbeOverload*Label).
	//
	// Its job is to explain, not to detect. A probe that never ran produces no
	// sample, so the monitor goes stale on its own and the console shows it; what
	// the console cannot say without this event is that the cause was the agent
	// running out of probe budget rather than the network going away. Raising
	// max_probe_concurrency (or probing fewer targets) is the fix it points at.
	EventProbeOverload EventType = "probe.overload"

	// Game sensor lifecycle. The permission report already says whether game
	// metrics are possible at all; these events carry the part it cannot; why
	// they are not. "Blocked" in particular is what separates "the component is
	// not installed" (nothing to report) from "it is installed but was refused a
	// trace session" (actionable), so the reason attribute is a stable code.
	EventGameSensorBlocked   EventType = "game.sensor.blocked"
	EventGameSensorFailed    EventType = "game.sensor.failed"
	EventGameSensorRecovered EventType = "game.sensor.recovered"
)

// Attrs keys on an EventProbeOverload. They are a wire contract: the console
// renders the counts, so a producer that cannot fill one omits it rather than
// guessing.
const (
	// ProbeOverloadAbandonedLabel is how many probe operations the budget refused
	// during the window (decimal integer). An operation is one ICMP echo or one
	// single-shot probe (DNS/HTTP/TCP/NAT) that was due but never ran.
	ProbeOverloadAbandonedLabel = "abandoned"
	// ProbeOverloadWindowLabel is the aggregation window in seconds (decimal
	// integer) that the abandoned count covers.
	ProbeOverloadWindowLabel = "window_s"
	// ProbeOverloadLimitLabel is the configured max_probe_concurrency the rounds
	// were competing for, so the console can name the knob to raise.
	ProbeOverloadLimitLabel = "limit"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarn     Severity = "warn"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

package telemetry

import "time"

// Metric is a single-value time-series sample. One probe run may emit several
// metrics sharing a timestamp and target (e.g. rtt + loss for one ping),
// which is the best fit for a narrow time-series table.
type Metric struct {
	TS     time.Time         `json:"ts"`
	Kind   MetricKind        `json:"kind"`
	Target string            `json:"target,omitempty"` // "gateway", "1.1.1.1", "https://…", "eth0"
	Layer  HealthLayer       `json:"layer,omitempty"`
	Value  float64           `json:"value"`
	Unit   string            `json:"unit,omitempty"` // "ms", "pct", "bool"
	Labels map[string]string `json:"labels,omitempty"`
}

// MetricKind is a string enum so unknown kinds are ignored rather than failing
// to decode on older servers.
type MetricKind string

const (
	ICMPRTTms  MetricKind = "probe.icmp.rtt_ms"
	ICMPLoss   MetricKind = "probe.icmp.loss_pct"
	ICMPJitter MetricKind = "probe.icmp.jitter_ms"
	DNSResolve MetricKind = "probe.dns.resolve_ms"
	DNSOK      MetricKind = "probe.dns.ok"
	HTTPStatus MetricKind = "probe.http.status"
	HTTPLat    MetricKind = "probe.http.latency_ms"
	HTTPOK     MetricKind = "probe.http.ok"
	IfaceUp    MetricKind = "iface.up"

	// Agent self-status (heartbeat), so status is reported outbound without the
	// agent exposing any endpoint.
	AgentUptime     MetricKind = "agent.uptime_s"
	AgentWALPending MetricKind = "agent.wal_pending"
)

// Units for Metric.Value.
const (
	UnitMs    = "ms"
	UnitPct   = "pct"
	UnitBool  = "bool"
	UnitCode  = "code"
	UnitCount = "count"
	UnitSec   = "s"
)

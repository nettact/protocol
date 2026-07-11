// Package config defines the DesiredState the server pushes down to an agent
// via the telemetry ack. Monitoring targets are configured centrally in Lite
// and delivered to agents on the agent's own outbound request — the agent
// never listens, and users don't edit agent config files (low friction).
package config

// DesiredState is the monitoring configuration for one agent. The agent applies
// it and echoes ConfigVersion back in reported_config_version so the server can
// tell when it is up to date.
type DesiredState struct {
	ConfigVersion int           `json:"config_version"`
	ProbeTargets  []ProbeTarget `json:"probe_targets"`
	Intervals     Intervals     `json:"intervals"`

	// SnapshotRequest, when non-nil, asks the agent to return an ephemeral
	// telemetry.HostSnapshot (live process/connection list) in its next upload.
	// This is how a console user's live-page request reaches the outbound-only
	// agent. The agent honors it only for caps it was started with; a request
	// for a disabled cap is dropped before any collection happens.
	SnapshotRequest *SnapshotRequest `json:"snapshot_request,omitempty"`
}

// SnapshotRequest is a one-shot ask for a live host snapshot. Not versioned into
// ConfigVersion — it is transient and cleared once the matching snapshot arrives.
type SnapshotRequest struct {
	RequestID       string `json:"request_id"`
	WantProcesses   bool   `json:"want_processes,omitempty"`
	WantConnections bool   `json:"want_connections,omitempty"`
}

// ProbeTarget is one monitoring target pushed to the agent.
type ProbeTarget struct {
	Kind   string      `json:"kind"`   // "icmp" (M2); "dns" / "http" added in M3
	Target string      `json:"target"` // "1.1.1.1", "example.com", "https://…"
	Tier   string      `json:"tier"`   // "base" | "regular"
	Params ProbeParams `json:"params"` // per-protocol probe settings (zero = collector defaults)
}

// ProbeParams carries per-target, per-protocol probe settings. Zero values mean
// "use the collector default", so an unconfigured target behaves as before.
type ProbeParams struct {
	// Common — applies to every protocol.
	IntervalSeconds int `json:"interval_seconds,omitempty"` // per-target check interval; 0 = fall back to tier default
	TimeoutMs       int `json:"timeout_ms,omitempty"`       // per-probe timeout

	// ICMP.
	PacketSize int `json:"packet_size,omitempty"` // ICMP echo payload bytes
	Retries    int `json:"retries,omitempty"`     // extra echoes per cycle beyond the first (count = retries+1)

	// DNS.
	RecordType string `json:"record_type,omitempty"` // A | AAAA | … (default A)

	// HTTP.
	Method         string `json:"method,omitempty"`          // GET | HEAD | … (default GET)
	ExpectedStatus int    `json:"expected_status,omitempty"` // 0 = any 2xx counts as ok
}

// Intervals controls the agent scheduler tiers (seconds).
type Intervals struct {
	BaseSeconds    int `json:"base_seconds"`
	RegularSeconds int `json:"regular_seconds"`
}

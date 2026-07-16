// Package config defines the DesiredState the server pushes down to an agent
// over the persistent WebSocket channel. Monitoring targets are configured
// centrally in Lite and pushed on connect and on every config change — the
// agent never listens, and users don't edit agent config files (low friction).
package config

import "time"

// DesiredState is the monitoring configuration for one agent. The agent applies
// it and echoes ConfigVersion back in reported_config_version so the server can
// tell when it is up to date.
type DesiredState struct {
	ConfigVersion int           `json:"config_version"`
	ProbeTargets  []ProbeTarget `json:"probe_targets"`
	Intervals     Intervals     `json:"intervals"`
}

// SnapshotRequest is a one-shot ask for a live host snapshot, pushed to the
// agent as a standalone frame. Not versioned into ConfigVersion — it is
// transient and cleared once the matching snapshot arrives. Scopes are the
// requested process/connection permission IDs (e.g. host.process.basic.read);
// the agent evaluates each against its effective policy and always answers,
// reporting collected/denied/unsupported/failed per scope.
type SnapshotRequest struct {
	RequestID string   `json:"request_id"`
	Scopes    []string `json:"scopes,omitempty"`
}

// IncidentSnapshotRequest is a one-shot server->agent ask for an immutable
// incident scene snapshot (INCIDENT-002), pushed as a standalone frame. It is
// transient — not versioned into ConfigVersion — and answered once with a
// telemetry.IncidentSnapshot carrying the same RequestID and IncidentID. The
// agent collects only the allowlisted evidence groups and stops at Deadline,
// reporting per-group collected/denied/unsupported/failed either way.
type IncidentSnapshotRequest struct {
	RequestID  string              `json:"request_id"`        // stable snapshot request id (idempotency key with IncidentID + agent)
	IncidentID string              `json:"incident_id"`       // the incident this snapshot belongs to
	Deadline   time.Time           `json:"deadline"`          // absolute collection deadline; past it the agent stops and reports terminal group states
	Targets    []SnapshotTargetRef `json:"targets,omitempty"` // monitor targets to resolve endpoints/error class for
}

// SnapshotTargetRef identifies one monitor target the incident snapshot should
// resolve. It carries enough to key the result by monitor id, choose the probe
// semantics (Kind), and reconstruct the endpoint (Target + Port).
type SnapshotTargetRef struct {
	MonitorID string `json:"monitor_id"`     // stable server-side monitor id (probe_tasks.id)
	Kind      string `json:"kind"`           // "icmp" | "dns" | "http" | "tcp" | "nat" | "gateway"
	Target    string `json:"target"`         // literal/host/URL as configured
	Port      int    `json:"port,omitempty"` // TCP/UDP port when the kind carries one
}

// Traceroute modes (TraceRequest.Mode / telemetry.TraceResult.Mode). ICMP and
// TCP are executed independently; there is no automatic fallback between them.
const (
	TraceModeICMP = "icmp"
	TraceModeTCP  = "tcp"
)

// TraceRequest is a one-shot server->agent ask to run a single incident
// traceroute (DIAG-001), pushed as a standalone frame. The detecting agent runs
// exactly one trace per request and answers once with a telemetry.TraceResult
// carrying the same ReportID. Deadline is the only validity window: past it the
// agent must not start, and a running trace is bounded by TotalTimeoutMs. The
// mode is fixed by the request — the agent never falls back to the other mode.
type TraceRequest struct {
	ReportID            string    `json:"report_id"`             // stable shared report/request id all referencing incidents read through
	Mode                string    `json:"mode"`                  // TraceModeICMP | TraceModeTCP
	DestinationHost     string    `json:"destination_host"`      // host or IP to trace toward
	TCPPort             int       `json:"tcp_port,omitempty"`    // required for Mode == TraceModeTCP
	MaxHops             int       `json:"max_hops"`              // TTL ceiling
	AttemptsPerHop      int       `json:"attempts_per_hop"`      // probes sent per TTL
	TotalTimeoutMs      int       `json:"total_timeout_ms"`      // overall wall-clock budget for the whole trace
	ResolveHopHostnames bool      `json:"resolve_hop_hostnames"` // reverse-DNS each responder (default off)
	Deadline            time.Time `json:"deadline"`              // absolute request validity window
}

// ProbeTarget is one monitoring target pushed to the agent.
type ProbeTarget struct {
	// MonitorID is the stable server-side id of this monitor (probe_tasks.id).
	// The agent stamps it onto every Metric the probe emits so the server keys
	// series per monitor — two monitors on the same target string stay distinct.
	MonitorID string      `json:"monitor_id,omitempty"`
	Kind      string      `json:"kind"`           // "icmp" | "dns" | "http" | "tcp" | "nat" | "gateway" (host is server-side only)
	Name      string      `json:"name,omitempty"` // human-friendly display name; optional
	Target    string      `json:"target"`         // "1.1.1.1", "example.com", "https://…"
	Params    ProbeParams `json:"params"`         // per-protocol probe settings (zero = collector defaults)
}

// ProbeParams carries per-target, per-protocol probe settings. Zero values mean
// "use the collector default", so an unconfigured target behaves as before.
type ProbeParams struct {
	// Common — applies to every protocol.
	IntervalSeconds int `json:"interval_seconds,omitempty"` // per-target check interval; 0 = fall back to the collector default
	TimeoutMs       int `json:"timeout_ms,omitempty"`       // per-probe timeout

	// ICMP / Ping.
	PacketSize      int `json:"packet_size,omitempty"`       // ICMP echo payload bytes
	Retries         int `json:"retries,omitempty"`           // extra echoes per cycle beyond the first (count = retries+1); superseded by PacketCount
	PacketCount     int `json:"packet_count,omitempty"`      // total echoes per cycle; 0 = fall back to Retries+1
	GlobalTimeoutMs int `json:"global_timeout_ms,omitempty"` // overall deadline across all echoes in one cycle

	// DNS.
	RecordType       string `json:"record_type,omitempty"`       // A | AAAA | CNAME | MX | TXT | NS (default A/AAAA)
	ResolverServer   string `json:"resolver_server,omitempty"`   // resolver IP/host override, or DoH URL (default: system resolver)
	ResolverPort     int    `json:"resolver_port,omitempty"`     // resolver port (default 53, or 853 for DoT)
	ResolverProtocol string `json:"resolver_protocol,omitempty"` // "" | udp | tcp | dot | doh (default plain UDP/system)

	// HTTP.
	Method           string            `json:"method,omitempty"`             // GET | HEAD | POST | … (default GET)
	ExpectedStatus   int               `json:"expected_status,omitempty"`    // legacy single status; 0 = any 2xx (kept for back-compat)
	AcceptedStatuses string            `json:"accepted_statuses,omitempty"`  // ranges/CSV e.g. "200-299,301"; overrides ExpectedStatus when set
	Keyword          string            `json:"keyword,omitempty"`            // body keyword; ok requires presence (or absence when KeywordInvert)
	KeywordInvert    bool              `json:"keyword_invert,omitempty"`     // invert keyword match (fail when keyword present)
	Headers          map[string]string `json:"headers,omitempty"`            // request headers
	Body             string            `json:"body,omitempty"`               // request body (sent for non-GET/HEAD)
	MaxRedirects     int               `json:"max_redirects,omitempty"`      // max redirects to follow; <0 disables following
	IgnoreTLS        bool              `json:"ignore_tls,omitempty"`         // skip TLS certificate verification
	MaxResponseBytes int               `json:"max_response_bytes,omitempty"` // cap on body bytes read for keyword match (default 1 KiB)

	// TCP.
	Port int  `json:"port,omitempty"` // TCP port to connect to (also the STUN port for kind=nat; default 3478)
	TLS  bool `json:"tls,omitempty"`  // perform a TLS handshake after connect

	// NAT (STUN behavior discovery, RFC 5780 / RFC 4787).
	NATTransport string `json:"nat_transport,omitempty"` // "" | udp | tcp | tls | dtls (default udp); only udp does the filtering test + classic type
	STUNServer2  string `json:"stun_server2,omitempty"`  // optional 2nd STUN server host[:port] used as the mapping-test alternate when the primary lacks OTHER-ADDRESS

	// Gateway.
	Interface string `json:"interface,omitempty"` // NIC to resolve the gateway from, matched against IfaceInfo.ID or Name; "" = default interface
}

// Intervals controls the agent scheduler tiers (seconds).
type Intervals struct {
	BaseSeconds    int `json:"base_seconds"`
	RegularSeconds int `json:"regular_seconds"`
}

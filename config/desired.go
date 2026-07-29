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
	// Proxies are the egress proxies referenced by the targets above — only those
	// actually referenced, and only those still enabled. A target whose ProxyID has
	// no entry here is deliberately left in ProbeTargets so the agent reports it as
	// a proxy-missing operational issue: dropping it server-side would make a
	// disabled proxy look like a deleted monitor.
	Proxies []ProxySpec `json:"proxies,omitempty"`
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
// agent collects only the allowlisted evidence groups and stops when BudgetMs
// runs out, reporting per-group collected/denied/unsupported/failed either way.
type IncidentSnapshotRequest struct {
	RequestID  string              `json:"request_id"`        // stable snapshot request id (idempotency key with IncidentID + agent)
	IncidentID string              `json:"incident_id"`       // the incident this snapshot belongs to
	BudgetMs   int                 `json:"budget_ms"`         // collection budget measured from arrival on the agent's own clock (see BudgetWindow)
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
// carrying the same ReportID. BudgetMs is the only validity window: exhausted,
// the agent must not start, and a running trace is bounded by TotalTimeoutMs.
// The mode is fixed by the request — the agent never falls back to the other mode.
type TraceRequest struct {
	ReportID            string `json:"report_id"`             // stable shared report/request id all referencing incidents read through
	Mode                string `json:"mode"`                  // TraceModeICMP | TraceModeTCP
	DestinationHost     string `json:"destination_host"`      // host or IP to trace toward
	TCPPort             int    `json:"tcp_port,omitempty"`    // required for Mode == TraceModeTCP
	MaxHops             int    `json:"max_hops"`              // TTL ceiling
	AttemptsPerHop      int    `json:"attempts_per_hop"`      // probes sent per TTL
	TotalTimeoutMs      int    `json:"total_timeout_ms"`      // overall wall-clock budget for the whole trace
	ResolveHopHostnames bool   `json:"resolve_hop_hostnames"` // reverse-DNS each responder (default off)
	BudgetMs            int    `json:"budget_ms"`             // request validity window measured from arrival on the agent's own clock (see BudgetWindow)
}

// BudgetWindow converts a request's receipt-relative budget in milliseconds into
// an absolute deadline on the receiving agent's own clock, anchored at the
// request's arrival instant receivedAt and evaluated as of now.
//
// One-shot server->agent requests carry a duration, never an absolute timestamp:
// the two clocks are independent and can be minutes apart, and a timestamp minted
// on the server clock and consumed on the agent clock has the whole skew
// subtracted from (or added to) the window. A skew larger than the budget makes
// the request expire the instant it arrives, so every collection reports a
// timeout that never happened. A duration is skew-immune — it costs only the push
// latency, which the server absorbs by keeping its own reaping deadline.
//
// Anchoring at arrival rather than at now is what keeps handler scheduling delay
// from being handed back as extra window, so the two instants are separate
// arguments and both are needed: ok is false for a non-positive budget AND for a
// window that already elapsed between arrival and now. Either way the window is
// spent, so the receiver must not start and reports its terminal timed-out state.
func BudgetWindow(budgetMs int, receivedAt, now time.Time) (time.Time, bool) {
	if budgetMs <= 0 {
		return time.Time{}, false
	}
	deadline := receivedAt.Add(time.Duration(budgetMs) * time.Millisecond)
	if !now.Before(deadline) {
		return time.Time{}, false
	}
	return deadline, true
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
	// ProxyID pins this target's egress to one DesiredState.Proxies entry. Empty
	// means a direct dial. A non-empty id the agent cannot honor (absent spec, or a
	// type this kind cannot use) makes the monitor un-runnable rather than direct:
	// there is no fallback, by design.
	ProxyID string `json:"proxy_id,omitempty"`
	// ConfigSerial is this target's material config generation
	// (probe_tasks.config_serial) at push time. The agent echoes it on every
	// Metric (Metric.ConfigSerial) and MonitorStatusEntry (TargetConfigSerial)
	// the target produces, so the server can reject obsolete-generation samples
	// and distinguish a current-generation status report from a stale one.
	ConfigSerial int `json:"config_serial,omitempty"`
}

// ProbeParams carries per-target, per-protocol probe settings. Zero values mean
// "use the collector default", so an unconfigured target behaves as before.
type ProbeParams struct {
	// Common — applies to every protocol.
	IntervalSeconds int `json:"interval_seconds,omitempty"` // per-target check interval; 0 = fall back to the collector default
	TimeoutMs       int `json:"timeout_ms,omitempty"`       // per-probe timeout

	// ICMP / Ping.
	PacketSize      int `json:"packet_size,omitempty"`       // ICMP echo payload bytes
	PacketCount     int `json:"packet_count,omitempty"`      // total echoes per cycle; 0 = collector default
	GlobalTimeoutMs int `json:"global_timeout_ms,omitempty"` // overall deadline across all echoes in one cycle

	// DNS.
	RecordType       string `json:"record_type,omitempty"`       // A | AAAA | CNAME | MX | TXT | NS (default A/AAAA)
	ResolverServer   string `json:"resolver_server,omitempty"`   // resolver IP/host override, or DoH URL (default: system resolver)
	ResolverPort     int    `json:"resolver_port,omitempty"`     // resolver port (default 53, or 853 for DoT)
	ResolverProtocol string `json:"resolver_protocol,omitempty"` // "" | udp | tcp | dot | doh (default plain UDP/system)

	// HTTP.
	Method           string            `json:"method,omitempty"`             // GET | HEAD | POST | … (default GET)
	AcceptedStatuses string            `json:"accepted_statuses,omitempty"`  // ranges/CSV e.g. "200-299,301"; empty = any 2xx/3xx
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

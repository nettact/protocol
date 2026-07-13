// Package config defines the DesiredState the server pushes down to an agent
// over the persistent WebSocket channel. Monitoring targets are configured
// centrally in Lite and pushed on connect and on every config change — the
// agent never listens, and users don't edit agent config files (low friction).
package config

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
// transient and cleared once the matching snapshot arrives. The agent honors
// it only for caps it was started with; a request for a disabled cap is
// dropped before any collection happens.
type SnapshotRequest struct {
	RequestID       string `json:"request_id"`
	WantProcesses   bool   `json:"want_processes,omitempty"`
	WantConnections bool   `json:"want_connections,omitempty"`
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

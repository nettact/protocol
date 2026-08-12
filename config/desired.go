// Package config defines the DesiredState the server pushes down to an agent
// over the persistent WebSocket channel. Monitoring targets are configured
// centrally in Lite and pushed on connect and on every config change — the
// agent never listens, and users don't edit agent config files (low friction).
package config

// DesiredState is the monitoring configuration for one agent. The agent applies
// it and echoes ConfigVersion back in every MonitorStatus frame, which is how
// the server tells when the agent is up to date.
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
	// Game is the game-profile configuration for the site. Nil means the server
	// has nothing to say about game capture; an empty-but-present block is a
	// deliberate "record everything, no profiles defined".
	Game *GameConfig `json:"game,omitempty"`
	// Diag is the path-diagnostic policy the agent applies to its own traceroute
	// trigger. Nil means this server has nothing to say and the agent's built-in
	// defaults stand — which is also what an agent reporting to several servers
	// needs, since each server states its policy independently.
	Diag *DiagPolicy `json:"diag,omitempty"`
}

// DiagPolicy says when an agent may traceroute a target it has just decided is
// down, and how far the sweep may go. The server no longer commands the
// execution — the agent notices the failure and acts, because the moment worth
// tracing is precisely the moment the link to the server is least trustworthy —
// but it remains the policy source, so the numbers still come from here.
//
// # Why it is global and not per target
//
// It answers "how eager is this install about path diagnostics", which is an
// install-level question. A per-target copy would have to be re-pushed on every
// unrelated monitor edit to keep saying the same thing, and would invite a
// configuration where two monitors on one destination disagree about whether it
// may be traced.
//
// ConsecutiveFailures deliberately matches the server's own availability
// confirmation threshold: the agent's streak is counting the same rounds the
// server would count, so a trace fires as the fault is confirmed rather than
// before it (noise) or long after (evidence collected past the interesting
// moment). A zero field means "use the agent's built-in default", never zero.
type DiagPolicy struct {
	Enabled             bool `json:"enabled"`
	ConsecutiveFailures int  `json:"consecutive_failures,omitempty"`
	// Serial orders policy updates, mirroring what ConfigVersion does for the
	// probe half and GameConfig.Version for the game half. The server bumps it
	// on every diag_* settings change; the agent applies a policy only when the
	// serial is newer than what it holds. It is its own axis because the other
	// two serials do not move when only the diagnostic numbers change, and
	// DesiredState builds can be delivered out of build order — unversioned,
	// a stale enabled=true arriving last would keep tracing after the operator
	// turned diagnostics off.
	Serial uint64 `json:"serial,omitempty"`
	// CooldownSeconds is the minimum spacing between two traces of the SAME
	// destination on this agent. Without it a target that stays down would be
	// traced on every subsequent failing round, spending the machine's probe
	// budget re-measuring a path whose answer has not changed.
	CooldownSeconds int `json:"cooldown_seconds,omitempty"`
	MaxHops         int `json:"max_hops,omitempty"`
	Attempts        int `json:"attempts,omitempty"`
	// PerHopTimeoutMs bounds one probe attempt. Zero lets the agent derive it
	// from BudgetMs and MaxHops, which is the sane relationship; setting it is
	// for installs whose links need a longer or shorter wait than that implies.
	PerHopTimeoutMs int `json:"per_hop_timeout_ms,omitempty"`
	// BudgetMs is the whole sweep's wall-clock ceiling.
	BudgetMs int `json:"budget_ms,omitempty"`
}

// GameConfig is the site's game-capture configuration: which processes count as
// which game, and whether everything else is worth recording at all.
//
// Version is its own axis, deliberately not ConfigVersion. The two describe
// unrelated things that change at unrelated times: renaming a game profile has
// nothing to say to a ping monitor, and bumping ConfigVersion for it would make
// every agent re-evaluate every probe target and restart the ones whose serial
// it cannot prove unchanged. Keeping the serials apart means a profile edit
// re-pushes DesiredState with an unchanged ConfigVersion — the probe side
// no-ops — and a probe edit re-pushes it with an unchanged game Version, so the
// sensor is not restarted for a change it cannot see (product doc §13).
type GameConfig struct {
	Version int `json:"version"` // sites.game_config_serial — independent of ConfigVersion
	// RecordUnmatched decides what happens to a presenting process that matches
	// no profile: recorded as an "other process" run, or ignored. It is a site
	// setting rather than a per-profile one because it is a privacy choice about
	// the machine, not a measurement choice about a game.
	RecordUnmatched bool          `json:"record_unmatched"`
	Profiles        []GameProfile `json:"profiles,omitempty"`
}

// GameProfile is one named game as the agent needs it.
//
// The profile's monitor_ids are deliberately absent: linking a game to network
// monitors only affects how a run is charted in the console, and the agent would
// carry the list across every push without ever reading it. Anything the agent
// cannot act on stays server-side.
type GameProfile struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Exe       []string `json:"exe"`                  // case-insensitive process names
	TargetFPS int      `json:"target_fps,omitempty"` // 0 = unset
	Tier      string   `json:"tier"`                 // "base" | "diag"
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

// Traceroute modes (telemetry.TraceResult.Mode). ICMP and TCP are executed
// independently; there is no automatic fallback between them at execution time.
// The mode is no longer part of a pushed request — the agent picks it from the
// probe kind that failed — but the vocabulary stays here because it is shared
// with the stored report and the console.
const (
	TraceModeICMP = "icmp"
	TraceModeTCP  = "tcp"
)

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
	// SizeSweep probes each cycle at several ICMP echo payload sizes instead of
	// one fixed PacketSize, so the server can separate size-correlated loss
	// (physical-layer degradation: optics / CRC / FEC / ASIC / policer) from
	// congestion, where loss is flat across sizes. PayloadSizes lists the swept
	// sizes; empty means the built-in default sweep (see config.SweepSizes). When
	// enabled the cycle sends PingCount(params) echoes PER size, round-robin
	// across sizes, so the total (count × len(sizes)) flows through PingCount —
	// the same single source the server's freshness and round-completeness checks
	// read, so the longer cycle is never mistaken for a stale or truncated one.
	SizeSweep    bool  `json:"size_sweep,omitempty"`
	PayloadSizes []int `json:"payload_sizes,omitempty"` // swept ICMP payload bytes; empty = default sweep

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
	// FlowFanout probes the target with FlowFanout distinct pinned source ports
	// each cycle instead of one ephemeral-port connection, so a deterministic bad
	// subset of ECMP/LAG-hashed flows — the same five-tuple hits the same member
	// every cycle, hence reproducible — is distinguishable from uniform loss.
	// 0/1 means the single-flow behavior. Direct-dial targets only: a proxied
	// target's local endpoint belongs to the proxy tunnel and cannot be pinned.
	FlowFanout int `json:"flow_fanout,omitempty"` // TCP source-port fan-out count; 0 = off

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

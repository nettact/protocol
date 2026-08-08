package telemetry

import "time"

// IncidentSnapshot is the agent's allowlisted incident-scene evidence for one
// config.IncidentSnapshotRequest (INCIDENT-002). It is collected once, on
// demand, and answered back over the WebSocket channel; the server keys it by
// (RequestID + IncidentID + agent) and treats it as immutable once terminal.
//
// Collection is allowlisted by construction: the typed field groups below carry
// only network context, agent identity, a basic resource summary, and target
// resolution — never process lists, user names, file paths, credentials,
// request/response headers, or bodies. Each requested group reports its own
// collected/denied/unsupported/failed outcome in Groups so a partial snapshot
// completes immediately instead of timing out; a group's typed payload is set
// only when that group's Status is collected.
type IncidentSnapshot struct {
	RequestID   string    `json:"request_id"`   // echoes config.IncidentSnapshotRequest.RequestID
	IncidentID  string    `json:"incident_id"`  // echoes config.IncidentSnapshotRequest.IncidentID
	CollectedAt time.Time `json:"collected_at"` // agent clock when collection finished (server compares against its receipt time for clock-skew)

	Groups []SnapshotGroupResult `json:"groups"` // one result per attempted field group, always present

	Network   *SnapshotNetwork       `json:"network,omitempty"`   // set iff the network group collected
	Agent     *SnapshotAgentInfo     `json:"agent,omitempty"`     // set iff the agent-info group collected
	Resources *SnapshotResources     `json:"resources,omitempty"` // set iff the resources group collected
	Targets   []SnapshotTargetResult `json:"targets,omitempty"`   // set iff the target-resolution group collected
}

// Incident snapshot field-group ids (SnapshotGroupResult.Group). Each group is
// gathered and reported independently.
const (
	SnapshotGroupNetwork   = "network"   // interfaces/addresses, default route, DNS servers
	SnapshotGroupAgent     = "agent"     // agent identity/version
	SnapshotGroupResources = "resources" // basic CPU/memory summary
	SnapshotGroupTargets   = "targets"   // per-target resolution/endpoints/error class
)

// SnapshotGroupResult is the outcome of one allowlisted field group. Status
// reuses the snapshot scope status vocabulary (ScopeCollected/ScopeDenied/
// ScopeUnsupported/ScopeFailed); Reason is a stable code on a non-collected
// group; CollectedAt is the agent clock when the group was gathered.
type SnapshotGroupResult struct {
	Group       string    `json:"group"`                  // one of the SnapshotGroup* ids
	Status      string    `json:"status"`                 // collected | denied | unsupported | failed
	Reason      string    `json:"reason,omitempty"`       // stable reason code when not collected
	CollectedAt time.Time `json:"collected_at,omitempty"` // agent clock when this group was gathered
}

// SnapshotNetwork is the collected local network context. It intentionally
// excludes any per-connection or per-process detail.
type SnapshotNetwork struct {
	Interfaces   []SnapshotInterface `json:"interfaces,omitempty"`
	DefaultRoute *SnapshotRoute      `json:"default_route,omitempty"`
	DNSServers   []string            `json:"dns_servers,omitempty"`
}

// SnapshotInterface is one interface's up/wireless state and addresses.
type SnapshotInterface struct {
	Name       string   `json:"name"`
	Addrs      []string `json:"addrs,omitempty"`
	Up         bool     `json:"up"`
	IsWireless bool     `json:"is_wireless,omitempty"`
}

// SnapshotRoute is the host's default route: the next-hop gateway and the
// egress interface it resolves through.
type SnapshotRoute struct {
	Gateway   string `json:"gateway,omitempty"`
	Interface string `json:"interface,omitempty"`
}

// SnapshotAgentInfo is the detecting agent's own identity and version at
// collection time — fixed into the snapshot so later renames don't rewrite
// history.
type SnapshotAgentInfo struct {
	AgentID      string `json:"agent_id,omitempty"`
	Hostname     string `json:"hostname,omitempty"`
	Platform     string `json:"platform,omitempty"`
	AgentVersion string `json:"agent_version,omitempty"`
}

// SnapshotResources is a basic host CPU/memory summary. Pointers carry presence
// so a value the platform cannot read stays absent rather than reporting zero.
type SnapshotResources struct {
	CPUPercent       *float64 `json:"cpu_percent,omitempty"`        // instantaneous total CPU utilization, 0–100
	MemoryTotalBytes *uint64  `json:"memory_total_bytes,omitempty"` // physical memory installed
	MemoryUsedBytes  *uint64  `json:"memory_used_bytes,omitempty"`  // physical memory in use
}

// SnapshotTargetResult is one monitor target's resolution as seen from the
// detecting agent: the addresses it resolved to, the endpoints it would probe,
// and a coarse error class. It carries no request/response content.
type SnapshotTargetResult struct {
	MonitorID   string   `json:"monitor_id"`
	Kind        string   `json:"kind,omitempty"`
	Target      string   `json:"target,omitempty"`
	ResolvedIPs []string `json:"resolved_ips,omitempty"`
	Endpoints   []string `json:"endpoints,omitempty"`   // host:port endpoints derived for probing
	ErrorClass  string   `json:"error_class,omitempty"` // invalid_target | policy_denied | dns_error | timeout | canceled ("" when resolved)
}

// TraceResult is one traceroute an agent decided to run against a target it had
// just found unreachable (DIAG-001), reported once per ReportID. Intermediate
// `*` (a timed-out attempt) never implies a broken path — only a responder from
// the destination sets Reached.
//
// # Why it describes itself
//
// The server does not command traceroutes and therefore holds no plan to match
// an answer against: the agent notices the failure streak, derives the subject
// and destination from the target it was pushed, and runs the sweep whether or
// not anyone is listening. Everything the report has to be filed, deduplicated
// and rendered under therefore travels inside it — the canonical DestKey, the
// subject, the path scope, and the trigger that produced it. A field left out
// here is a fact no other party can supply afterwards.
//
// It rides the WAL inside a telemetry.Packet rather than the socket, for the
// blunt reason that the fault being diagnosed is the most likely cause of the
// link being down. At-least-once delivery plus (agent, sequence) dedup is what
// makes a trace collected during an outage survive to be read after it.
type TraceResult struct {
	// ReportID is minted by the agent (a UUID) and is the server's idempotency
	// key together with the authenticated agent id. A replayed packet carries the
	// same id and is a no-op.
	ReportID string `json:"report_id"`

	// ---- what was traced ----

	// DestKey is the canonical destination key — "ip:<canonical-ip>" for a
	// literal, "host:<lowercased-host>" otherwise. It is what the server matches
	// a fault against when it claims a report as evidence, so the agent computes
	// it rather than leaving the server to re-derive it from a display string.
	DestKey  string `json:"dest_key"`
	DestHost string `json:"dest_host"`
	Mode     string `json:"mode"` // config.TraceModeICMP | config.TraceModeTCP
	Port     int    `json:"port,omitempty"`

	DestinationIP string `json:"destination_ip,omitempty"` // resolved destination address the trace ran against
	Reached       bool   `json:"reached"`                  // true once the destination itself responded
	ReachedTTL    int    `json:"reached_ttl,omitempty"`    // TTL/hop number at which the destination replied

	// SubjectKind names WHAT the destination is (TraceSubject*), and
	// SubjectReason why that subject was chosen where the choice is not implied
	// by the probe kind. Without them a resolver trace and a target trace are
	// indistinguishable on read: same destination columns, opposite meanings.
	SubjectKind   string `json:"subject_kind"`
	SubjectReason string `json:"subject_reason,omitempty"`

	// PathScope says which path the probes travelled — TracePathDirect (host
	// stack; "" reads the same) or TracePathWireGuardInner (probes sent inside
	// the tunnel). The egress fields name the generation that carried them and
	// are empty for a host-stack trace. The agent fails closed rather than
	// silently falling back to the host stack, so these describe the execution
	// and not an intention.
	PathScope          string `json:"path_scope,omitempty"`
	EgressProxyID      string `json:"egress_proxy_id,omitempty"`
	EgressConfigSerial int    `json:"egress_config_serial,omitempty"`

	// FallbackFrom/FallbackReason record an automatic mode downgrade: a TCP plan
	// whose agent lacks the TCP traceroute permission but holds the ICMP one runs
	// as ICMP with FallbackFrom="tcp" and a stable why-code, so a fallback reads
	// as a fallback rather than as a failure.
	FallbackFrom   string `json:"fallback_from,omitempty"`
	FallbackReason string `json:"fallback_reason,omitempty"`

	// ---- why it ran ----

	// TriggerReason is the agent-local rule that fired (TraceTrigger*),
	// TriggerStreak how many consecutive failing rounds it took, and
	// FirstFailedAt when that streak began. Nothing server-side records the cause
	// of a trace any more, so an unexplained report would be exactly that.
	TriggerReason string    `json:"trigger_reason,omitempty"`
	TriggerStreak int       `json:"trigger_streak,omitempty"`
	FirstFailedAt time.Time `json:"first_failed_at,omitempty"`

	// MaxHops/AttemptsPerHop are the bounds the sweep really ran under, carried
	// so the server clamps the stored hop rows to what was asked for instead of
	// to a default it would otherwise have to guess.
	MaxHops        int `json:"max_hops,omitempty"`
	AttemptsPerHop int `json:"attempts_per_hop,omitempty"`

	// ---- outcome ----

	Status string `json:"status"`           // terminal TraceStatus* value
	Reason string `json:"reason,omitempty"` // stable reason code for unsupported/failed/canceled

	StartedAt   time.Time  `json:"started_at,omitempty"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	Hops        []TraceHop `json:"hops,omitempty"`
}

// Traceroute trigger rules (TraceResult.TriggerReason). One value today; it is a
// named open enum rather than a bare boolean because "the agent ran a trace" and
// "the agent ran a trace BECAUSE the target failed three rounds running" are
// different statements, and only the second one is worth showing an operator.
const (
	TraceTriggerConsecutiveFailures = "consecutive_failures"
)

// What a trace report diagnoses (TraceResult.SubjectKind). The monitored target
// is only one option: a probe reaches its target through a resolver, a proxy or
// a tunnel, and when the fault is on that leg, tracing the target measures a
// path the probe never used. Mirrored by the server's trace_reports.subject_kind
// CHECK constraint and by the console's subject labels.
const (
	TraceSubjectTarget     = "target"      // the monitored endpoint itself
	TraceSubjectResolver   = "resolver"    // the DNS server a dns monitor queried
	TraceSubjectProxy      = "proxy"       // the socks5/http proxy a pinned monitor dialed
	TraceSubjectWGEndpoint = "wg_endpoint" // a WireGuard peer's physical endpoint
	TraceSubjectSTUNServer = "stun_server" // the STUN server a nat monitor probed
)

// Which question a WireGuard fault's trace answers (TraceResult.SubjectReason).
// The verdict comes from the failing round's frozen reason code, not from the
// trace itself; together with PathScope it tells the operator whether the hops
// describe the fault's own path or the nearest evidence to it.
const (
	// TraceSubjectTunnelUnreachable: the probe never got through the tunnel (a
	// proxy_* reason), so the peer's physical reachability IS the fault.
	TraceSubjectTunnelUnreachable = "tunnel_unreachable"
	// TraceSubjectTunnelTargetUnreachable: the tunnel carried the probe and the
	// TARGET failed beyond it. The fault's own path is the in-tunnel one, so the
	// trace runs INSIDE the tunnel (subject target, path scope
	// wireguard_inner), pinned to the egress generation that carried the failing
	// probes. Only when the in-tunnel destination cannot be derived does the
	// peer's physical path stand in as nearest-available evidence, labelled as
	// exactly that.
	TraceSubjectTunnelTargetUnreachable = "tunnel_target_unreachable"
	// TraceSubjectTunnelNotAttempted: the tunnel was never used, because the
	// pinned proxy was missing, disabled, unusable for the probe kind or failed
	// to initialize. No packet left the host, so nothing was observed about the
	// tunnel or the target — the peer trace is only a reachability check, and the
	// fault is a configuration problem.
	TraceSubjectTunnelNotAttempted = "tunnel_not_attempted"
	// An empty subject reason on a WireGuard plan means none of the above could
	// be established, and no verdict may be asserted.
)

// Terminal traceroute statuses (TraceResult.Status). Pre-terminal queued/running
// states live server-side and are never sent by the agent.
const (
	TraceStatusSucceeded   = "succeeded"
	TraceStatusPartial     = "partial"
	TraceStatusTimedOut    = "timed_out"
	TraceStatusUnsupported = "unsupported"
	TraceStatusFailed      = "failed"
	TraceStatusCanceled    = "canceled"
)

// Trace path scopes (TraceResult.PathScope / server-side trace_reports.path_scope).
// The scope is orthogonal to the trace subject: it answers "which path did the
// probes travel", not "who is being measured". WireGuardPhysical is planned
// server-side only — a host-stack trace toward a peer's physical endpoint — so
// agents attest Direct for it.
const (
	TracePathDirect            = "direct"
	TracePathWireGuardPhysical = "wireguard_physical"
	TracePathWireGuardInner    = "wireguard_inner"
)

// TraceHop is one TTL and all its per-attempt probe responses.
type TraceHop struct {
	TTL      int            `json:"ttl"`
	Attempts []TraceAttempt `json:"attempts,omitempty"`
}

// TraceAttempt is one probe sent at a hop's TTL. Timeout means no responder
// answered within the per-attempt budget (rendered as `*`); ResponderAddr,
// Hostname, and RTTMs are then empty. Hostname is populated only when reverse
// DNS was enabled on the request and resolution succeeded within budget.
type TraceAttempt struct {
	ResponderAddr string  `json:"responder_addr,omitempty"`
	Hostname      string  `json:"hostname,omitempty"`
	RTTMs         float64 `json:"rtt_ms,omitempty"`
	Timeout       bool    `json:"timeout,omitempty"`
}

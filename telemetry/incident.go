package telemetry

import "time"

// SceneReport is the agent's allowlisted evidence about its own surroundings at
// the moment it decided something was broken (INCIDENT-005). The agent collects
// it on a fault edge it detected itself and ships it in the WAL; the server
// claims it afterwards as evidence for whichever incident the trigger identifies.
//
// # Why the agent decides, and why that changes what a scene means
//
// The server used to command this: an incident opened, the opening transaction
// froze a target list, and a request went down the socket. That contract cannot
// survive the fault it exists to explain — the agent most worth asking is the
// one that just went unreachable, and a command to an offline agent is a no-op.
// A scene now describes what the AGENT saw when IT detected the fault, which is
// a different (and earlier) statement than what the server would have asked for
// when it opened the incident. Nothing server-side can reconstruct it, so the
// report carries its own reason for existing in Triggers.
//
// Collection is allowlisted by construction: the typed field groups below carry
// only network context, agent identity, a basic resource summary, and target
// resolution — never process lists, user names, file paths, credentials,
// request/response headers, or bodies. Each attempted group reports its own
// collected/denied/unsupported/failed outcome in Groups so a partial scene is
// still a complete answer; a group's typed payload is set only when that group's
// Status is collected.
type SceneReport struct {
	// ReportID is minted by the agent (a UUID) and is the server's idempotency
	// key together with the authenticated agent id, exactly as for TraceResult:
	// a replayed packet carries the same id and is a no-op.
	ReportID string `json:"report_id"`

	// CollectedAt is the agent clock when collection finished. The server keeps
	// its own receipt time and reports the difference as clock skew rather than
	// trusting either one.
	CollectedAt time.Time `json:"collected_at"`

	// Triggers are the fault edges this scene answers for, at least one. It is a
	// list and not a single value because collection takes real time and faults
	// arrive in clusters: an edge crossed while a scene is already being gathered
	// joins that scene instead of queueing a second copy of the same machine. Each
	// entry carries enough identity to be claimed on its own, so one scene can be
	// filed as evidence under several incidents.
	Triggers []SceneTrigger `json:"triggers"`

	Groups []SnapshotGroupResult `json:"groups"` // one result per attempted field group, always present

	Network   *SnapshotNetwork       `json:"network,omitempty"`   // set iff the network group collected
	Agent     *SnapshotAgentInfo     `json:"agent,omitempty"`     // set iff the agent-info group collected
	Resources *SnapshotResources     `json:"resources,omitempty"` // set iff the resources group collected
	Targets   []SnapshotTargetResult `json:"targets,omitempty"`   // set iff the target-resolution group collected
}

// What made an agent collect a scene (SceneTrigger.Kind).
const (
	// SceneTriggerProbeFault: a monitored target failed enough consecutive
	// rounds to cross the agent's local confirmation threshold — the same edge
	// that fires a traceroute.
	SceneTriggerProbeFault = "probe_fault"
	// SceneTriggerServerDisconnect: the agent's session to THIS server ended and
	// it is about to retry. No probe streak accompanies it — an agent-connectivity
	// fault is detected server-side, by a sweeper noticing the agent is gone, and
	// crosses no local probe edge at all. Without this trigger a connectivity
	// incident would have no scene to claim, which is the case that most needs one.
	SceneTriggerServerDisconnect = "server_disconnect"
)

// SceneTrigger is one fault edge that caused (or joined) a scene collection. It
// is the claim key: the server matches it against its own confirmed fault
// signals to decide which incident the scene is evidence for.
//
// # Why the identity is (MonitorID, ConfigSerial) and not a time window
//
// A window alone cannot say which of two targets failing in the same minute a
// scene belongs to, and it cannot tell a scene collected under the target's old
// definition from one collected after an edit. The monitor id answers the first
// and the material generation answers the second — the same serial that already
// participates in metric series identity, so stale-generation evidence cannot
// surface under a target it never described.
type SceneTrigger struct {
	Kind string `json:"kind"` // SceneTrigger* — which of the two edges below is filled in

	// ---- probe_fault ----

	// MonitorID is the failing monitor (probe_tasks.id) and ConfigSerial the
	// material generation of the target as the agent had it when the streak
	// confirmed. Together they are the claim key.
	MonitorID    string `json:"monitor_id,omitempty"`
	ConfigSerial int    `json:"config_serial,omitempty"`

	// TriggerStreak is how many consecutive failing rounds it took and
	// FirstFailedAt when that streak began. FirstFailedAt is also the ordering
	// gate on the claim: a scene from a streak that started before the incident's
	// own observation window describes an earlier fault, not this one.
	TriggerStreak int       `json:"trigger_streak,omitempty"`
	FirstFailedAt time.Time `json:"first_failed_at,omitempty"`

	// ---- server_disconnect ----

	// DisconnectedAt is the agent clock when the session ended and Reason the
	// agent's stable classification of what ended it (dns/refused/timeout/tls/
	// network/ack_timeout). The pair is the connectivity claim key together with
	// the agent id: the server's agent-connectivity signal is per-agent and
	// carries no target, so no further discriminator is needed.
	DisconnectedAt time.Time `json:"disconnected_at,omitempty"`
	Reason         string    `json:"reason,omitempty"`

	// EdgeCount is how many disconnect edges this one entry stands for, always at
	// least 1. A flapping link produces edges faster than a scene is worth
	// collecting, so they merge into one trigger rather than one scene each — and
	// the count is what keeps the merge from reading as a single clean drop.
	EdgeCount int `json:"edge_count,omitempty"`
}

// Scene field-group ids (SnapshotGroupResult.Group). Each group is gathered and
// reported independently.
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

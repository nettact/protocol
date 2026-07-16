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
	ErrorClass  string   `json:"error_class,omitempty"` // e.g. dns_error | connect_refused | timeout | tls_error
}

// TraceResult is the agent's terminal result for one config.TraceRequest
// (DIAG-001). It is answered once per ReportID; every incident/alert that
// referenced the shared report reads status and hops through it. Intermediate
// `*` (a timed-out attempt) never implies a broken path — only a responder from
// the destination sets Reached.
type TraceResult struct {
	ReportID string `json:"report_id"`        // echoes config.TraceRequest.ReportID
	Mode     string `json:"mode"`             // echoes the request: config.TraceModeICMP | config.TraceModeTCP
	Status   string `json:"status"`           // terminal TraceStatus* value
	Reason   string `json:"reason,omitempty"` // stable reason code for unsupported/failed/canceled

	DestinationIP string `json:"destination_ip,omitempty"` // resolved destination address the trace ran against
	Reached       bool   `json:"reached"`                  // true once the destination itself responded
	ReachedTTL    int    `json:"reached_ttl,omitempty"`    // TTL/hop number at which the destination replied

	StartedAt   time.Time  `json:"started_at,omitempty"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	Hops        []TraceHop `json:"hops,omitempty"`
}

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

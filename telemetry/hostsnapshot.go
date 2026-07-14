package telemetry

import "time"

// HostSnapshot is an ephemeral, pull-on-demand view of a host's running
// processes and network connections. Unlike Metric samples, it is never stored:
// the agent collects it only when the server (on behalf of a console user
// viewing the live processes page) sets a config.SnapshotRequest, and the server
// keeps only the latest snapshot per agent in memory. It is also never alerted
// on.
//
// Collection is scope-gated: the agent invokes an OS/gopsutil operation only for
// a granted scope and never enumerates-then-redacts. Fields that a scope can
// deny use pointers so their presence/absence survives Go, JSON, and protobuf.
// Scopes carries one result per requested scope so a partial result completes
// immediately instead of timing out.
type HostSnapshot struct {
	TS        time.Time             `json:"ts"`
	RequestID string                `json:"request_id"` // echoes config.SnapshotRequest.RequestID
	Scopes    []SnapshotScopeResult `json:"scopes"`     // one per requested scope, always present

	ProcessTotal *int             `json:"process_total,omitempty"` // set iff host.process.basic.read collected
	Processes    []ProcessInfo    `json:"processes,omitempty"`
	Connections  []ConnectionInfo `json:"connections,omitempty"`
}

// SnapshotScopeResult is the outcome of one requested scope.
type SnapshotScopeResult struct {
	Scope  string `json:"scope"`
	Status string `json:"status"`           // collected | denied | unsupported | failed
	Reason string `json:"reason,omitempty"` // e.g. unsatisfied_dependency, unknown_scope, rate_limited
}

// Snapshot scope status values.
const (
	ScopeCollected   = "collected"
	ScopeDenied      = "denied"
	ScopeUnsupported = "unsupported"
	ScopeFailed      = "failed"
)

// ProcessInfo mirrors one row of the NeoHtop process table. Rows exist only when
// the basic scope was collected; the pointer fields are populated only under
// their owning scope (owner / resource / io).
type ProcessInfo struct {
	PID    int32  `json:"pid"`              // basic
	Name   string `json:"name"`             // basic
	Status string `json:"status,omitempty"` // basic — Running / Sleeping / …

	User *string `json:"user,omitempty"` // owner scope

	CPUPct         *float64 `json:"cpu_pct,omitempty"`          // resource scope
	RSSBytes       *uint64  `json:"rss_bytes,omitempty"`        // resource scope
	VirtBytes      *uint64  `json:"virt_bytes,omitempty"`       // resource scope
	RunTimeSeconds *float64 `json:"run_time_seconds,omitempty"` // resource scope

	DiskReadBytes  *uint64 `json:"disk_read_bytes,omitempty"`  // io scope
	DiskWriteBytes *uint64 `json:"disk_write_bytes,omitempty"` // io scope
}

// ConnectionInfo is one active network connection. Rows exist only when the
// summary scope was collected; the pointer fields are populated only under their
// owning scope (local / remote / owner).
type ConnectionInfo struct {
	Proto string `json:"proto"`           // summary — tcp / tcp6 / udp / udp6
	State string `json:"state,omitempty"` // summary — ESTABLISHED / LISTEN / …

	LocalAddr   *string `json:"local_addr,omitempty"`   // local scope
	RemoteAddr  *string `json:"remote_addr,omitempty"`  // remote scope
	PID         *int32  `json:"pid,omitempty"`          // owner scope
	ProcessName *string `json:"process_name,omitempty"` // owner scope
}

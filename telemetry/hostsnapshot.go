package telemetry

import "time"

// HostSnapshot is an ephemeral, pull-on-demand view of a host's running
// processes and network connections. Unlike Metric samples, it is never stored:
// the agent collects it only when the server (on behalf of a console user
// viewing the live processes page) sets a config.SnapshotRequest, and the server
// keeps only the latest snapshot per agent in memory. It is also never alerted
// on. The agent honors a request only when started with the matching
// --report-processes / --report-connections flag; otherwise the fields stay nil.
type HostSnapshot struct {
	TS           time.Time        `json:"ts"`
	RequestID    string           `json:"request_id"`             // echoes config.SnapshotRequest.RequestID
	ProcessTotal int              `json:"process_total"`          // total process count (Processes may be a subset)
	Processes    []ProcessInfo    `json:"processes,omitempty"`    // nil when --report-processes is off
	Connections  []ConnectionInfo `json:"connections,omitempty"`  // nil when --report-connections is off
}

// ProcessInfo mirrors one row of the NeoHtop process table.
type ProcessInfo struct {
	PID            int32   `json:"pid"`
	Name           string  `json:"name"`
	User           string  `json:"user,omitempty"`
	Status         string  `json:"status,omitempty"` // Running / Sleeping / …
	CPUPct         float64 `json:"cpu_pct"`
	RSSBytes       uint64  `json:"rss_bytes"`        // resident memory (RAM column)
	VirtBytes      uint64  `json:"virt_bytes"`       // virtual memory (VIRT column)
	DiskReadBytes  uint64  `json:"disk_read_bytes"`  // Disk R column
	DiskWriteBytes uint64  `json:"disk_write_bytes"` // Disk W column
	RunTimeSeconds float64 `json:"run_time_seconds"` // now - create time (Run Time column)
}

// ConnectionInfo is one active network connection with its owning process.
type ConnectionInfo struct {
	Proto       string `json:"proto"` // tcp / tcp6 / udp / udp6
	LocalAddr   string `json:"local_addr"`
	RemoteAddr  string `json:"remote_addr,omitempty"`
	State       string `json:"state,omitempty"` // ESTABLISHED / LISTEN / …
	PID         int32  `json:"pid,omitempty"`
	ProcessName string `json:"process_name,omitempty"`
}

package telemetry

import "time"

// Metric is a single-value time-series sample. One probe run may emit several
// metrics sharing a timestamp and target (e.g. rtt + loss for one ping),
// which is the best fit for a narrow time-series table.
type Metric struct {
	TS     time.Time         `json:"ts"`
	Kind   MetricKind        `json:"kind"`
	Target string            `json:"target,omitempty"` // "gateway", "1.1.1.1", "https://…", "eth0"
	Layer  HealthLayer       `json:"layer,omitempty"`
	Value  float64           `json:"value"`
	Unit   string            `json:"unit,omitempty"` // "ms", "pct", "bool"
	Labels map[string]string `json:"labels,omitempty"`
}

// MetricKind is a string enum so unknown kinds are ignored rather than failing
// to decode on older servers.
type MetricKind string

const (
	ICMPRTTms  MetricKind = "probe.icmp.rtt_ms"
	ICMPLoss   MetricKind = "probe.icmp.loss_pct"
	ICMPJitter MetricKind = "probe.icmp.jitter_ms"
	DNSResolve MetricKind = "probe.dns.resolve_ms"
	DNSOK      MetricKind = "probe.dns.ok"
	HTTPStatus MetricKind = "probe.http.status"
	HTTPLat    MetricKind = "probe.http.latency_ms"
	HTTPOK     MetricKind = "probe.http.ok"
	TCPOK        MetricKind = "probe.tcp.ok"
	TCPConnectMs MetricKind = "probe.tcp.connect_ms"

	// NAT / STUN behavior discovery (LayerWAN). Categorical results are encoded as
	// stable numeric codes in Value (Unit=UnitCode), ordered so a higher code is a
	// "worse" NAT — a rule can fire with gte. The mapped/reflexive public address is
	// carried on the success Event (Attrs["mapped_addr"]), not as a stored label.
	NATOK        MetricKind = "probe.nat.ok"        // bool: STUN binding reachable (1) or not (0)
	NATRTTms     MetricKind = "probe.nat.rtt_ms"    // binding round-trip latency
	NATMapping   MetricKind = "probe.nat.mapping"   // code: 0 unknown, 1 endpoint-independent, 2 address-dependent, 3 address-and-port-dependent
	NATFiltering MetricKind = "probe.nat.filtering" // code: 0 unknown, 1 endpoint-independent, 2 address-dependent, 3 address-and-port-dependent (udp only)
	NATType      MetricKind = "probe.nat.type"      // code: 0 unknown, 1 open, 2 full-cone, 3 restricted-cone, 4 port-restricted-cone, 5 symmetric (udp only)

	IfaceUp    MetricKind = "iface.up"

	// Agent self-status (heartbeat), so status is reported outbound without the
	// agent exposing any endpoint.
	AgentUptime     MetricKind = "agent.uptime_s"
	AgentWALPending MetricKind = "agent.wal_pending"

	// Host / system metrics (LayerLocal). Emitted only when the agent is started
	// with --report-host; stored as ordinary time-series so History graphs and
	// the alert engine work unchanged. Modeled on the NeoHtop dashboard.
	HostCPUPct     MetricKind = "host.cpu.pct"      // overall CPU utilization, Target="host"
	HostCPUCorePct MetricKind = "host.cpu.core.pct" // per-core, Target="core0","core1",…
	HostMemPct     MetricKind = "host.mem.pct"
	HostMemTotal   MetricKind = "host.mem.total" // bytes
	HostMemUsed    MetricKind = "host.mem.used"  // bytes
	HostMemFree    MetricKind = "host.mem.free"  // bytes
	HostDiskPct    MetricKind = "host.disk.pct"  // per mount, Target=mount ("C:")
	HostDiskTotal  MetricKind = "host.disk.total"
	HostDiskUsed   MetricKind = "host.disk.used"
	HostDiskFree   MetricKind = "host.disk.free"
	HostLoad1      MetricKind = "host.load.1m"
	HostLoad5      MetricKind = "host.load.5m"
	HostLoad15     MetricKind = "host.load.15m"
	HostUptime     MetricKind = "host.uptime_s"
	HostNetRxBps   MetricKind = "host.net.rx_bps" // receive rate, bytes/s
	HostNetTxBps   MetricKind = "host.net.tx_bps" // send rate, bytes/s
)

// Units for Metric.Value.
const (
	UnitMs    = "ms"
	UnitPct   = "pct"
	UnitBool  = "bool"
	UnitCode  = "code"
	UnitCount = "count"
	UnitSec   = "s"
	UnitBytes = "bytes"
	UnitBps   = "bps"  // bytes per second
	UnitLoad  = "load" // load average (dimensionless)
)

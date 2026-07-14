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
	// MonitorID is the user-created monitor (probe task) that produced this
	// sample; empty for system metrics (host.*, iface.up, agent.*, the built-in
	// gateway probe). It keys the server's series so same-target monitors stay
	// distinct.
	MonitorID string `json:"monitor_id,omitempty"`
}

// MetricKind is a string enum so unknown kinds are ignored rather than failing
// to decode on older servers.
type MetricKind string

const (
	// ICMP probe results. One ping cycle sends packet_count echoes (default 5)
	// and emits the loss, the distribution over RECEIVED echoes (avg/min/max), and
	// the count received — all sharing one TS+Target+MonitorID. Errors/timeouts are
	// NEVER recorded as zero-latency: rtt/min/max/jitter are emitted only when the
	// corresponding samples exist, so loss/sample-count carry the failures instead.
	ICMPRTTms   MetricKind = "probe.icmp.rtt_ms"     // mean RTT over received echoes
	ICMPLoss    MetricKind = "probe.icmp.loss_pct"   // (sent-received)/sent*100
	ICMPRTTMin  MetricKind = "probe.icmp.rtt_min_ms" // min RTT over received echoes
	ICMPRTTMax  MetricKind = "probe.icmp.rtt_max_ms" // max RTT over received echoes
	ICMPJitter  MetricKind = "probe.icmp.jitter_ms"  // IPDV: mean |Δ| of adjacent received RTTs (emitted only when received>=2)
	ICMPSamples MetricKind = "probe.icmp.samples"    // count: echoes received this cycle (with loss ⇒ sent)

	DNSResolve MetricKind = "probe.dns.resolve_ms"
	DNSOK      MetricKind = "probe.dns.ok"
	HTTPStatus MetricKind = "probe.http.status"
	HTTPLat    MetricKind = "probe.http.latency_ms"
	HTTPOK     MetricKind = "probe.http.ok"

	// TCP probe results (single connect per cycle). The dial is split into distinct
	// timed segments so a slow-DNS vs slow-connect vs slow-TLS problem is separable,
	// and the failure is classified by error_class. connect_ms is the PURE TCP
	// connect only (DNS and TLS are separate) and is emitted only on success.
	TCPOK         MetricKind = "probe.tcp.ok"          // bool: connect (+ optional TLS) succeeded
	TCPDNSms      MetricKind = "probe.tcp.dns_ms"      // hostname resolution time (omitted for literal-IP targets)
	TCPConnectMs  MetricKind = "probe.tcp.connect_ms"  // pure TCP connect time (success only)
	TCPTLSms      MetricKind = "probe.tcp.tls_ms"      // TLS handshake time (only when TLS enabled and connect succeeded)
	TCPErrorClass MetricKind = "probe.tcp.error_class" // code: 0 none,1 timeout,2 refused,3 unreachable,4 dns,5 tls,9 other

	// NAT / STUN behavior discovery (LayerWAN). Categorical results are encoded as
	// stable numeric codes in Value (Unit=UnitCode), ordered so a higher code is a
	// "worse" NAT — a rule can fire with gte. The mapped/reflexive public address is
	// carried on the success Event (Attrs["mapped_addr"]), not as a stored label.
	NATOK        MetricKind = "probe.nat.ok"        // bool: STUN binding reachable (1) or not (0)
	NATRTTms     MetricKind = "probe.nat.rtt_ms"    // binding round-trip latency
	NATMapping   MetricKind = "probe.nat.mapping"   // code: 0 unknown, 1 endpoint-independent, 2 address-dependent, 3 address-and-port-dependent
	NATFiltering MetricKind = "probe.nat.filtering" // code: 0 unknown, 1 endpoint-independent, 2 address-dependent, 3 address-and-port-dependent (udp only)
	NATType      MetricKind = "probe.nat.type"      // code: 0 unknown, 1 open, 2 full-cone, 3 restricted-cone, 4 port-restricted-cone, 5 symmetric (udp only)

	IfaceUp MetricKind = "iface.up"

	// Local Wi-Fi link metrics (LayerWireless). Emitted per wireless adapter by
	// the interface collector, Target=<interface name>, MonitorID="". Only honest
	// known values are emitted: an unreadable adapter produces NO wifi.up sample
	// (a chart gap, never a synthetic zero/third-state), and the numeric link
	// values are emitted only when connected AND the driver reports them.
	WiFiUp         MetricKind = "wifi.up"           // bool: 1 connected / 0 disconnected (readable only)
	WiFiSignalDBm  MetricKind = "wifi.signal_dbm"   // signal strength, dBm
	WiFiQualityPct MetricKind = "wifi.quality_pct"  // link quality, percent
	WiFiLinkRxMbps MetricKind = "wifi.link_rx_mbps" // receive link rate, Mbps
	WiFiLinkTxMbps MetricKind = "wifi.link_tx_mbps" // transmit link rate, Mbps

	// Agent self-status (heartbeat), so status is reported outbound without the
	// agent exposing any endpoint.
	AgentUptime     MetricKind = "agent.uptime_s"
	AgentWALPending MetricKind = "agent.wal_pending"

	// Host / system metrics (LayerLocal). Emitted only when the agent is granted
	// the matching host.* permission; stored as ordinary time-series so History
	// graphs and the alert engine work unchanged. Modeled on the NeoHtop dashboard.
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
	UnitDBm   = "dbm"  // signal strength in decibel-milliwatts
	UnitMbps  = "mbps" // link rate in megabits per second
)

// TCP error-class codes carried in probe.tcp.error_class (Unit=UnitCode). Stable
// numeric codes shared by the producing agent and the consuming server/UI so the
// meaning never drifts. Emitted every cycle (TCPErrNone on success).
const (
	TCPErrNone        = 0 // connect (+ optional TLS) succeeded
	TCPErrTimeout     = 1 // no answer within the deadline
	TCPErrRefused     = 2 // connection actively refused (host up, port closed)
	TCPErrUnreachable = 3 // host/network unreachable (no route)
	TCPErrDNS         = 4 // hostname resolution failed
	TCPErrTLS         = 5 // TCP connected but the TLS handshake failed
	TCPErrOther       = 9 // any other connect error
)

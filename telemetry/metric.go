package telemetry

import (
	"strings"
	"time"
)

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
	// sample; empty for system metrics (host.*, iface.up, agent.*). It keys the
	// server's series so same-target monitors stay distinct.
	MonitorID string `json:"monitor_id,omitempty"`
	// ConfigSerial is the probe target's material config generation
	// (probe_tasks.config_serial) this sample was produced under, stamped by the
	// probing collector from the applied ProbeTarget. Zero for system metrics
	// (MonitorID == ""). The server rejects samples whose serial does not match
	// the target's current generation (obsolete backlog / replay / forged future).
	ConfigSerial int `json:"config_serial,omitempty"`
}

// MetricKind is a string enum so unknown kinds are ignored rather than failing
// to decode on older servers.
type MetricKind string

const (
	// ICMP probe results. One ping cycle attempts up to packet_count echoes
	// (default 5) and emits the loss, the distribution over RECEIVED echoes
	// (avg/min/max), the count received, and the count actually sent — all sharing
	// one TS+Target+MonitorID. Errors/timeouts are NEVER recorded as zero-latency:
	// rtt/min/max/jitter are emitted only when the corresponding samples exist, so
	// loss/sample-count carry the failures instead.
	ICMPRTTms   MetricKind = "probe.icmp.rtt_ms"     // mean RTT over received echoes
	ICMPLoss    MetricKind = "probe.icmp.loss_pct"   // (sent-received)/sent*100, over echoes actually SENT
	ICMPRTTMin  MetricKind = "probe.icmp.rtt_min_ms" // min RTT over received echoes
	ICMPRTTMax  MetricKind = "probe.icmp.rtt_max_ms" // max RTT over received echoes
	ICMPJitter  MetricKind = "probe.icmp.jitter_ms"  // IPDV: mean |Δ| of adjacent received RTTs (emitted only when received>=2)
	ICMPSamples MetricKind = "probe.icmp.samples"    // count: echoes received this cycle
	// ICMPSent is how many echoes the cycle ACTUALLY sent, which is normally the
	// target's configured packet_count. It is less when the agent's probe-concurrency
	// budget could not admit every echo inside the cycle's own timing budget — an
	// overloaded agent, not a network fault.
	//
	// It exists so loss stays honest under that overload. Loss is a ratio over what
	// was sent, so a cycle that managed one echo reports either 0% or 100% — figures
	// indistinguishable from a healthy or a dead target on their own. The server
	// therefore compares this against the target's configured packet_count and
	// refuses to move a monitor's availability state on any round where the two
	// differ: an incomplete round is stored and graphed, but it can neither confirm
	// nor clear an outage. Without it an overloaded agent would silently invent
	// both false recoveries and false faults.
	ICMPSent MetricKind = "probe.icmp.sent"
	// ICMPErrorClass classifies a fully-failed ping cycle (received==0) into a
	// ProbeReason* code (UnitCode); ProbeReasonNone when the target answered. Emitted
	// by both the public-ping and gateway-ping collectors via appendICMPMetrics.
	ICMPErrorClass MetricKind = "probe.icmp.error_class"
	// ICMPSizeSweep classifies whether loss rises with ICMP payload size, emitted
	// every cycle when the target's size sweep is on (ProbeParams.SizeSweep).
	// Unit=code; codes:
	//   0 = flat — loss not size-correlated (the congestion/queuing signature)
	//   1 = size-correlated — large payloads lose far more than small ones (the
	//       physical-layer fingerprint: optics / CRC / FEC / ASIC / policer)
	//   2 = insufficient evidence — too few echoes at the compared sizes to judge
	// The compared sizes and per-size loss ride as labels (SizeSmallLabel,
	// SizeLargeLabel, LossSmallLabel, LossLargeLabel, CountSmallLabel,
	// CountLargeLabel) so the server can render the evidence without re-deriving
	// it and the console can chart the two loss figures side by side.
	ICMPSizeSweep MetricKind = "probe.icmp.size_sweep"

	DNSResolve MetricKind = "probe.dns.resolve_ms"
	DNSOK      MetricKind = "probe.dns.ok"
	// DNSErrorClass classifies a resolve failure into a ProbeReason* code (UnitCode);
	// ProbeReasonNone on success. Emitted every cycle.
	DNSErrorClass MetricKind = "probe.dns.error_class"
	HTTPStatus    MetricKind = "probe.http.status"
	HTTPLat       MetricKind = "probe.http.latency_ms"
	HTTPOK        MetricKind = "probe.http.ok"
	// HTTPErrorClass classifies the probe failure into a ProbeReason* code (UnitCode):
	// a transport failure (DNS/refused/timeout/TLS/…) by error type, a completed
	// request that fails the acceptance check as HTTPStatus/HTTPKeyword. ProbeReasonNone
	// only when the probe fully passed. Emitted every cycle.
	HTTPErrorClass MetricKind = "probe.http.error_class"

	// TCP probe results (single connect per cycle). The dial is split into distinct
	// timed segments so a slow-DNS vs slow-connect vs slow-TLS problem is separable,
	// and the failure is classified by error_class. connect_ms is the PURE TCP
	// connect only (DNS and TLS are separate) and is emitted only on success.
	TCPOK         MetricKind = "probe.tcp.ok"          // bool: connect (+ optional TLS) succeeded
	TCPDNSms      MetricKind = "probe.tcp.dns_ms"      // hostname resolution time (omitted for literal-IP targets)
	TCPConnectMs  MetricKind = "probe.tcp.connect_ms"  // pure TCP connect time (success only)
	TCPTLSms      MetricKind = "probe.tcp.tls_ms"      // TLS handshake time (only when TLS enabled and connect succeeded)
	TCPErrorClass MetricKind = "probe.tcp.error_class" // ProbeReason* code (UnitCode); classifies the connect/TLS failure
	// TCPFlowFanout classifies a TCP target probed with several pinned source
	// ports (ProbeParams.FlowFanout >= 2), emitted every cycle. Unit=code; codes:
	//   0 = single flow — fan-out off, or unsupported here (e.g. proxied target)
	//   1 = uniform — failures/latency spread across flows (congestion signature)
	//   2 = member-level — a deterministic subset of flows fails while the rest
	//       stay clean, stable across cycles (ECMP/LAG member fault signature)
	//   3 = all flows failed — the target is unreachable, not merely degraded
	//   4 = insufficient evidence — too few flows or too short a history to judge
	// Flow counts ride as labels (FlowFanoutFlowsLabel, FlowFanoutBadStableLabel,
	// FlowFanoutBadNewLabel, FlowFanoutOKLabel).
	TCPFlowFanout MetricKind = "probe.tcp.flow_fanout"

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

	// Logical core count, Target="host". Inventory rather than a measurement: it
	// exists so the server can judge load per core, because a raw load average is
	// meaningless without it (2.0 idles a 16-core box and pins a 2-core one). The
	// agent therefore emits it when EITHER cpu or load reporting is granted, so by
	// RequiredForHostMetric's prefix rule it can arrive under a load-only grant.
	// That is deliberate: a core count is not a CPU measurement, and refusing to
	// send it would leave load alerting unable to say anything at all.
	HostCPUCores MetricKind = "host.cpu.cores"

	// Temperature follows the CPU aggregate/detail split: the "host" series is
	// the hottest plausible sensor (what the console overview graphs), the
	// per-sensor series keeps the detail. Sensors are wildly platform-dependent,
	// so both are emitted only when a real reading succeeds.
	HostTempC       MetricKind = "host.temp.c"        // hottest sensor, Target="host"
	HostTempSensorC MetricKind = "host.temp.sensor.c" // per sensor, Target=sanitized sensor key

	// DERIVED kinds. No agent ever emits these and no series carries them: they
	// exist only as the frozen metric_kind on system-status fault evidence, where
	// the raw series is in the wrong unit to be read back honestly. A load alert
	// is authored per core, so freezing host.load.1m=12.4 against a threshold of
	// 2.0 would render "12.4 (≥ 2)" and read like a six-fold breach on a box where
	// it is a 1.55 one; a network alert is authored in Mbps while the series is
	// bytes/s. Evidence has to speak the unit the operator set the threshold in.
	HostLoadPerCore MetricKind = "host.load.per_core" // host.load.1m ÷ logical cores
	HostNetRxMbps   MetricKind = "host.net.rx_mbps"   // host.net.rx_bps × 8 ÷ 1e6
	HostNetTxMbps   MetricKind = "host.net.tx_mbps"   // host.net.tx_bps × 8 ÷ 1e6
)

// Game presentation data is deliberately absent from this list. It does not fit
// a narrow time series: a second of frames is a distribution, not a value, and
// the figures players ask for (1% low, stutter counts) are properties of a whole
// play session rather than of any one second. It rides its own run/bucket model
// instead — see protocol/gamesense.

// Units for Metric.Value.
const (
	UnitMs      = "ms"
	UnitPct     = "pct"
	UnitBool    = "bool"
	UnitCode    = "code"
	UnitCount   = "count"
	UnitSec     = "s"
	UnitBytes   = "bytes"
	UnitBps     = "bps"  // bytes per second
	UnitLoad    = "load" // load average (dimensionless)
	UnitDBm     = "dbm"  // signal strength in decibel-milliwatts
	UnitMbps    = "mbps" // link rate in megabits per second
	UnitCelsius = "c"    // temperature in degrees Celsius
)

// Probe failure-reason codes carried in the probe.*.error_class metrics
// (Unit=UnitCode). Stable numeric codes shared by every producing collector
// (icmp/dns/http/tcp) and the consuming server/UI so the meaning never drifts.
// Emitted every cycle (ProbeReasonNone on success). Not every code applies to
// every probe (e.g. a raw ICMP echo is never "refused"/"tls").
//
// The single-digit codes are failure FAMILIES; the two-digit codes refine a
// family (4x=DNS, 5x=TLS, 7x=HTTP acceptance), grouped by tens so family
// membership is code/10 or the base digit. A collector that cannot
// discriminate finer emits the family code, so every consumer must render an
// unknown code as at least "other". Whenever the code is non-None the
// collector also attaches the raw underlying cause (e.g. the OS error text,
// "HTTP 503", "NXDOMAIN") as Labels["detail"] on the error_class metric —
// capped at 256 chars, never localized: the code carries the translated
// meaning, the detail carries the machine truth. The server freezes it onto
// alert evidence as reason_detail.
const (
	ProbeReasonNone        = 0 // the probe succeeded
	ProbeReasonTimeout     = 1 // no answer within the deadline
	ProbeReasonRefused     = 2 // connection actively refused (host up, port closed)
	ProbeReasonUnreachable = 3 // host/network unreachable (no route)
	ProbeReasonDNS         = 4 // name resolution failed (family; unclassified resolver error)
	ProbeReasonTLS         = 5 // connected but the TLS handshake failed (family; unclassified TLS error)
	ProbeReasonReset       = 6 // connection reset/aborted by peer mid-exchange
	ProbeReasonOther       = 9 // any other failure

	// The DNS refinements may be asserted only from a conclusive answer: a
	// readable rcode, or a completed lookup that returned nothing. A stub
	// resolver's "no such host" is NOT conclusive (it collapses NXDOMAIN and
	// NODATA) and a truncated answer is not either — both stay ProbeReasonDNS,
	// so a missing record is never reported as a missing domain.
	ProbeReasonDNSNXDomain = 41 // the queried name does not exist (NXDOMAIN rcode)
	ProbeReasonDNSServFail = 42 // the DNS server answered SERVFAIL
	ProbeReasonDNSNoRecord = 43 // name exists but has no record of the queried type

	ProbeReasonTLSExpired   = 51 // certificate expired (or not yet valid)
	ProbeReasonTLSUntrusted = 52 // certificate chain not trusted (unknown authority)
	ProbeReasonTLSHostname  = 53 // certificate valid but not for the requested hostname

	ProbeReasonHTTPStatus  = 71 // request completed but the status code failed the acceptance check
	ProbeReasonHTTPKeyword = 72 // request completed but the body keyword check failed

	// The 8x family covers a probe pinned to an egress proxy that failed BEFORE the
	// target was ever reached. Keeping these apart from the 1-6 families is the whole
	// point of the family: "the proxy is down" and "the monitored service is down"
	// are opposite operational conclusions, and collapsing an unreachable proxy into
	// ProbeReasonTimeout would page someone about a service that is perfectly fine.
	// A code here always means the failure is on the egress path, never at the target.
	ProbeReasonProxyConnect = 81 // could not reach the proxy, or its handshake/tunnel failed
	ProbeReasonProxyAuth    = 82 // the proxy rejected the supplied credentials
	ProbeReasonProxyDNS     = 83 // the proxy could not resolve the target hostname
	ProbeReasonProxyRefused = 84 // the proxy reached but refused to relay to this target
	// ProbeReasonProxyConfig marks a probe that never dialed at all: the pinned proxy
	// is absent from the pushed config, unusable for this probe kind, or failed to
	// initialize. It is deliberately a probe FAILURE rather than a direct dial —
	// falling back would leak the real egress IP and make an "up" verdict a lie.
	ProbeReasonProxyConfig = 85
)

// ProbeReasonDetailLabel is the Metric.Labels key on which a probe.*.error_class
// sample carries the raw underlying error (see the ProbeReason* contract above).
const ProbeReasonDetailLabel = "detail"

// Labels naming the endpoint a probe actually TALKED TO, as opposed to the
// endpoint it was asked about. A DNS monitor's Target is the queried name, so
// without these the resolver that failed is unnameable; a NAT monitor's Target
// is the STUN server itself, but the resolved host:port (defaults applied) is
// only known to the collector. The server freezes them onto fault evidence and
// aims the path diagnostic at them, so the values are wire contracts: a probe
// that cannot name its endpoint omits the label rather than guessing.
const (
	// DNSResolverLabel carries the resolver endpoint a probe.dns.error_class
	// sample's query actually used — "host:port", or the DoH URL. Absent when
	// the agent could not name the system resolver on this platform.
	DNSResolverLabel = "resolver"
	// DNSResolverProtocolLabel is that resolver's protocol: udp | tcp | dot | doh.
	DNSResolverProtocolLabel = "resolver_protocol"

	// NATServerLabel is the resolved STUN server "host:port" on probe.nat.*.
	NATServerLabel = "server"
	// NATTransportLabel is the STUN transport on probe.nat.*: udp | tcp | tls | dtls.
	NATTransportLabel = "transport"

	// SizeSmallLabel / SizeLargeLabel are the two payload sizes a probe.icmp.size_sweep
	// sample compared (bytes); LossSmallLabel / LossLargeLabel the loss percent at each
	// ("%.1f"); CountSmallLabel / CountLargeLabel the echoes sent at each. The comparison
	// is the smallest vs the largest swept size.
	SizeSmallLabel  = "size_small"
	SizeLargeLabel  = "size_large"
	LossSmallLabel  = "loss_small"
	LossLargeLabel  = "loss_large"
	CountSmallLabel = "count_small"
	CountLargeLabel = "count_large"

	// FlowFanout*Label are the per-flow counts on a probe.tcp.flow_fanout sample:
	// the total flows attempted, the flows bad this cycle AND the previous one
	// (the deterministic / reproducible set), the flows bad this cycle but clean
	// last cycle (flapping), and the flows clean in both.
	FlowFanoutFlowsLabel    = "flows"
	FlowFanoutBadStableLabel = "bad_stable"
	FlowFanoutBadNewLabel    = "bad_new"
	FlowFanoutOKLabel        = "ok"
)

// MetricAllowedForProbeKind reports whether a metric kind can be produced by a
// monitoring target of the given probe kind. Gateway pings emit through the
// shared probe.icmp.* set; a host anchor carries the host.* / iface.up / wifi.* /
// agent.* series instead of a probe family.
//
// This is the single source of truth for the (probe kind → metric family)
// relation: the server validates alert conditions against it, drops the
// conditions a target's kind can no longer satisfy when that kind changes, and
// filters a monitor's listed series by it. An unknown kind allows nothing.
func MetricAllowedForProbeKind(probeKind, metricKind string) bool {
	switch probeKind {
	case "icmp", "gateway":
		return strings.HasPrefix(metricKind, "probe.icmp.")
	case "dns":
		return strings.HasPrefix(metricKind, "probe.dns.")
	case "http":
		return strings.HasPrefix(metricKind, "probe.http.")
	case "tcp":
		return strings.HasPrefix(metricKind, "probe.tcp.")
	case "nat":
		return strings.HasPrefix(metricKind, "probe.nat.")
	case "host":
		return strings.HasPrefix(metricKind, "host.") || metricKind == string(IfaceUp) ||
			strings.HasPrefix(metricKind, "wifi.") || strings.HasPrefix(metricKind, "agent.")
	}
	return false
}

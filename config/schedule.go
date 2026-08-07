package config

import "time"

// Shared probe-schedule math. These constants and helpers are the single source
// of truth for a target's effective check interval, whole-cycle deadline, and
// staleness window, consumed BOTH by the agent collectors (so a probe's real
// cadence and per-cycle timeout come from here) and by the server's freshness
// derivation (so a server-side stale window can never drift from what the agent
// actually runs). Every default below mirrors the historical per-collector
// behavior exactly; changing one changes both sides at once.

// Default per-target check intervals: the schedState fallback a collector uses
// when a target sets no IntervalSeconds. icmp/gateway run on the fast base tier,
// dns/http/tcp on the regular tier, and NAT rarely (a full discovery is several
// round-trips and its result changes seldom).
const (
	DefaultICMPInterval    = 10 * time.Second
	DefaultGatewayInterval = 10 * time.Second
	DefaultDNSInterval     = 30 * time.Second
	DefaultHTTPInterval    = 30 * time.Second
	DefaultTCPInterval     = 30 * time.Second
	DefaultNATInterval     = 30 * time.Minute
)

// ICMP cycle shape. One cycle sends DefaultPingCount echoes by default, each
// bounded by DefaultPingEchoTimeout.
//
// The echoes are NOT a burst: after every echo the collector recomputes the gap
// to the next one as "time left until this target is due again, divided by the
// echoes still to send", so a healthy cycle spreads its packets evenly across
// the whole check interval (5 packets over a 10s interval → one roughly every
// 2.25s) and the loss/jitter it reports describe the entire interval rather than
// a ~1s window at the top of it.
//
// A lost or errored echo instead sends the next one immediately. That fail-fast
// is what keeps spreading from costing alert latency: a target that is fully
// down finishes its cycle — and therefore its 100%-loss round — in
// count×perEcho (5×1s by default), no slower than the old fixed-spacing burst.
// So the two bounds below are the two shapes a cycle can take, and
// CycleDeadline is their max.
const (
	DefaultPingCount       = 5
	DefaultPingEchoTimeout = time.Second
)

// Per-probe default timeouts (used when a target sets no TimeoutMs / global).
const (
	DefaultDNSTimeout           = 3 * time.Second
	DefaultTCPTimeout           = 5 * time.Second
	DefaultHTTPTimeout          = 10 * time.Second
	DefaultNATPerRequestTimeout = 3 * time.Second
	// DefaultNATCycleDeadline bounds one whole NAT discovery (binding + filtering
	// + mapping with a couple of retries) when a target sets no GlobalTimeoutMs.
	DefaultNATCycleDeadline = 25 * time.Second
)

// EffectiveInterval is a target's per-check interval before the agent-local
// MinProbeInterval floor: the configured IntervalSeconds when > 0, else the
// collector default for the kind. It does NOT apply the MinProbeInterval floor —
// callers that need the reported effective schedule apply that floor themselves.
func EffectiveInterval(kind string, p ProbeParams) time.Duration {
	if p.IntervalSeconds > 0 {
		return time.Duration(p.IntervalSeconds) * time.Second
	}
	switch kind {
	case "dns":
		return DefaultDNSInterval
	case "http":
		return DefaultHTTPInterval
	case "tcp":
		return DefaultTCPInterval
	case "nat":
		return DefaultNATInterval
	case "gateway":
		return DefaultGatewayInterval
	default: // icmp (and any unknown kind falls back to the base tier)
		return DefaultICMPInterval
	}
}

// PingCount is the number of echoes one ICMP cycle sends for these params:
// PacketCount when set (>0), else the default burst. It is shared by the ICMP
// collectors and CycleDeadline so the per-cycle timing and the reported deadline
// can never disagree.
func PingCount(p ProbeParams) int {
	if p.PacketCount > 0 {
		return p.PacketCount
	}
	return DefaultPingCount
}

// PingEchoTimeout is the per-echo timeout for an ICMP cycle: the configured
// TimeoutMs when > 0, else the default.
func PingEchoTimeout(p ProbeParams) time.Duration {
	if p.TimeoutMs > 0 {
		return time.Duration(p.TimeoutMs) * time.Millisecond
	}
	return DefaultPingEchoTimeout
}

// CycleDeadline is the worst-case wall-clock a single probe cycle can take for a
// target, matching each collector's own bounding:
//   - icmp/gateway: GlobalTimeoutMs when set, else max(interval, count×perEcho).
//   - dns/tcp/http: TimeoutMs when set, else the per-kind default.
//   - nat: GlobalTimeoutMs when set, else the default whole-cycle deadline.
//
// The icmp/gateway max is the two cycle shapes described above. A healthy cycle
// spreads its echoes across the interval, and paces them so that every echo
// still to come fits its full per-echo timeout before the interval ends — so it
// completes by the next due instant, bounded by the interval. A failing cycle
// fail-fasts to back-to-back echoes — bounded by count×perEcho. Neither bound
// implies the other (a 1s interval with 5 packets is count-bound; a 5-minute
// interval is interval-bound), so the deadline is whichever is larger. When the
// count bound wins the echoes cannot fit the interval at all, and the pacing
// degenerates to back-to-back, which is that same bound.
//
// Note for StaleAfter callers: whenever the interval branch wins, cycle ≤
// interval, so StaleAfter's base collapses to 3×interval — the same window the
// old fixed-spacing burst produced. Spreading a cycle out therefore does not
// widen any freshness window on its own.
func CycleDeadline(kind string, p ProbeParams) time.Duration {
	switch kind {
	case "icmp", "gateway":
		if p.GlobalTimeoutMs > 0 {
			return time.Duration(p.GlobalTimeoutMs) * time.Millisecond
		}
		cycle := time.Duration(PingCount(p)) * PingEchoTimeout(p)
		if iv := EffectiveInterval(kind, p); iv > cycle {
			return iv
		}
		return cycle
	case "dns":
		if p.TimeoutMs > 0 {
			return time.Duration(p.TimeoutMs) * time.Millisecond
		}
		return DefaultDNSTimeout
	case "tcp":
		if p.TimeoutMs > 0 {
			return time.Duration(p.TimeoutMs) * time.Millisecond
		}
		return DefaultTCPTimeout
	case "http":
		if p.TimeoutMs > 0 {
			return time.Duration(p.TimeoutMs) * time.Millisecond
		}
		return DefaultHTTPTimeout
	case "nat":
		if p.GlobalTimeoutMs > 0 {
			return time.Duration(p.GlobalTimeoutMs) * time.Millisecond
		}
		return DefaultNATCycleDeadline
	default:
		return 0
	}
}

// DefaultUploadInterval is the agent's default WAL batch-upload cadence (mirrors
// the agent envcfg default). The server uses it as the StaleAfter fallback when
// an agent has not (yet) reported its own upload interval, so a freshly-connected
// or pre-generation agent still gets link-latency slack in its freshness window.
//
// 30s rather than a few seconds because every upload is one server-side write
// transaction, and in SQLite each commit rewrites every touched 4 KiB page into
// the WAL (and again at checkpoint). At the design scale — 50 agents × 20
// monitors — a 5s cadence puts the server past 100 GB/day of block writes for
// megabytes of actual data; 30s cuts the commit count 6× and batches several
// probe rounds per series into each page rewrite. Latency-sensitive paths do
// not ride on this: game seconds drain on their own capped interval, and fault
// confirmation only ever waits out at most one upload of backlogged rounds.
const DefaultUploadInterval = 30 * time.Second

// StaleAfter is the freshness window after which a sample is considered stale:
// max(3×interval, interval + 2×cycle) + 2×upload.
//
// The sample timestamp is the probe instant, but a sample does not reach the
// server store at that instant: the agent buffers results in its WAL and flushes
// them on a batch-upload cadence (upload), then the server drains and ingests
// them. The freshness window must tolerate this whole probe→arrival link, or a
// short-interval target (10s → a ~30s window) is marked stale on nothing more
// than ordinary batching jitter. The base term is the probe cadence itself:
// max(3×interval, interval + 2×cycle) tolerates a couple of missed cycles while
// keeping a legitimately long cycle (the 25s NAT discovery, an ICMP target given
// a GlobalTimeoutMs longer than its interval) from tripping stale mid-run. The
// +2×upload term adds the link slack: 2× (not 1×) covers a sample that just
// missed one batch boundary plus one upload retry.
//
// upload ≤ 0 falls back to DefaultUploadInterval (the server's value when the
// agent has not reported its own).
func StaleAfter(interval, cycle, upload time.Duration) time.Duration {
	if upload <= 0 {
		upload = DefaultUploadInterval
	}
	base := 3 * interval
	if b := interval + 2*cycle; b > base {
		base = b
	}
	return base + 2*upload
}

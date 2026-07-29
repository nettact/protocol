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

// ICMP cycle shape. One cycle sends DefaultPingCount echoes by default, spaced
// by PingSpacing, each bounded by DefaultPingEchoTimeout, so a fully-lost default
// cycle (5×1s + 4×200ms ≈ 5.8s) stays under the 10s interval.
const (
	PingSpacing            = 200 * time.Millisecond
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
//   - icmp/gateway: GlobalTimeoutMs when set, else count×perEcho + (count−1)×spacing.
//   - dns/tcp/http: TimeoutMs when set, else the per-kind default.
//   - nat: GlobalTimeoutMs when set, else the default whole-cycle deadline.
func CycleDeadline(kind string, p ProbeParams) time.Duration {
	switch kind {
	case "icmp", "gateway":
		if p.GlobalTimeoutMs > 0 {
			return time.Duration(p.GlobalTimeoutMs) * time.Millisecond
		}
		count := PingCount(p)
		return time.Duration(count)*PingEchoTimeout(p) + time.Duration(count-1)*PingSpacing
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
const DefaultUploadInterval = 5 * time.Second

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
// keeping a legitimately long cycle (multi-echo ICMP, the 25s NAT discovery)
// from tripping stale mid-run. The +2×upload term adds the link slack: 2× (not
// 1×) covers a sample that just missed one batch boundary plus one upload retry.
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

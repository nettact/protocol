// Package permission is the typed, immutable local-permission model shared by
// the agent (enforcement), the server (ingestion, pre-check, remediation), and
// the desktop trust-boundary exception. It replaces the old capability package:
// instead of a flat advertised list, an agent reports three views — supported
// (what the build+platform can do), granted (the local policy), and effective
// (the usable intersection) — plus a policy source and hash.
//
// The package depends only on the stdlib plus protocol/config (a leaf DTO
// package) so both protocol/wire and protocol/enroll can carry the shared
// PermissionReport without importing each other.
package permission

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nettact/protocol/config"
)

// ID is a permission identifier. Wildcards ("*", "all") are never valid values.
type ID string

// Active probe permissions.
const (
	ProbeICMP           ID = "probe.icmp"
	ProbeDNS            ID = "probe.dns"
	ProbeHTTP           ID = "probe.http"
	ProbeHTTPExtended   ID = "probe.http.extended"
	ProbeTCP            ID = "probe.tcp"
	ProbeNAT            ID = "probe.nat"
	NetworkGatewayProbe ID = "network.gateway.probe"
)

// Local network data permissions.
const (
	NetIfaceStatusRead  ID = "network.interface.status.read"
	NetIfaceAddressRead ID = "network.interface.address.read"
	NetWiFiStatusRead   ID = "network.wifi.status.read"
	NetWiFiSSIDRead     ID = "network.wifi.ssid.read"
	NetNeighborRead     ID = "network.neighbor.read"
	NetNeighborHostRead ID = "network.neighbor.hostname.read"
)

// Host metric permissions.
const (
	HostCPURead       ID = "host.cpu.read"
	HostMemoryRead    ID = "host.memory.read"
	HostDiskRead      ID = "host.disk.read"
	HostLoadRead      ID = "host.load.read"
	HostUptimeRead    ID = "host.uptime.read"
	HostNetworkIORead ID = "host.network.io.read"
	// HostTemperatureRead is capability-probed rather than assumed: many boards
	// and VMs expose no thermal sensors at all, so a platform advertises it only
	// when it has a trustworthy backend and a startup sensor read returns a real
	// value. Windows deliberately has no backend until ACPI WMI is replaced.
	HostTemperatureRead ID = "host.temperature.read"
)

// Process snapshot scope permissions.
const (
	HostProcessBasicRead    ID = "host.process.basic.read"
	HostProcessOwnerRead    ID = "host.process.owner.read"
	HostProcessResourceRead ID = "host.process.resource.read"
	HostProcessIORead       ID = "host.process.io.read"
)

// Connection snapshot scope permissions.
const (
	HostConnectionSummaryRead ID = "host.connection.summary.read"
	HostConnectionLocalRead   ID = "host.connection.local.read"
	HostConnectionRemoteRead  ID = "host.connection.remote.read"
	HostConnectionOwnerRead   ID = "host.connection.owner.read"
)

// Diagnostic traceroute permissions. Each mode is gated independently so a
// platform that can send ICMP echoes but lacks raw-socket TCP (or vice versa)
// reports only the mode it can actually run in its supported/effective views.
const (
	DiagnosticTracerouteICMP ID = "diagnostic.traceroute.icmp"
	DiagnosticTracerouteTCP  ID = "diagnostic.traceroute.tcp"
)

// Game experience permissions. Both are capability-probed like
// HostTemperatureRead: frame presentation data comes from a separate signed
// sensor component that the agent only detects — never implements — so the
// supported view holds them only when that component is installed beside the
// agent executable AND can actually open a trace session. "Component missing",
// "component present but blocked", and "working" are therefore three
// distinguishable states, not one silent unsupported.
//
// GameGPURead is gated apart from the frame data because it is a different
// read: frame timings come from the game's own presentation, while GPU and VRAM
// telemetry describes the adapter and every process sharing it. A machine can
// support one and not the other — plenty of drivers publish no adapter
// telemetry at all — so the sensor probes them separately.
const (
	GameProcessDetect   ID = "game.process.detect"
	GamePerformanceRead ID = "game.performance.read"
	GameGPURead         ID = "game.gpu.read"
)

// canonicalOrder is the compiled registry in canonical (stable) order. Set.Sorted
// and Strings emit IDs in this order; unknown IDs sort after all known ones. It
// is the single source of truth for All().
var canonicalOrder = []ID{
	ProbeICMP, ProbeDNS, ProbeHTTP, ProbeHTTPExtended, ProbeTCP, ProbeNAT,
	NetworkGatewayProbe,
	NetIfaceStatusRead, NetIfaceAddressRead,
	NetWiFiStatusRead, NetWiFiSSIDRead,
	NetNeighborRead, NetNeighborHostRead,
	HostCPURead, HostMemoryRead, HostDiskRead, HostLoadRead, HostUptimeRead, HostNetworkIORead,
	HostTemperatureRead,
	HostProcessBasicRead, HostProcessOwnerRead, HostProcessResourceRead, HostProcessIORead,
	HostConnectionSummaryRead, HostConnectionLocalRead, HostConnectionRemoteRead, HostConnectionOwnerRead,
	DiagnosticTracerouteICMP, DiagnosticTracerouteTCP,
	GameProcessDetect, GamePerformanceRead, GameGPURead,
}

// orderIndex maps a known ID to its canonical rank for sorting.
var orderIndex = func() map[ID]int {
	m := make(map[ID]int, len(canonicalOrder))
	for i, id := range canonicalOrder {
		m[id] = i
	}
	return m
}()

// deps maps a child permission to its direct required parents (spec §3.3).
var deps = map[ID][]ID{
	ProbeHTTPExtended:   {ProbeHTTP},
	NetIfaceAddressRead: {NetIfaceStatusRead},
	NetWiFiStatusRead:   {NetIfaceStatusRead},
	NetWiFiSSIDRead:     {NetWiFiStatusRead},
	NetNeighborHostRead: {NetNeighborRead},

	HostProcessOwnerRead:    {HostProcessBasicRead},
	HostProcessResourceRead: {HostProcessBasicRead},
	HostProcessIORead:       {HostProcessBasicRead},

	HostConnectionLocalRead:  {HostConnectionSummaryRead},
	HostConnectionRemoteRead: {HostConnectionSummaryRead},
	HostConnectionOwnerRead:  {HostConnectionSummaryRead},

	// Reading a game's frame timings means knowing which process is presenting,
	// so performance reads cannot be granted without process detection.
	GamePerformanceRead: {GameProcessDetect},
	// GPU and VRAM telemetry is collected on the frame-capture tick, for the
	// process being tracked, and lands in the same per-second bucket. There is no
	// path that produces it without the frame capture underneath, so granting it
	// alone would name a collection that cannot happen.
	GameGPURead: {GamePerformanceRead},
}

// known reports whether id is a compiled permission.
func known(id ID) bool {
	_, ok := orderIndex[id]
	return ok
}

// Dependencies returns the direct required parents of id (never transitive).
func Dependencies(id ID) []ID {
	return append([]ID(nil), deps[id]...)
}

// Set is a permission set. The zero value (nil map) is a valid empty set.
type Set map[ID]struct{}

// NewSet builds a Set from the given IDs.
func NewSet(ids ...ID) Set {
	s := make(Set, len(ids))
	for _, id := range ids {
		s[id] = struct{}{}
	}
	return s
}

// Has reports membership.
func (s Set) Has(id ID) bool {
	_, ok := s[id]
	return ok
}

// Add inserts id.
func (s Set) Add(id ID) { s[id] = struct{}{} }

// Clone returns an independent copy.
func (s Set) Clone() Set {
	out := make(Set, len(s))
	for id := range s {
		out[id] = struct{}{}
	}
	return out
}

// Sorted returns the IDs in canonical order; unknown IDs follow, lexicographic.
func (s Set) Sorted() []ID {
	out := make([]ID, 0, len(s))
	for id := range s {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		ri, oki := orderIndex[out[i]]
		rj, okj := orderIndex[out[j]]
		switch {
		case oki && okj:
			return ri < rj
		case oki != okj:
			return oki // known IDs sort before unknown
		default:
			return out[i] < out[j]
		}
	})
	return out
}

// Strings returns the sorted IDs as a []string (wire/JSON form).
func (s Set) Strings() []string {
	ids := s.Sorted()
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

// FromStrings builds a Set from wire strings. It is lenient — unknown IDs are
// kept opaque so server-side ingestion of an agent's report never drops values
// this build does not recognize. Validate is the gate that rejects unknowns.
func FromStrings(ss []string) Set {
	s := make(Set, len(ss))
	for _, v := range ss {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		s[ID(v)] = struct{}{}
	}
	return s
}

// Closure returns ids plus every transitive dependency. Used for remediation so
// a suggested NETTACT_AGENT_PERMISSIONS value is self-consistent.
func Closure(ids Set) Set {
	out := ids.Clone()
	if out == nil {
		out = Set{}
	}
	changed := true
	for changed {
		changed = false
		for id := range out {
			for _, parent := range deps[id] {
				if !out.Has(parent) {
					out.Add(parent)
					changed = true
				}
			}
		}
	}
	return out
}

// Validate reports unknown IDs and unsatisfied dependencies. It does NOT
// auto-add dependencies — a missing dependency is an error (spec: fail fast).
// Every violation is enumerated via errors.Join.
func Validate(s Set) error {
	var errs []error
	for _, id := range s.Sorted() {
		if !known(id) {
			errs = append(errs, fmt.Errorf("unknown permission %q", id))
			continue
		}
		for _, parent := range deps[id] {
			if !s.Has(parent) {
				errs = append(errs, fmt.Errorf("permission %q requires %q", id, parent))
			}
		}
	}
	return errors.Join(errs...)
}

// All returns the full compiled registry.
func All() Set {
	return NewSet(canonicalOrder...)
}

// DefaultStandalone is the frozen safety baseline enabled when no permission
// variable is set (spec §3.2). It is a literal list, NOT derived from All(): a
// future permission is off until a product decision adds it here.
func DefaultStandalone() Set {
	return NewSet(
		ProbeICMP,
		ProbeDNS,
		ProbeHTTP,
		ProbeTCP,
		ProbeNAT,
		NetworkGatewayProbe,
		NetIfaceStatusRead,
		NetIfaceAddressRead,
		NetWiFiStatusRead,
		DiagnosticTracerouteICMP,
		DiagnosticTracerouteTCP,
	)
}

// Bundle is a named permission set the console offers when an operator enrolls
// an Agent, so the common choices do not have to be assembled permission by
// permission.
type Bundle struct {
	ID  string
	Set Set
}

// Bundles are the enrollment presets, in the order a chooser should show them:
// from the safety baseline to everything. They live here beside
// DefaultStandalone because which permissions belong together is a product
// decision about the permission model, not a UI detail — the console must not be
// the place that decides what "recommended" means.
//
// Every bundle is dependency-closed, so any of them is directly usable as a
// NETTACT_AGENT_PERMISSIONS value.
func Bundles() []Bundle {
	return []Bundle{
		// The frozen safety baseline: standard probes plus basic network state.
		{ID: "recommended", Set: Closure(DefaultStandalone())},
		// The baseline plus host resource metrics — the usual next step, and the
		// reason most operators end up editing a policy at all. Process and
		// connection snapshots are deliberately NOT here: they read command lines,
		// owning users and remote addresses, which is a different privacy decision
		// from "how busy is this machine".
		{ID: "host_metrics", Set: Closure(union(DefaultStandalone(), NewSet(
			HostCPURead, HostMemoryRead, HostDiskRead,
			HostLoadRead, HostUptimeRead, HostNetworkIORead,
			HostTemperatureRead,
		)))},
		// Every compiled permission. Capability gating still applies, so this
		// grants only what the Agent's platform can actually do.
		{ID: "full", Set: All()},
	}
}

// union returns a new set containing every ID in a and b.
func union(a, b Set) Set {
	out := a.Clone()
	for id := range b {
		out.Add(id)
	}
	return out
}

// EffectiveFrom is the intersection of granted and supported, then a fixpoint
// prune of any child whose required parent dropped out of the intersection.
func EffectiveFrom(granted, supported Set) Set {
	out := Set{}
	for id := range granted {
		if supported.Has(id) {
			out.Add(id)
		}
	}
	changed := true
	for changed {
		changed = false
		for id := range out {
			for _, parent := range deps[id] {
				if !out.Has(parent) {
					delete(out, id)
					changed = true
				}
			}
		}
	}
	return out
}

// Source identifies where a policy's granted set came from.
type Source string

const (
	SourceDefault           Source = "default"
	SourceEnvironment       Source = "environment"
	SourceDesktopFullAccess Source = "desktop_full_access"

	// SourceServerConfig marks a grant configured per server rather than per
	// process: one agent may report a different granted set to each server it
	// connects to (the machine owner deciding "this server may collect host
	// metrics, that one only basic probes"). It says nothing about which server
	// — the report reaching a server IS that server's grant.
	SourceServerConfig Source = "server_config"
)

// Policy is one connection's immutable local permission grant. It is per server,
// not per process: an agent talking to N servers holds N policies and reports
// each one only to its own server.
type Policy struct {
	Granted    Set
	Source     Source
	FullAccess bool // desktop: also bypasses probe access policy
}

// FullAccess returns the desktop trust-boundary policy: every compiled
// permission, probe access bypassed.
func FullAccess() Policy {
	return Policy{Granted: All(), Source: SourceDesktopFullAccess, FullAccess: true}
}

// PermissionReport is the shared DTO carried by both wire.Hello and
// enroll.EnrollRequest. Supported/Granted/Effective are wire strings in
// canonical order.
type PermissionReport struct {
	Supported  []string `json:"supported"`
	Granted    []string `json:"granted"`
	Effective  []string `json:"effective"`
	Source     string   `json:"source"`
	PolicyHash string   `json:"policy_hash"`
	// UnsupportedReasons explains, per permission id, why a capability probe
	// concluded the permission is not supported here. Without it the three sets
	// say only "supported: false", which leaves a console guessing the single
	// remediation it happens to know — and a guess sends people to install
	// software they already have when the real cause was something else.
	//
	// Keyed by permission ID, and it only ever holds ids ABSENT from Supported: a
	// supported permission has nothing to explain.
	//
	// An absent key is not "no reason" — it means the question was never asked.
	// The probe did not run, typically because nothing granted the capability and
	// an agent refuses to probe what it was not granted. A reader must render a
	// missing entry as unprobed, never as an unexplained failure.
	//
	// The value vocabulary is owned by whatever probes the capability, not by this
	// package: game capture uses the codes in protocol/gamesense, and a future
	// temperature probe would bring its own. This package only transports them,
	// which is why the values are plain strings and are never validated here.
	// Readers MUST tolerate codes they do not know — a newer agent reporting to an
	// older console is ordinary, not a fault — and fall back to their own generic
	// text rather than putting a raw identifier in front of a user.
	//
	// Full-state on every report, like the three sets beside it: it describes the
	// last probe and carries no history.
	UnsupportedReasons map[string]string `json:"unsupported_reasons,omitempty"`
}

// RequiredForTarget returns the permissions a probe monitor needs. HTTP with a
// non-basic method/body/header additionally requires probe.http.extended.
func RequiredForTarget(t config.ProbeTarget) []ID {
	switch t.Kind {
	case "icmp":
		return []ID{ProbeICMP}
	case "dns":
		return []ID{ProbeDNS}
	case "http":
		req := []ID{ProbeHTTP}
		if httpNeedsExtended(t.Params) {
			req = append(req, ProbeHTTPExtended)
		}
		return req
	case "tcp":
		return []ID{ProbeTCP}
	case "nat":
		return []ID{ProbeNAT}
	case "gateway":
		return []ID{NetworkGatewayProbe}
	default:
		return nil
	}
}

// httpNeedsExtended reports whether these HTTP params exceed the basic profile
// (GET/HEAD only, no body, only allowlisted headers). Mirrors the agent's
// request-build gate so the server pre-check never drifts.
func httpNeedsExtended(p config.ProbeParams) bool {
	switch strings.ToUpper(strings.TrimSpace(p.Method)) {
	case "", "GET", "HEAD":
		// basic method
	default:
		return true
	}
	if p.Body != "" {
		return true
	}
	for name := range p.Headers {
		if !BasicHTTPHeaderAllowed(name) {
			return true
		}
	}
	return false
}

// basicHTTPHeaders is the case-insensitive allowlist a basic probe.http may send.
var basicHTTPHeaders = map[string]struct{}{
	"user-agent":      {},
	"accept":          {},
	"accept-language": {},
	"cache-control":   {},
}

// BasicHTTPHeaderAllowed reports whether a basic probe.http may send this header.
// Everything else (Host override, Authorization, Cookie, Content-*, X-*, …)
// requires probe.http.extended.
func BasicHTTPHeaderAllowed(name string) bool {
	_, ok := basicHTTPHeaders[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// RequiredForHostMetric maps a telemetry metric kind to the host/network
// permission that gates it (spec §3.2). wifi.* and iface.up map through the
// interface/wifi families; Closure adds the interface-status dependency.
// Returns nil for kinds not gated by a host permission (agent.*, probe.*).
func RequiredForHostMetric(metricKind string) []ID {
	switch {
	case strings.HasPrefix(metricKind, "host.cpu."):
		return []ID{HostCPURead}
	case strings.HasPrefix(metricKind, "host.mem."):
		return []ID{HostMemoryRead}
	case strings.HasPrefix(metricKind, "host.disk."):
		return []ID{HostDiskRead}
	case strings.HasPrefix(metricKind, "host.load."):
		return []ID{HostLoadRead}
	case metricKind == "host.uptime_s":
		return []ID{HostUptimeRead}
	case strings.HasPrefix(metricKind, "host.net."):
		return []ID{HostNetworkIORead}
	case strings.HasPrefix(metricKind, "host.temp."):
		return []ID{HostTemperatureRead}
	case strings.HasPrefix(metricKind, "wifi."):
		return []ID{NetWiFiStatusRead}
	case metricKind == "iface.up":
		return []ID{NetIfaceStatusRead}
	default:
		return nil
	}
}

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
	HostProcessBasicRead, HostProcessOwnerRead, HostProcessResourceRead, HostProcessIORead,
	HostConnectionSummaryRead, HostConnectionLocalRead, HostConnectionRemoteRead, HostConnectionOwnerRead,
	DiagnosticTracerouteICMP, DiagnosticTracerouteTCP,
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
)

// Policy is one process's immutable local permission grant.
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
	case strings.HasPrefix(metricKind, "wifi."):
		return []ID{NetWiFiStatusRead}
	case metricKind == "iface.up":
		return []ID{NetIfaceStatusRead}
	default:
		return nil
	}
}

package permission

import (
	"reflect"
	"testing"

	"github.com/nettact/protocol/config"
)

func TestDefaultStandaloneIsFrozenBaseline(t *testing.T) {
	want := []string{
		"probe.icmp",
		"probe.dns",
		"probe.http",
		"probe.tcp",
		"probe.nat",
		"network.gateway.probe",
		"network.interface.status.read",
		"network.interface.address.read",
		"network.wifi.status.read",
		"diagnostic.traceroute.icmp",
		"diagnostic.traceroute.tcp",
	}
	if got := DefaultStandalone().Strings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultStandalone() = %v, want exactly %v", got, want)
	}

	for _, sensitive := range []ID{
		NetWiFiSSIDRead, NetNeighborRead, HostCPURead,
		HostProcessBasicRead, HostConnectionSummaryRead,
	} {
		if DefaultStandalone().Has(sensitive) {
			t.Errorf("default unexpectedly grants sensitive permission %q", sensitive)
		}
	}
}

func TestEffectiveFromPrunesUnsupportedDependencyChain(t *testing.T) {
	granted := NewSet(NetIfaceStatusRead, NetWiFiStatusRead, NetWiFiSSIDRead)
	// The build claims the two Wi-Fi children but not their interface-status
	// dependency. Neither child is usable in the effective set.
	supported := NewSet(NetWiFiStatusRead, NetWiFiSSIDRead)
	if got := EffectiveFrom(granted, supported); len(got) != 0 {
		t.Fatalf("EffectiveFrom() = %v, want empty after dependency pruning", got.Strings())
	}
}

func TestRequiredForTargetSeparatesBasicAndExtendedHTTP(t *testing.T) {
	tests := []struct {
		name   string
		params config.ProbeParams
		want   []ID
	}{
		{name: "basic get", params: config.ProbeParams{Method: "GET"}, want: []ID{ProbeHTTP}},
		{name: "get body", params: config.ProbeParams{Method: "GET", Body: "payload"}, want: []ID{ProbeHTTP, ProbeHTTPExtended}},
		{name: "post", params: config.ProbeParams{Method: "POST"}, want: []ID{ProbeHTTP, ProbeHTTPExtended}},
		{name: "restricted header", params: config.ProbeParams{Headers: map[string]string{"Authorization": "secret"}}, want: []ID{ProbeHTTP, ProbeHTTPExtended}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RequiredForTarget(config.ProbeTarget{Kind: "http", Params: tt.params})
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("RequiredForTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBundlesAreUsablePolicies: every enrollment preset is handed to an operator
// as a NETTACT_AGENT_PERMISSIONS value, so each one must be a policy the Agent
// will actually accept at startup — known IDs, dependencies satisfied.
func TestBundlesAreUsablePolicies(t *testing.T) {
	bundles := Bundles()
	if len(bundles) == 0 {
		t.Fatal("Bundles() must offer at least one preset")
	}
	seen := map[string]bool{}
	for _, b := range bundles {
		if seen[b.ID] {
			t.Fatalf("duplicate bundle id %q", b.ID)
		}
		seen[b.ID] = true
		if len(b.Set) == 0 {
			t.Fatalf("bundle %q is empty", b.ID)
		}
		if err := Validate(b.Set); err != nil {
			t.Fatalf("bundle %q is not a usable policy: %v", b.ID, err)
		}
	}
	for _, id := range []string{"recommended", "host_metrics", "full"} {
		if !seen[id] {
			t.Fatalf("bundle %q missing from %v", id, seen)
		}
	}
}

// TestBundlesAreOrderedBySize pins the chooser's progression: recommended ⊂
// host_metrics ⊂ full. A preset that is not a superset of the one before it would
// make the list a set of unrelated options rather than escalating levels.
func TestBundlesAreOrderedBySize(t *testing.T) {
	bundles := Bundles()
	for i := 1; i < len(bundles); i++ {
		prev, cur := bundles[i-1], bundles[i]
		for id := range prev.Set {
			if !cur.Set.Has(id) {
				t.Fatalf("bundle %q drops %q from %q; presets must escalate", cur.ID, id, prev.ID)
			}
		}
		if len(cur.Set) <= len(prev.Set) {
			t.Fatalf("bundle %q (%d) must be larger than %q (%d)", cur.ID, len(cur.Set), prev.ID, len(prev.Set))
		}
	}
}

// TestFullBundleCoversEveryCompiledPermission: "full" is what an operator picks
// to stop thinking about permissions, so a permission added to the registry
// without landing here would silently stay off.
func TestFullBundleCoversEveryCompiledPermission(t *testing.T) {
	var full Set
	for _, b := range Bundles() {
		if b.ID == "full" {
			full = b.Set
		}
	}
	for _, id := range All().Sorted() {
		if !full.Has(id) {
			t.Fatalf("bundle \"full\" is missing %q", id)
		}
	}
}

// TestHostMetricsBundleExcludesSnapshotScopes: process and connection snapshots
// read command lines, owning users and remote addresses. Folding them into a
// bundle labelled "host metrics" would grant a different class of visibility than
// its name promises.
func TestHostMetricsBundleExcludesSnapshotScopes(t *testing.T) {
	var hm Set
	for _, b := range Bundles() {
		if b.ID == "host_metrics" {
			hm = b.Set
		}
	}
	if !hm.Has(HostCPURead) || !hm.Has(HostNetworkIORead) {
		t.Fatalf("host_metrics must carry the resource metrics, got %v", hm.Strings())
	}
	for _, id := range []ID{
		HostProcessBasicRead, HostProcessOwnerRead,
		HostConnectionSummaryRead, HostConnectionRemoteRead,
		NetNeighborRead,
	} {
		if hm.Has(id) {
			t.Fatalf("host_metrics must not include %q", id)
		}
	}
}

// TestRequiredForHostMetricGatesTemperature pins both temperature series to the
// temperature permission, and re-checks the neighbouring host kinds: the switch
// is prefix-based, so a careless "host.temp" case could shadow them.
func TestRequiredForHostMetricGatesTemperature(t *testing.T) {
	for kind, want := range map[string]ID{
		"host.temp.c":        HostTemperatureRead,
		"host.temp.sensor.c": HostTemperatureRead,
		"host.uptime_s":      HostUptimeRead,
		"host.net.rx_bps":    HostNetworkIORead,
		"host.cpu.pct":       HostCPURead,
	} {
		got := RequiredForHostMetric(kind)
		if len(got) != 1 || got[0] != want {
			t.Fatalf("RequiredForHostMetric(%q) = %v, want [%s]", kind, got, want)
		}
	}
	if got := RequiredForHostMetric("probe.icmp.rtt_ms"); got != nil {
		t.Fatalf("probe kinds are not host-gated, got %v", got)
	}
}

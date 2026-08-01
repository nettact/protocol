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

// TestGamePermissionsAreOptIn: frame data identifies which program a person is
// running, so it must never arrive by way of a bundle chosen for something else.
// Only "full" — an explicit everything — may carry it.
func TestGamePermissionsAreOptIn(t *testing.T) {
	for _, id := range []ID{GameProcessDetect, GamePerformanceRead, GameGPURead} {
		if DefaultStandalone().Has(id) {
			t.Errorf("default policy must not grant %q", id)
		}
		for _, b := range Bundles() {
			if b.ID == "full" {
				if !b.Set.Has(id) {
					t.Errorf("bundle %q must carry %q", b.ID, id)
				}
				continue
			}
			if b.Set.Has(id) {
				t.Errorf("bundle %q must not carry %q", b.ID, id)
			}
		}
	}
}

// TestGamePerformanceRequiresProcessDetect: reading a game's frame timings means
// first knowing which process is presenting. A build that can do the second but
// not the first must report neither as effective.
func TestGamePerformanceRequiresProcessDetect(t *testing.T) {
	if deps := Dependencies(GamePerformanceRead); len(deps) != 1 || deps[0] != GameProcessDetect {
		t.Fatalf("Dependencies(%q) = %v, want [%s]", GamePerformanceRead, deps, GameProcessDetect)
	}
	granted := NewSet(GameProcessDetect, GamePerformanceRead)
	// The sensor component is absent, so the platform supports neither.
	if got := EffectiveFrom(granted, Set{}); len(got) != 0 {
		t.Fatalf("EffectiveFrom() = %v, want empty when the sensor is absent", got.Strings())
	}
	// A build claiming the read without detection is pruned back to nothing.
	if got := EffectiveFrom(granted, NewSet(GamePerformanceRead)); len(got) != 0 {
		t.Fatalf("EffectiveFrom() = %v, want empty after dependency pruning", got.Strings())
	}
	if got := EffectiveFrom(granted, granted); len(got) != 2 {
		t.Fatalf("EffectiveFrom() = %v, want both when supported", got.Strings())
	}
}

// TestGameGPUReadSitsBelowThePerformanceRead: adapter telemetry is collected on
// the frame-capture tick and lands in the same per-second bucket, so there is no
// path that produces it without the frame capture underneath. The chain is three
// deep, which makes it the first place a single-step dependency walk would break
// — granting the GPU read on a build that detects processes but cannot read
// frames must leave nothing effective, not the two ends of a chain missing its
// middle.
func TestGameGPUReadSitsBelowThePerformanceRead(t *testing.T) {
	if deps := Dependencies(GameGPURead); len(deps) != 1 || deps[0] != GamePerformanceRead {
		t.Fatalf("Dependencies(%q) = %v, want [%s]", GameGPURead, deps, GamePerformanceRead)
	}
	// Closure must walk the whole chain, since the value it produces is handed to
	// an operator as a NETTACT_AGENT_PERMISSIONS line that has to validate.
	closed := Closure(NewSet(GameGPURead))
	for _, id := range []ID{GameGPURead, GamePerformanceRead, GameProcessDetect} {
		if !closed.Has(id) {
			t.Errorf("Closure(game.gpu.read) is missing %q: %v", id, closed.Strings())
		}
	}
	if err := Validate(closed); err != nil {
		t.Fatalf("closure of the GPU read is not a usable policy: %v", err)
	}
	// The middle of the chain missing prunes both ends, not just the child.
	granted := NewSet(GameProcessDetect, GamePerformanceRead, GameGPURead)
	if got := EffectiveFrom(granted, NewSet(GameProcessDetect, GameGPURead)); len(got) != 1 || !got.Has(GameProcessDetect) {
		t.Fatalf("EffectiveFrom() = %v, want only game.process.detect once the chain breaks", got.Strings())
	}
	if got := EffectiveFrom(granted, granted); len(got) != 3 {
		t.Fatalf("EffectiveFrom() = %v, want all three when supported", got.Strings())
	}
	// A machine that captures frames but whose driver publishes no adapter
	// telemetry is ordinary, and it keeps everything below the GPU read.
	noGPU := NewSet(GameProcessDetect, GamePerformanceRead)
	if got := EffectiveFrom(granted, noGPU); len(got) != 2 || got.Has(GameGPURead) {
		t.Fatalf("EffectiveFrom() = %v, want frame capture without the GPU read", got.Strings())
	}
}

// TestGamePermissionsFollowTheirCanonicalOrder: Sorted() drives the console's
// permission list and the policy hash, so the family must read parent-first
// rather than in whatever order a map yielded.
func TestGamePermissionsFollowTheirCanonicalOrder(t *testing.T) {
	want := []string{"game.process.detect", "game.performance.read", "game.gpu.read"}
	got := NewSet(GameGPURead, GamePerformanceRead, GameProcessDetect).Strings()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Sorted() = %v, want %v", got, want)
	}
}

// TestGameDataIsNotGatedThroughTheMetricPath pins the fact that game presentation
// data never travels as a metric: it has its own run/bucket model, and the server
// gates that on GamePerformanceRead where it is ingested. Anyone reintroducing a
// game.* metric kind would find it silently ungated, which is what this catches.
func TestGameDataIsNotGatedThroughTheMetricPath(t *testing.T) {
	for _, kind := range []string{"game.fps.current", "game.frame_time.avg_ms"} {
		if got := RequiredForHostMetric(kind); got != nil {
			t.Fatalf("RequiredForHostMetric(%q) = %v, want nil — game data is not a metric", kind, got)
		}
	}
}

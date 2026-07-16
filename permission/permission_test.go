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

package telemetry

import "testing"

func TestMetricAllowedForProbeKind(t *testing.T) {
	cases := []struct {
		probeKind  string
		metricKind string
		want       bool
	}{
		// Each probe kind accepts only its own family.
		{"icmp", string(ICMPLoss), true},
		{"icmp", string(DNSOK), false},
		{"dns", string(DNSOK), true},
		{"dns", string(HTTPOK), false},
		{"http", string(HTTPOK), true},
		{"http", string(HTTPErrorClass), true},
		{"http", string(DNSResolve), false},
		{"tcp", string(TCPConnectMs), true},
		{"tcp", string(HTTPStatus), false},
		{"nat", string(NATType), true},
		{"nat", string(ICMPRTTms), false},

		// Gateway pings ride the shared ICMP metric set.
		{"gateway", string(ICMPLoss), true},
		{"gateway", "probe.gateway.loss_pct", false},

		// A host anchor carries the system series instead of a probe family.
		{"host", string(HostCPUPct), true},
		{"host", string(IfaceUp), true},
		{"host", string(WiFiUp), true},
		{"host", string(AgentUptime), true},
		// What the machine renders is a property of the machine, like what it
		// measures of its own CPU — not something probed over the network.
		{"host", string(GameFPS), true},
		{"host", string(GameFrameTimeP95), true},
		{"host", string(HTTPOK), false},
		// …but only for the host anchor: a probe monitor never carries them.
		{"icmp", string(GameFPS), false},

		// An unknown kind allows nothing.
		{"", string(HTTPOK), false},
		{"bogus", string(HostCPUPct), false},
	}
	for _, c := range cases {
		if got := MetricAllowedForProbeKind(c.probeKind, c.metricKind); got != c.want {
			t.Errorf("MetricAllowedForProbeKind(%q, %q) = %v, want %v", c.probeKind, c.metricKind, got, c.want)
		}
	}
}

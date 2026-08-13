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
		{"http", string(HTTPFlowFanout), true},
		{"tcp", string(HTTPFlowFanout), false},
		{"dns", string(DNSOK), true},
		{"dns", string(HTTPOK), false},
		{"http", string(HTTPOK), true},
		{"http", string(HTTPTotalMs), true},
		{"http", string(HTTPTTFBMs), true},
		{"http", string(HTTPConnectMs), true},
		{"http", string(HTTPDNSMs), true},
		{"http", string(HTTPTLSMs), true},
		{"http", string(HTTPConnectionReused), true},
		{"http", string(HTTPErrorClass), true},
		{"http", string(DNSResolve), false},
		{"tcp", string(TCPConnectMs), true},
		{"tcp", string(HTTPStatus), false},
		{"nat", string(NATType), true},
		{"nat", string(ICMPRTTms), false},

		// Gateway pings ride the shared ICMP metric set.
		{"gateway", string(ICMPLoss), true},
		{"gateway", string(ICMPSent), true},
		{"icmp", string(ICMPSent), true},
		{"gateway", "probe.gateway.loss_pct", false},

		// A host anchor carries the system series instead of a probe family.
		{"host", string(HostCPUPct), true},
		{"host", string(IfaceUp), true},
		{"host", string(WiFiUp), true},
		{"host", string(AgentUptime), true},
		{"host", string(HTTPOK), false},
		// Game presentation data is not a metric family at all — it has its own
		// run/bucket model — so no probe kind carries it.
		{"host", "game.fps.current", false},

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

func TestHTTPMetricKindNames(t *testing.T) {
	cases := []struct {
		kind MetricKind
		want string
	}{
		{HTTPTotalMs, "probe.http.total_ms"},
		{HTTPTTFBMs, "probe.http.ttfb_ms"},
		{HTTPConnectMs, "probe.http.connect_ms"},
		{HTTPDNSMs, "probe.http.dns_ms"},
		{HTTPTLSMs, "probe.http.tls_ms"},
		{HTTPConnectionReused, "probe.http.connection_reused"},
	}

	for _, c := range cases {
		if got := string(c.kind); got != c.want {
			t.Errorf("HTTP metric kind = %q, want %q", got, c.want)
		}
	}
}

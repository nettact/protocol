package wire

import (
	"reflect"
	"testing"
	"time"

	"github.com/nettact/protocol/config"
	"github.com/nettact/protocol/permission"
	"github.com/nettact/protocol/telemetry"
)

// samplePacket exercises every field type: timestamps, maps, repeated fields,
// a false bool (Up), and int32/uint64/float64 values.
func samplePacket() telemetry.Packet {
	ts := time.Date(2026, 7, 11, 12, 30, 45, 0, time.UTC)
	return telemetry.Packet{
		SchemaVersion:         1,
		AgentID:               "agent-abc",
		SiteID:                "site-1",
		Sequence:              9001,
		SentAt:                ts,
		ReportedConfigVersion: 7,
		Metrics: []telemetry.Metric{
			// One monitor-stamped probe metric and one id-less system metric, so the
			// round-trip covers both monitor_id shapes.
			{TS: ts, Kind: telemetry.ICMPRTTms, Target: "1.1.1.1", Layer: telemetry.HealthLayer("internet"), Value: 12.5, Unit: telemetry.UnitMs, Labels: map[string]string{"iface": "eth0", "region": "us"}, MonitorID: "probe_mon1", ConfigSerial: 41},
			{TS: ts, Kind: telemetry.MetricKind("some.future.kind"), Value: 0, Unit: telemetry.UnitBool},
		},
		Events: []telemetry.Event{
			{ID: "evt-1", TS: ts, Type: telemetry.EventIfaceDown, Layer: telemetry.HealthLayer("local"), Severity: telemetry.SeverityWarn, Message: "iface down", Attrs: map[string]string{"iface": "eth0"}},
		},
		InventoryDelta: []telemetry.InventoryItem{
			{Kind: telemetry.InventoryDevice, Op: telemetry.OpUpsert, ID: "aa:bb:cc:dd:ee:ff", MAC: "aa:bb:cc:dd:ee:ff", IP: "10.0.0.5", Hostname: "printer", Vendor: "HP", LastSeen: ts},
		},
		InterfaceSnapshots: []telemetry.InterfaceSnapshot{
			{
				SampledAt:    ts,
				WiFiState:    telemetry.WiFiCollectionOK,
				DefaultRoute: &telemetry.SnapshotRoute{Gateway: "10.0.0.1", Interface: "eth0"},
				Interfaces: []telemetry.InterfaceState{
					{Name: "eth0", Addrs: []string{"10.0.0.2/24"}, Gateway: "10.0.0.1", DNS: []string{"1.1.1.1"}, Up: true},
					{Name: "wlan0", Addrs: []string{"192.168.1.2/24"}, Up: true, IsWireless: true, WiFi: &telemetry.WiFiInfo{
						State: telemetry.WiFiConnected, Reason: telemetry.WiFiReasonPermission,
						SSID: "home", Band: telemetry.WiFiBand5, Channel: 36,
					}},
				},
			},
			// Explicit empty is a semantic shape: protobuf decode must reconstruct
			// a non-nil empty slice so JSON remains `interfaces: []`, not null.
			{SampledAt: ts.Add(time.Second), WiFiState: telemetry.WiFiCollectionOK, Interfaces: []telemetry.InterfaceState{}},
		},
	}
}

func sampleHostSnapshot() telemetry.HostSnapshot {
	ts := time.Date(2026, 7, 11, 12, 30, 45, 0, time.UTC)
	total := 2
	user := "root"
	var cpu float64 = 1.5
	var rss uint64 = 1 << 20
	var virt uint64 = 1 << 21
	var dr uint64 = 100
	var dw uint64 = 200
	var rt float64 = 3600.5
	local := "10.0.0.2:80"
	remote := "1.2.3.4:5555"
	var pid int32 = 42
	pname := "nginx"
	return telemetry.HostSnapshot{
		TS: ts, RequestID: "req-1", ProcessTotal: &total,
		Scopes: []telemetry.SnapshotScopeResult{
			{Scope: "host.process.basic.read", Status: telemetry.ScopeCollected},
			{Scope: "host.process.owner.read", Status: telemetry.ScopeDenied, Reason: "unsatisfied_dependency"},
			{Scope: "host.connection.summary.read", Status: telemetry.ScopeCollected},
		},
		Processes: []telemetry.ProcessInfo{
			{PID: 42, Name: "nginx", Status: "Running", User: &user, CPUPct: &cpu, RSSBytes: &rss, VirtBytes: &virt, DiskReadBytes: &dr, DiskWriteBytes: &dw, RunTimeSeconds: &rt},
			// A basic-only row: every optional pointer stays nil (must survive round-trip as nil).
			{PID: 7, Name: "sshd", Status: "Sleeping"},
		},
		Connections: []telemetry.ConnectionInfo{
			{Proto: "tcp", State: "ESTABLISHED", LocalAddr: &local, RemoteAddr: &remote, PID: &pid, ProcessName: &pname},
			// A summary-only row: local/remote/owner pointers stay nil.
			{Proto: "udp", State: "NONE"},
		},
	}
}

func sampleDesiredState() config.DesiredState {
	return config.DesiredState{
		ConfigVersion: 8,
		ProbeTargets: []config.ProbeTarget{
			{MonitorID: "probe_mon1", Kind: "icmp", Name: "Cloudflare DNS", Target: "1.1.1.1", Params: config.ProbeParams{IntervalSeconds: 10, TimeoutMs: 1000, PacketSize: 56, PacketCount: 3, GlobalTimeoutMs: 10000}, ConfigSerial: 41},
			{Kind: "http", Name: "Example keyword", Target: "https://example.com", Params: config.ProbeParams{
				Method: "POST", AcceptedStatuses: "200-299,301",
				Keyword: "Example Domain", KeywordInvert: true, Headers: map[string]string{"X-Test": "1"},
				Body: `{"k":"v"}`, MaxRedirects: 5, IgnoreTLS: true, MaxResponseBytes: 2048,
			}},
			{Kind: "tcp", Name: "TLS port", Target: "1.1.1.1", Params: config.ProbeParams{Port: 443, TLS: true, TimeoutMs: 2000}, ProxyID: "prx_socks"},
			{Kind: "dns", Name: "MX lookup", Target: "example.com", Params: config.ProbeParams{RecordType: "MX", ResolverServer: "https://cloudflare-dns.com/dns-query", ResolverProtocol: "doh"}},
			{Kind: "nat", Name: "NAT type", Target: "stun.example.com", Params: config.ProbeParams{Port: 3478, NATTransport: "udp", STUNServer2: "stun2.example.com:3478", TimeoutMs: 3000}},
			{MonitorID: "probe_gw1", Kind: "gateway", Name: "LAN gateway", Target: "gateway", Params: config.ProbeParams{Interface: "以太网", PacketCount: 3, TimeoutMs: 2000}},
			{MonitorID: "probe_tun1", Kind: "icmp", Name: "Tunnelled ping", Target: "10.7.0.1", Params: config.ProbeParams{PacketCount: 3}, ProxyID: "prx_wg"},
		},
		Intervals: config.Intervals{BaseSeconds: 10, RegularSeconds: 60},
		// One spec per shape: a credentialed relay and a tunnel, so every ProxySpec
		// field is exercised by the round-trip (a dropped field would otherwise only
		// surface as a proxy that silently stops authenticating).
		Proxies: []config.ProxySpec{
			{
				ID: "prx_socks", Name: "Office SOCKS5", Type: config.ProxyTypeSOCKS5, ConfigSerial: 12,
				Host: "proxy.example.com", Port: 1080, Username: "probe", Password: "s3cret",
				DNSMode: config.ProxyDNSRemote, ConnectTimeoutMs: 3000,
			},
			{
				ID: "prx_wg", Name: "Site tunnel", Type: config.ProxyTypeWireGuard, ConfigSerial: 12,
				WGPrivateKey:    "aFBrZXlwa2V5cGtleXBrZXlwa2V5cGtleXBrZXk=",
				WGPeerPublicKey: "cHVia2V5cHVia2V5cHVia2V5cHVia2V5cHViaw==",
				WGPresharedKey:  "cHNrcHNrcHNrcHNrcHNrcHNrcHNrcHNrcHNrcHM=",
				WGEndpoint:      "wg.example.com:51820", WGAllowedIPs: "10.7.0.0/24,192.168.9.0/24",
				WGLocalAddrs: "10.7.0.2/32", WGDNS: "10.7.0.53", WGMTU: 1380, WGKeepaliveSeconds: 25,
			},
		},
	}
}

func sampleAck() Ack {
	return Ack{
		HighestSequence: 9001,
		ServerTime:      time.Date(2026, 7, 11, 12, 30, 46, 0, time.UTC),
	}
}

func TestPacketRoundTrip(t *testing.T) {
	in := samplePacket()
	for _, ct := range []string{ContentTypeJSON, ContentTypeProtobuf} {
		data, err := MarshalPacket(in, ct)
		if err != nil {
			t.Fatalf("MarshalPacket(%s): %v", ct, err)
		}
		out, err := UnmarshalPacket(data, ct)
		if err != nil {
			t.Fatalf("UnmarshalPacket(%s): %v", ct, err)
		}
		if !reflect.DeepEqual(in, out) {
			t.Errorf("%s round-trip mismatch:\n in=%+v\nout=%+v", ct, in, out)
		}
	}
}

func TestAckRoundTrip(t *testing.T) {
	in := sampleAck()
	for _, ct := range []string{ContentTypeJSON, ContentTypeProtobuf} {
		data, err := MarshalAck(in, ct)
		if err != nil {
			t.Fatalf("MarshalAck(%s): %v", ct, err)
		}
		out, err := UnmarshalAck(data, ct)
		if err != nil {
			t.Fatalf("UnmarshalAck(%s): %v", ct, err)
		}
		if !reflect.DeepEqual(in, out) {
			t.Errorf("%s round-trip mismatch:\n in=%+v\nout=%+v", ct, in, out)
		}
	}
}

// Empty packet must round-trip without spurious fields.
func TestEmptyRoundTrip(t *testing.T) {
	for _, ct := range []string{ContentTypeJSON, ContentTypeProtobuf} {
		data, err := MarshalPacket(telemetry.Packet{}, ct)
		if err != nil {
			t.Fatalf("MarshalPacket empty(%s): %v", ct, err)
		}
		out, err := UnmarshalPacket(data, ct)
		if err != nil {
			t.Fatalf("UnmarshalPacket empty(%s): %v", ct, err)
		}
		if !reflect.DeepEqual(telemetry.Packet{}, out) {
			t.Errorf("%s empty packet mismatch: %+v", ct, out)
		}
	}
}

// TestFrameRoundTrip covers every Frame variant in both formats.
func TestFrameRoundTrip(t *testing.T) {
	pkt := samplePacket()
	snap := sampleHostSnapshot()
	ack := sampleAck()
	ds := sampleDesiredState()
	sr := config.SnapshotRequest{RequestID: "req-1", Scopes: []string{"host.process.basic.read", "host.connection.summary.read"}}
	hello := Hello{
		SchemaVersion: 2, Hostname: "host-1", Platform: "windows", AgentVersion: "0.3.0",
		Permissions: permission.PermissionReport{
			Supported:  []string{"probe.icmp", "probe.dns"},
			Granted:    []string{"probe.icmp", "probe.dns"},
			Effective:  []string{"probe.icmp"},
			Source:     "environment",
			PolicyHash: "abc123",
		},
		ReportedConfigVersion: 7,
	}
	ms := MonitorStatus{
		ConfigVersion: 8, PolicyHash: "abc123", UploadIntervalSeconds: 5,
		Statuses: []MonitorStatusEntry{
			{MonitorID: "probe_mon1", Status: MonitorStatusActive, EffectiveIntervalSeconds: 15, CycleDeadlineMs: 5800, TargetConfigSerial: 41},
			{MonitorID: "probe_mon2", Status: MonitorStatusPermissionBlocked, MissingPermissions: []string{"probe.http.extended"}, Reason: "method_requires_extended"},
			{MonitorID: "probe_mon3", Status: MonitorStatusTargetBlocked, MatchedSelector: "scope:loopback", Reason: "literal_denied"},
		},
	}
	isr := config.IncidentSnapshotRequest{
		RequestID: "isnapreq-1", IncidentID: "inc-1", BudgetMs: 10_000,
		Targets: []config.SnapshotTargetRef{
			{MonitorID: "probe_mon1", Kind: "http", Target: "http://example.com/generate_204", Port: 80},
			{MonitorID: "probe_mon2", Kind: "icmp", Target: "1.1.1.1"},
		},
	}
	tr := config.TraceRequest{
		ReportID: "trace-1", Mode: config.TraceModeTCP, DestinationHost: "example.com", TCPPort: 443,
		MaxHops: 30, AttemptsPerHop: 3, TotalTimeoutMs: 30_000, ResolveHopHostnames: true, BudgetMs: 60_000,
	}
	frames := map[string]Frame{
		"hello":                     {Hello: &hello},
		"packet":                    {Packet: &pkt},
		"host_snapshot":             {HostSnapshot: &snap},
		"monitor_status":            {MonitorStatus: &ms},
		"ack":                       {Ack: &ack},
		"desired_state":             {DesiredState: &ds},
		"snapshot_request":          {SnapshotRequest: &sr},
		"incident_snapshot_request": {IncidentSnapshotRequest: &isr},
		"trace_request":             {TraceRequest: &tr},
	}
	for name, in := range frames {
		for _, ct := range []string{ContentTypeJSON, ContentTypeProtobuf} {
			data, err := MarshalFrame(in, ct)
			if err != nil {
				t.Fatalf("MarshalFrame %s(%s): %v", name, ct, err)
			}
			out, err := UnmarshalFrame(data, ct)
			if err != nil {
				t.Fatalf("UnmarshalFrame %s(%s): %v", name, ct, err)
			}
			if !reflect.DeepEqual(in, out) {
				t.Errorf("%s(%s) round-trip mismatch:\n in=%+v\nout=%+v", name, ct, in, out)
			}
		}
	}
}

// A frame with zero or multiple payloads must be rejected on both paths.
func TestFrameVariantValidation(t *testing.T) {
	ack := sampleAck()
	hello := Hello{SchemaVersion: 1}
	bad := []Frame{
		{},                         // empty
		{Hello: &hello, Ack: &ack}, // two payloads
	}
	for i, f := range bad {
		for _, ct := range []string{ContentTypeJSON, ContentTypeProtobuf} {
			if _, err := MarshalFrame(f, ct); err != ErrFrameVariant {
				t.Errorf("MarshalFrame bad[%d](%s): want ErrFrameVariant, got %v", i, ct, err)
			}
		}
	}
	// Unmarshal side: an empty JSON object decodes to zero payloads.
	if _, err := UnmarshalFrame([]byte(`{}`), ContentTypeJSON); err != ErrFrameVariant {
		t.Errorf("UnmarshalFrame empty json: want ErrFrameVariant, got %v", err)
	}
	// And an empty protobuf frame likewise.
	if _, err := UnmarshalFrame(nil, ContentTypeProtobuf); err != ErrFrameVariant {
		t.Errorf("UnmarshalFrame empty protobuf: want ErrFrameVariant, got %v", err)
	}
}

func TestSubprotocolContentType(t *testing.T) {
	cases := map[string]string{
		SubprotocolProtobuf: ContentTypeProtobuf,
		SubprotocolJSON:     ContentTypeJSON,
		"":                  ContentTypeJSON,
		"bogus":             ContentTypeJSON,
	}
	for sub, want := range cases {
		if got := SubprotocolContentType(sub); got != want {
			t.Errorf("SubprotocolContentType(%q)=%q want %q", sub, got, want)
		}
	}
}

func TestNegotiate(t *testing.T) {
	cases := map[string]string{
		"":                                      ContentTypeJSON,
		"application/json":                      ContentTypeJSON,
		"application/x-protobuf":                ContentTypeProtobuf,
		"application/x-protobuf; charset=utf-8": ContentTypeProtobuf,
		"application/x-protobuf, application/json": ContentTypeProtobuf,
		"text/plain": ContentTypeJSON,
		// q-values: protobuf explicitly rejected must fall back to JSON.
		"application/x-protobuf;q=0, application/json":   ContentTypeJSON,
		"application/x-protobuf;q=0.0":                   ContentTypeJSON,
		"application/json, application/x-protobuf;q=0.9": ContentTypeProtobuf,
	}
	for header, want := range cases {
		if got := Negotiate(header); got != want {
			t.Errorf("Negotiate(%q)=%q want %q", header, got, want)
		}
	}
}

// Protobuf should be materially smaller than JSON for a representative packet.
func TestProtobufSmallerThanJSON(t *testing.T) {
	in := Frame{Packet: func() *telemetry.Packet { p := samplePacket(); return &p }()}
	j, _ := MarshalFrame(in, ContentTypeJSON)
	p, _ := MarshalFrame(in, ContentTypeProtobuf)
	t.Logf("frame size: json=%d protobuf=%d (%.0f%% of json)", len(j), len(p), 100*float64(len(p))/float64(len(j)))
	if len(p) >= len(j) {
		t.Errorf("expected protobuf smaller than json: json=%d protobuf=%d", len(j), len(p))
	}
}

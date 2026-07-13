package wire

import (
	"reflect"
	"testing"
	"time"

	"github.com/nettact/protocol/config"
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
			{TS: ts, Kind: telemetry.ICMPRTTms, Target: "1.1.1.1", Layer: telemetry.HealthLayer("internet"), Value: 12.5, Unit: telemetry.UnitMs, Labels: map[string]string{"iface": "eth0", "region": "us"}, MonitorID: "probe_mon1"},
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
				SampledAt: ts,
				WiFiState: telemetry.WiFiCollectionOK,
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
	return telemetry.HostSnapshot{
		TS: ts, RequestID: "req-1", ProcessTotal: 2,
		Processes: []telemetry.ProcessInfo{
			{PID: 42, Name: "nginx", User: "root", Status: "Running", CPUPct: 1.5, RSSBytes: 1 << 20, VirtBytes: 1 << 21, DiskReadBytes: 100, DiskWriteBytes: 200, RunTimeSeconds: 3600.5},
		},
		Connections: []telemetry.ConnectionInfo{
			{Proto: "tcp", LocalAddr: "10.0.0.2:80", RemoteAddr: "1.2.3.4:5555", State: "ESTABLISHED", PID: 42, ProcessName: "nginx"},
		},
	}
}

func sampleDesiredState() config.DesiredState {
	return config.DesiredState{
		ConfigVersion: 8,
		ProbeTargets: []config.ProbeTarget{
			{MonitorID: "probe_mon1", Kind: "icmp", Name: "Cloudflare DNS", Target: "1.1.1.1", Params: config.ProbeParams{IntervalSeconds: 10, TimeoutMs: 1000, PacketSize: 56, Retries: 2, PacketCount: 3, GlobalTimeoutMs: 10000}},
			{Kind: "http", Name: "Example keyword", Target: "https://example.com", Params: config.ProbeParams{
				Method: "POST", ExpectedStatus: 200, AcceptedStatuses: "200-299,301",
				Keyword: "Example Domain", KeywordInvert: true, Headers: map[string]string{"X-Test": "1"},
				Body: `{"k":"v"}`, MaxRedirects: 5, IgnoreTLS: true, MaxResponseBytes: 2048,
			}},
			{Kind: "tcp", Name: "TLS port", Target: "1.1.1.1", Params: config.ProbeParams{Port: 443, TLS: true, TimeoutMs: 2000}},
			{Kind: "dns", Name: "MX lookup", Target: "example.com", Params: config.ProbeParams{RecordType: "MX", ResolverServer: "https://cloudflare-dns.com/dns-query", ResolverProtocol: "doh"}},
			{Kind: "nat", Name: "NAT type", Target: "stun.example.com", Params: config.ProbeParams{Port: 3478, NATTransport: "udp", STUNServer2: "stun2.example.com:3478", TimeoutMs: 3000}},
			{MonitorID: "probe_gw1", Kind: "gateway", Name: "LAN gateway", Target: "gateway", Params: config.ProbeParams{Interface: "以太网", PacketCount: 3, TimeoutMs: 2000}},
		},
		Intervals: config.Intervals{BaseSeconds: 10, RegularSeconds: 60},
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
	sr := config.SnapshotRequest{RequestID: "req-1", WantProcesses: true, WantConnections: false}
	hello := Hello{
		SchemaVersion: 1, Hostname: "host-1", Platform: "windows", AgentVersion: "0.3.0",
		Capabilities: []string{"probe.icmp", "host.stat.read"}, ReportedConfigVersion: 7,
	}
	frames := map[string]Frame{
		"hello":            {Hello: &hello},
		"packet":           {Packet: &pkt},
		"host_snapshot":    {HostSnapshot: &snap},
		"ack":              {Ack: &ack},
		"desired_state":    {DesiredState: &ds},
		"snapshot_request": {SnapshotRequest: &sr},
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

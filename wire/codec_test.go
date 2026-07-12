package wire

import (
	"reflect"
	"testing"
	"time"

	"github.com/nettact/protocol/config"
	"github.com/nettact/protocol/telemetry"
)

// samplePacket exercises every field type: timestamps, maps, repeated fields,
// a false bool (Up), int32/uint64/float64, and a populated HostSnapshot.
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
			{Kind: telemetry.InventoryInterface, Op: telemetry.OpUpsert, ID: "eth0", Name: "eth0", Addrs: []string{"10.0.0.2"}, Gateway: "10.0.0.1", DNS: []string{"1.1.1.1", "8.8.8.8"}, Up: false},
			{Kind: telemetry.InventoryDevice, Op: telemetry.OpUpsert, ID: "aa:bb:cc:dd:ee:ff", MAC: "aa:bb:cc:dd:ee:ff", IP: "10.0.0.5", Hostname: "printer", Vendor: "HP", LastSeen: ts},
		},
		HostSnapshot: &telemetry.HostSnapshot{
			TS: ts, RequestID: "req-1", ProcessTotal: 2,
			Processes: []telemetry.ProcessInfo{
				{PID: 42, Name: "nginx", User: "root", Status: "Running", CPUPct: 1.5, RSSBytes: 1 << 20, VirtBytes: 1 << 21, DiskReadBytes: 100, DiskWriteBytes: 200, RunTimeSeconds: 3600.5},
			},
			Connections: []telemetry.ConnectionInfo{
				{Proto: "tcp", LocalAddr: "10.0.0.2:80", RemoteAddr: "1.2.3.4:5555", State: "ESTABLISHED", PID: 42, ProcessName: "nginx"},
			},
		},
	}
}

func sampleAck() Ack {
	ts := time.Date(2026, 7, 11, 12, 30, 46, 0, time.UTC)
	return Ack{
		HighestSequence: 9001,
		ServerTime:      ts,
		ConfigVersion:   8,
		DesiredState: &config.DesiredState{
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
			},
			Intervals:       config.Intervals{BaseSeconds: 10, RegularSeconds: 60},
			SnapshotRequest: &config.SnapshotRequest{RequestID: "req-1", WantProcesses: true, WantConnections: false},
		},
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

// Empty packet and nil-DesiredState ack must round-trip without spurious fields.
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

		ackData, err := MarshalAck(Ack{HighestSequence: 5}, ct)
		if err != nil {
			t.Fatalf("MarshalAck nil-ds(%s): %v", ct, err)
		}
		ack, err := UnmarshalAck(ackData, ct)
		if err != nil {
			t.Fatalf("UnmarshalAck nil-ds(%s): %v", ct, err)
		}
		if ack.DesiredState != nil || ack.HighestSequence != 5 {
			t.Errorf("%s nil-ds ack mismatch: %+v", ct, ack)
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
	in := samplePacket()
	j, _ := MarshalPacket(in, ContentTypeJSON)
	p, _ := MarshalPacket(in, ContentTypeProtobuf)
	t.Logf("packet size: json=%d protobuf=%d (%.0f%% of json)", len(j), len(p), 100*float64(len(p))/float64(len(j)))
	if len(p) >= len(j) {
		t.Errorf("expected protobuf smaller than json: json=%d protobuf=%d", len(j), len(p))
	}
}

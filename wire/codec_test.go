package wire

import (
	"reflect"
	"testing"
	"time"

	"github.com/nettact/protocol/config"
	"github.com/nettact/protocol/gamesense"
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
		GameRuns:    sampleGameRuns(ts),
		GameBuckets: sampleGameBuckets(ts),
	}
}

// sampleGameRuns covers both run shapes: one still going (no end) and one
// finished. The distinction is carried by presence alone, so a converter that
// substituted a zero time would pass every other assertion and silently declare
// live runs to have ended in year one.
//
// The pair also splits on profile: the live run matched a configured game, the
// finished one is an unmatched "other process". An empty profile id must stay
// empty rather than acquire whichever profile the previous run belonged to.
func sampleGameRuns(ts time.Time) []gamesense.Run {
	ended := ts.Add(90 * time.Second)
	return []gamesense.Run{
		{
			ID: "run-live", Proc: "eldenring.exe", Title: "ELDEN RING", ProfileID: "gp_er",
			StartedAt: ts, LastSeenAt: ts.Add(30 * time.Second),
			Source: gamesense.SourcePresentMonService,
			Caps:   []string{gamesense.CapDisplayed, gamesense.CapFrameType, gamesense.CapPresentMeta, gamesense.CapPerFrameComplete},
		},
		{
			ID: "run-done", Proc: "cs2.exe",
			StartedAt: ts, LastSeenAt: ended, EndedAt: &ended,
			Source: gamesense.SourcePresentMonService,
		},
	}
}

// sampleGameBuckets covers the four shapes a second comes in. The first is fully
// observed, down to the diag blocks a TierDiag profile buys. The second is the
// important case: every optional field is nil, and the round-trip must bring
// back nil rather than a zero that would read as "this game dropped no frames"
// when nothing ever looked.
//
// The third is the half-observed middle, which is where a converter that reads
// emptiness as absence goes wrong. Its stutter block is present with count 0 — a
// second that was watched and held no hitch, which must not be dropped as if
// nothing watched — and its resource block carries memory without CPU, the shape
// of the first second of a run, where there is no delta to compute a percentage
// from yet. Its two diag blocks are the same hazard one layer down: adapter
// telemetry with utilization but no memory (an ordinary vendor gap), and a
// process VRAM block that is empty on the wire because the process holds no
// dedicated bytes and the OS published no budget — an all-defaults message that
// still has to arrive as a block.
//
// The fourth is a degraded second: the sensor overran its per-second budget and
// stopped polling, so the frame-derived breakdowns continue while the polled
// blocks stop. Losing that distinction would make a partial second look like a
// base-tier one, which is the difference between "we stopped looking" and "we
// were never looking".
func sampleGameBuckets(ts time.Time) []gamesense.Bucket {
	displayed, dropped, app, generated := 140, 2, 71, 71
	sync := 0
	tearing := true
	cpu := 42.5
	var ws, priv uint64 = 1 << 30, 1<<30 + 1<<28
	util, core := 96.5, 88.25
	var memUsed, memSize, vram, budget uint64 = 7 << 30, 8 << 30, 6 << 30, 7<<30 + 1<<29
	full := make([]uint32, gamesense.HistBins)
	full[12], full[16] = 140, 2
	sparse := make([]uint32, gamesense.HistBins)
	sparse[9] = 143
	steady := make([]uint32, gamesense.HistBins)
	steady[11] = 144
	degraded := make([]uint32, gamesense.HistBins)
	degraded[13] = 139
	return []gamesense.Bucket{
		{
			RunID: "run-live", TS: ts,
			Sample: gamesense.Sample{
				Frames:         gamesense.Frames{Presented: 142, Displayed: &displayed, Dropped: &dropped, App: &app, Generated: &generated},
				FT:             gamesense.FrameTimes{Avg: 6.944, P50: 6.8, P95: 8.1, P99: 11.2, Max: 23.5, SD: 1.42},
				Hist:           gamesense.Histogram{Layout: gamesense.HistLayoutLog24V1, Counts: full},
				DispFT:         &gamesense.DispFT{Avg: 7.1, P95: 8.4},
				Present:        &gamesense.Present{Mode: gamesense.PresentModeHardwareIndependentFlip, Sync: &sync, Tearing: &tearing, API: gamesense.APIDXGI, Changed: true},
				Stutter:        &gamesense.Stutter{Count: 2, ExcessMs: 118.4},
				ProcRes:        &gamesense.ProcRes{CPUPct: &cpu, WSBytes: &ws, PrivBytes: &priv},
				CPUSplit:       &gamesense.CPUSplit{BusyAvg: 4.12, BusyP95: 5.9, WaitAvg: 2.81, WaitP95: 3.44},
				GPUSplit:       &gamesense.GPUSplit{LatencyAvg: 1.21, TimeAvg: 6.13, TimeP95: 7.72, BusyAvg: 5.86, BusyP95: 7.21, WaitAvg: 0.27, InPresentAvg: 0.94, RenderLatencyAvg: 5.18},
				Latency:        &gamesense.Latency{DisplayAvg: 21.43, AnimErrAvg: 1.12, AnimErrP95: 3.61},
				GPUTel:         &gamesense.GPUTel{UtilPct: &util, MemUsed: &memUsed, MemSize: &memSize},
				ProcVRAM:       &gamesense.ProcVRAM{Used: vram, Budget: &budget},
				BusiestCorePct: &core,
				Quality:        []string{gamesense.QualityHistClipped},
			},
		},
		{
			RunID: "run-done", TS: ts.Add(time.Second),
			Sample: gamesense.Sample{
				Frames: gamesense.Frames{Presented: 143},
				FT:     gamesense.FrameTimes{Avg: 6.99, P50: 6.9, P95: 7.4, P99: 7.9, Max: 8.2, SD: 0.31},
				Hist:   gamesense.Histogram{Layout: gamesense.HistLayoutLog24V1, Counts: sparse},
			},
		},
		{
			RunID: "run-live", TS: ts.Add(2 * time.Second),
			Sample: gamesense.Sample{
				Frames:   gamesense.Frames{Presented: 144},
				FT:       gamesense.FrameTimes{Avg: 6.94, P50: 6.94, P95: 7.0, P99: 7.1, Max: 7.2, SD: 0.08},
				Hist:     gamesense.Histogram{Layout: gamesense.HistLayoutLog24V1, Counts: steady},
				Stutter:  &gamesense.Stutter{},
				ProcRes:  &gamesense.ProcRes{WSBytes: &ws, PrivBytes: &priv},
				GPUTel:   &gamesense.GPUTel{UtilPct: &util},
				ProcVRAM: &gamesense.ProcVRAM{},
			},
		},
		{
			RunID: "run-live", TS: ts.Add(3 * time.Second),
			Sample: gamesense.Sample{
				Frames:   gamesense.Frames{Presented: 139},
				FT:       gamesense.FrameTimes{Avg: 7.19, P50: 7.1, P95: 7.8, P99: 8.4, Max: 9.1, SD: 0.44},
				Hist:     gamesense.Histogram{Layout: gamesense.HistLayoutLog24V1, Counts: degraded},
				CPUSplit: &gamesense.CPUSplit{BusyAvg: 4.4, BusyP95: 6.2, WaitAvg: 2.79, WaitP95: 3.5},
				GPUSplit: &gamesense.GPUSplit{LatencyAvg: 1.3, TimeAvg: 6.4, TimeP95: 8.0, BusyAvg: 6.05, BusyP95: 7.5, WaitAvg: 0.35, InPresentAvg: 1.02, RenderLatencyAvg: 5.4},
				Latency:  &gamesense.Latency{DisplayAvg: 22.1, AnimErrAvg: 1.3, AnimErrP95: 4.02},
				Quality:  []string{gamesense.QualityDiagDegraded},
			},
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
		Game: sampleGameConfig(),
	}
}

// sampleGameConfig exercises the game block's own version axis alongside a
// profile set that differs field by field: one profile names several executables
// and a target rate, the other names one and leaves the target unset. A target
// of 0 means "unset", so a converter that turned it into a rate would be
// inventing a goal the user never set.
func sampleGameConfig() *config.GameConfig {
	return &config.GameConfig{
		Version:         5,
		RecordUnmatched: true,
		Profiles: []config.GameProfile{
			{ID: "gp_er", Name: "ELDEN RING", Exe: []string{"eldenring.exe", "start_protected_game.exe"}, TargetFPS: 60, Tier: "diag"},
			{ID: "gp_cs2", Name: "Counter-Strike 2", Exe: []string{"cs2.exe"}, Tier: "base"},
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

// A second's stutter and resource blocks say what they say by being there, and
// the emptiest of them are the ones a converter is most likely to lose: reading
// an empty message as an absent one turns "watched, and nothing stuttered" into
// "nothing was watching", which is the difference between a smooth run and an
// unmeasured one. Checked in both formats, because only one of them has a
// wire-level notion of message presence to get right.
func TestGameBucketPresenceSurvivesEmptyBlocks(t *testing.T) {
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
		if len(out.GameBuckets) != len(in.GameBuckets) {
			t.Fatalf("%s: %d buckets, want %d", ct, len(out.GameBuckets), len(in.GameBuckets))
		}
		// The unobserved second must not acquire blocks it never had.
		if b := out.GameBuckets[1]; b.Stutter != nil || b.ProcRes != nil {
			t.Errorf("%s: unwatched second gained stutter=%+v proc_res=%+v", ct, b.Stutter, b.ProcRes)
		}
		b := out.GameBuckets[2]
		if b.Stutter == nil {
			t.Fatalf("%s: quiet second lost its stutter block", ct)
		}
		if *b.Stutter != (gamesense.Stutter{}) {
			t.Errorf("%s: quiet stutter = %+v, want a zeroed block", ct, *b.Stutter)
		}
		if b.ProcRes == nil {
			t.Fatalf("%s: half-observed second lost its resource block", ct)
		}
		// The half that was never read stays unread; the half that was survives.
		if b.ProcRes.CPUPct != nil {
			t.Errorf("%s: cpu appeared with no delta to compute it from: %v", ct, *b.ProcRes.CPUPct)
		}
		if b.ProcRes.WSBytes == nil || *b.ProcRes.WSBytes != 1<<30 {
			t.Errorf("%s: working set = %v", ct, b.ProcRes.WSBytes)
		}
	}
}

// The diag blocks add three presence rules a converter can get wrong without
// failing any round-trip that only checks full values: a block whose fields are
// all defaults must still arrive as a block, a vendor gap inside GPUTel must
// stay a gap, and a degraded second must keep the frame-derived halves it never
// lost. Checked in both formats because only one of them has a wire-level notion
// of message presence to get right.
func TestGameDiagBlocksSurviveTheirPartialShapes(t *testing.T) {
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
		if len(out.GameBuckets) != 4 {
			t.Fatalf("%s: %d buckets, want 4", ct, len(out.GameBuckets))
		}
		// The base-tier second buys none of this and must not acquire any of it.
		if b := out.GameBuckets[1]; b.CPUSplit != nil || b.GPUSplit != nil || b.Latency != nil ||
			b.GPUTel != nil || b.ProcVRAM != nil || b.BusiestCorePct != nil {
			t.Errorf("%s: an unobserved second gained diag blocks: %+v", ct, b.Sample)
		}
		half := out.GameBuckets[2]
		if half.GPUTel == nil {
			t.Fatalf("%s: partially-published adapter telemetry was dropped entirely", ct)
		}
		if half.GPUTel.UtilPct == nil || *half.GPUTel.UtilPct != 96.5 {
			t.Errorf("%s: gpu utilization = %v", ct, half.GPUTel.UtilPct)
		}
		if half.GPUTel.MemUsed != nil || half.GPUTel.MemSize != nil {
			t.Errorf("%s: memory appeared from a card that never published it: %+v", ct, *half.GPUTel)
		}
		// An all-defaults block is the emptiest thing on the wire and the easiest
		// to mistake for nothing: zero committed bytes with no budget is a reading.
		if half.ProcVRAM == nil {
			t.Fatalf("%s: an empty process-vram block was read as an absent one", ct)
		}
		if half.ProcVRAM.Used != 0 || half.ProcVRAM.Budget != nil {
			t.Errorf("%s: process vram = %+v, want a zeroed, budget-less block", ct, *half.ProcVRAM)
		}
		if half.BusiestCorePct != nil {
			t.Errorf("%s: busiest core appeared unmeasured: %v", ct, *half.BusiestCorePct)
		}
		// Degradation stops the polling, not the frame stream.
		deg := out.GameBuckets[3]
		if deg.CPUSplit == nil || deg.GPUSplit == nil || deg.Latency == nil {
			t.Errorf("%s: a degraded second lost its frame-derived blocks: %+v", ct, deg.Sample)
		}
		if deg.GPUTel != nil || deg.ProcVRAM != nil || deg.BusiestCorePct != nil {
			t.Errorf("%s: a degraded second still carries polled blocks: %+v", ct, deg.Sample)
		}
		if len(deg.Quality) != 1 || deg.Quality[0] != gamesense.QualityDiagDegraded {
			t.Errorf("%s: degraded quality = %v", ct, deg.Quality)
		}
		// And the fully-observed second keeps every value, so a converter cannot
		// pass the presence rules above by dropping the contents.
		full := out.GameBuckets[0]
		if full.GPUSplit == nil || *full.GPUSplit != *in.GameBuckets[0].GPUSplit {
			t.Errorf("%s: gpu split = %+v, want %+v", ct, full.GPUSplit, in.GameBuckets[0].GPUSplit)
		}
		if full.CPUSplit == nil || *full.CPUSplit != *in.GameBuckets[0].CPUSplit {
			t.Errorf("%s: cpu split = %+v, want %+v", ct, full.CPUSplit, in.GameBuckets[0].CPUSplit)
		}
		if full.Latency == nil || *full.Latency != *in.GameBuckets[0].Latency {
			t.Errorf("%s: latency = %+v, want %+v", ct, full.Latency, in.GameBuckets[0].Latency)
		}
		if full.ProcVRAM == nil || full.ProcVRAM.Budget == nil || *full.ProcVRAM.Budget != *in.GameBuckets[0].ProcVRAM.Budget {
			t.Errorf("%s: process vram = %+v", ct, full.ProcVRAM)
		}
		if full.BusiestCorePct == nil || *full.BusiestCorePct != *in.GameBuckets[0].BusiestCorePct {
			t.Errorf("%s: busiest core = %v", ct, full.BusiestCorePct)
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
		EgressProxyID: "prx_wg", EgressConfigSerial: 4,
	}
	// Every TraceResult field populated, including the path attestation — a field
	// dropped in one converter half only surfaces through this fixture.
	tres := telemetry.TraceResult{
		ReportID: "trace-1", Mode: config.TraceModeICMP, Status: telemetry.TraceStatusPartial, Reason: "deadline_exceeded",
		DestinationIP: "10.7.0.10", Reached: true, ReachedTTL: 3,
		StartedAt: time.Unix(1700000300, 0).UTC(), CompletedAt: time.Unix(1700000310, 0).UTC(),
		Hops: []telemetry.TraceHop{
			{TTL: 1, Attempts: []telemetry.TraceAttempt{
				{ResponderAddr: "10.7.0.1", Hostname: "gw.internal", RTTMs: 1.25},
				{Timeout: true},
			}},
			{TTL: 2, Attempts: []telemetry.TraceAttempt{{ResponderAddr: "10.7.0.10", RTTMs: 8.5}}},
		},
		PathScope: telemetry.TracePathWireGuardInner, EgressProxyID: "prx_wg", EgressConfigSerial: 4,
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
		"trace_result":              {TraceResult: &tres},
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

// The game block is optional, and its absence is a claim: "the server has
// nothing to say about game capture" is not the same as "record everything with
// no profiles defined". Both must survive both encodings intact, because a
// converter that materialized an empty block out of a nil would switch every
// agent's recording behaviour on without anyone configuring it.
func TestDesiredStateGamePresence(t *testing.T) {
	cases := map[string]*config.GameConfig{
		"absent":        nil,
		"empty":         {},
		"no profiles":   {Version: 3, RecordUnmatched: true},
		"with profiles": sampleGameConfig(),
	}
	for name, game := range cases {
		ds := config.DesiredState{
			ConfigVersion: 8,
			Intervals:     config.Intervals{BaseSeconds: 10, RegularSeconds: 60},
			Game:          game,
		}
		for _, ct := range []string{ContentTypeJSON, ContentTypeProtobuf} {
			data, err := MarshalFrame(Frame{DesiredState: &ds}, ct)
			if err != nil {
				t.Fatalf("MarshalFrame %s(%s): %v", name, ct, err)
			}
			out, err := UnmarshalFrame(data, ct)
			if err != nil {
				t.Fatalf("UnmarshalFrame %s(%s): %v", name, ct, err)
			}
			if !reflect.DeepEqual(ds, *out.DesiredState) {
				t.Errorf("%s(%s) round-trip mismatch:\n in=%+v\nout=%+v", name, ct, ds, *out.DesiredState)
			}
		}
	}
}

// The two serials are independent axes: a profile edit bumps the game version
// and leaves config_version alone, and a probe edit does the reverse. Carrying
// them in separate fields is what lets each side no-op on the other's change, so
// a converter that derived one from the other would reintroduce exactly the
// probe churn the split exists to prevent.
func TestDesiredStateSerialsStaySeparate(t *testing.T) {
	ds := config.DesiredState{
		ConfigVersion: 8,
		Intervals:     config.Intervals{BaseSeconds: 10, RegularSeconds: 60},
		Game:          &config.GameConfig{Version: 41},
	}
	for _, ct := range []string{ContentTypeJSON, ContentTypeProtobuf} {
		data, err := MarshalFrame(Frame{DesiredState: &ds}, ct)
		if err != nil {
			t.Fatalf("MarshalFrame(%s): %v", ct, err)
		}
		out, err := UnmarshalFrame(data, ct)
		if err != nil {
			t.Fatalf("UnmarshalFrame(%s): %v", ct, err)
		}
		got := out.DesiredState
		if got.ConfigVersion != 8 || got.Game == nil || got.Game.Version != 41 {
			t.Errorf("%s: config_version = %d, game version = %+v", ct, got.ConfigVersion, got.Game)
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

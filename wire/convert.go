package wire

import (
	"time"

	"github.com/nettact/protocol/config"
	"github.com/nettact/protocol/gamesense"
	"github.com/nettact/protocol/permission"
	"github.com/nettact/protocol/telemetry"
	"github.com/nettact/protocol/wire/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// This file maps the canonical hand-written protocol structs (which stay the
// in-memory representation used everywhere) to and from the protobuf-generated
// messages in ./pb. Only this package imports the protobuf runtime, keeping the
// protocol/telemetry and protocol/config type packages stdlib-only.
//
// Timestamp mapping: a zero time.Time round-trips as a nil protobuf Timestamp
// (and back to the zero time.Time), so "unset" stays unset in both directions.

func tsToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func tsFromProto(t *timestamppb.Timestamp) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.AsTime()
}

// Optional-value mapping: a Go nil pointer and an absent protobuf field mean the
// same thing — the value was never measured — so the two representations map
// straight onto each other and a zero never appears in place of an unknown.

func tsPtrToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return tsToProto(*t)
}

func tsPtrFromProto(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}

func intPtrToProto(v *int) *int32 {
	if v == nil {
		return nil
	}
	n := int32(*v)
	return &n
}

func intPtrFromProto(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

// ---- Packet ----

func packetToProto(p telemetry.Packet) *pb.Packet {
	out := &pb.Packet{
		SchemaVersion: int32(p.SchemaVersion),
		AgentId:       p.AgentID,
		SiteId:        p.SiteID,
		Sequence:      p.Sequence,
		SentAt:        tsToProto(p.SentAt),
	}
	if len(p.Metrics) > 0 {
		out.Metrics = make([]*pb.Metric, len(p.Metrics))
		for i, m := range p.Metrics {
			out.Metrics[i] = metricToProto(m)
		}
	}
	if len(p.Events) > 0 {
		out.Events = make([]*pb.Event, len(p.Events))
		for i, e := range p.Events {
			out.Events[i] = eventToProto(e)
		}
	}
	if len(p.InventoryDelta) > 0 {
		out.InventoryDelta = make([]*pb.InventoryItem, len(p.InventoryDelta))
		for i, it := range p.InventoryDelta {
			out.InventoryDelta[i] = inventoryToProto(it)
		}
	}
	if len(p.InterfaceSnapshots) > 0 {
		out.InterfaceSnapshots = make([]*pb.InterfaceSnapshot, len(p.InterfaceSnapshots))
		for i, s := range p.InterfaceSnapshots {
			out.InterfaceSnapshots[i] = interfaceSnapshotToProto(s)
		}
	}
	if len(p.GameRuns) > 0 {
		out.GameRuns = make([]*pb.GameRun, len(p.GameRuns))
		for i, r := range p.GameRuns {
			out.GameRuns[i] = gameRunToProto(r)
		}
	}
	if len(p.GameBuckets) > 0 {
		out.GameBuckets = make([]*pb.GameBucket, len(p.GameBuckets))
		for i, b := range p.GameBuckets {
			out.GameBuckets[i] = gameBucketToProto(b)
		}
	}
	if len(p.GameGaps) > 0 {
		out.GameGaps = make([]*pb.GameGap, len(p.GameGaps))
		for i, g := range p.GameGaps {
			out.GameGaps[i] = gameGapToProto(g)
		}
	}
	if len(p.GameHostSeconds) > 0 {
		out.GameHostSeconds = make([]*pb.GameHostSecond, len(p.GameHostSeconds))
		for i, h := range p.GameHostSeconds {
			out.GameHostSeconds[i] = gameHostSecondToProto(h)
		}
	}
	if len(p.TraceResults) > 0 {
		out.TraceResults = make([]*pb.TraceResult, len(p.TraceResults))
		for i, t := range p.TraceResults {
			out.TraceResults[i] = traceResultToProto(t)
		}
	}
	if len(p.SceneReports) > 0 {
		out.SceneReports = make([]*pb.SceneReport, len(p.SceneReports))
		for i, s := range p.SceneReports {
			out.SceneReports[i] = sceneReportToProto(s)
		}
	}
	return out
}

func packetFromProto(p *pb.Packet) telemetry.Packet {
	if p == nil {
		return telemetry.Packet{}
	}
	out := telemetry.Packet{
		SchemaVersion: int(p.SchemaVersion),
		AgentID:       p.AgentId,
		SiteID:        p.SiteId,
		Sequence:      p.Sequence,
		SentAt:        tsFromProto(p.SentAt),
	}
	if len(p.Metrics) > 0 {
		out.Metrics = make([]telemetry.Metric, len(p.Metrics))
		for i, m := range p.Metrics {
			out.Metrics[i] = metricFromProto(m)
		}
	}
	if len(p.Events) > 0 {
		out.Events = make([]telemetry.Event, len(p.Events))
		for i, e := range p.Events {
			out.Events[i] = eventFromProto(e)
		}
	}
	if len(p.InventoryDelta) > 0 {
		out.InventoryDelta = make([]telemetry.InventoryItem, len(p.InventoryDelta))
		for i, it := range p.InventoryDelta {
			out.InventoryDelta[i] = inventoryFromProto(it)
		}
	}
	if len(p.InterfaceSnapshots) > 0 {
		out.InterfaceSnapshots = make([]telemetry.InterfaceSnapshot, len(p.InterfaceSnapshots))
		for i, s := range p.InterfaceSnapshots {
			out.InterfaceSnapshots[i] = interfaceSnapshotFromProto(s)
		}
	}
	if len(p.GameRuns) > 0 {
		out.GameRuns = make([]gamesense.Run, len(p.GameRuns))
		for i, r := range p.GameRuns {
			out.GameRuns[i] = gameRunFromProto(r)
		}
	}
	if len(p.GameBuckets) > 0 {
		out.GameBuckets = make([]gamesense.Bucket, len(p.GameBuckets))
		for i, b := range p.GameBuckets {
			out.GameBuckets[i] = gameBucketFromProto(b)
		}
	}
	if len(p.GameGaps) > 0 {
		out.GameGaps = make([]gamesense.Gap, len(p.GameGaps))
		for i, g := range p.GameGaps {
			out.GameGaps[i] = gameGapFromProto(g)
		}
	}
	if len(p.GameHostSeconds) > 0 {
		out.GameHostSeconds = make([]gamesense.HostSecond, len(p.GameHostSeconds))
		for i, h := range p.GameHostSeconds {
			out.GameHostSeconds[i] = gameHostSecondFromProto(h)
		}
	}
	if len(p.TraceResults) > 0 {
		out.TraceResults = make([]telemetry.TraceResult, len(p.TraceResults))
		for i, t := range p.TraceResults {
			out.TraceResults[i] = traceResultFromProto(t)
		}
	}
	if len(p.SceneReports) > 0 {
		out.SceneReports = make([]telemetry.SceneReport, len(p.SceneReports))
		for i, s := range p.SceneReports {
			out.SceneReports[i] = sceneReportFromProto(s)
		}
	}
	return out
}

// ---- Game runs and second buckets ----

func gameRunToProto(r gamesense.Run) *pb.GameRun {
	return &pb.GameRun{
		Id:         r.ID,
		Proc:       r.Proc,
		Title:      r.Title,
		StartedAt:  tsToProto(r.StartedAt),
		LastSeenAt: tsToProto(r.LastSeenAt),
		EndedAt:    tsPtrToProto(r.EndedAt),
		Source:     r.Source,
		Caps:       r.Caps,
		ProfileId:  r.ProfileID,
	}
}

func gameRunFromProto(r *pb.GameRun) gamesense.Run {
	if r == nil {
		return gamesense.Run{}
	}
	return gamesense.Run{
		ID:         r.Id,
		Proc:       r.Proc,
		Title:      r.Title,
		StartedAt:  tsFromProto(r.StartedAt),
		LastSeenAt: tsFromProto(r.LastSeenAt),
		EndedAt:    tsPtrFromProto(r.EndedAt),
		Source:     r.Source,
		Caps:       r.Caps,
		ProfileID:  r.ProfileId,
	}
}

func gameBucketToProto(b gamesense.Bucket) *pb.GameBucket {
	out := &pb.GameBucket{
		RunId: b.RunID,
		Ts:    tsToProto(b.TS),
		Frames: &pb.GameFrames{
			Presented: int32(b.Frames.Presented),
			Displayed: intPtrToProto(b.Frames.Displayed),
			Dropped:   intPtrToProto(b.Frames.Dropped),
			App:       intPtrToProto(b.Frames.App),
			Generated: intPtrToProto(b.Frames.Generated),
		},
		Ft: &pb.GameFrameTimes{
			Avg: b.FT.Avg,
			P50: b.FT.P50,
			P95: b.FT.P95,
			P99: b.FT.P99,
			Max: b.FT.Max,
			Sd:  b.FT.SD,
		},
		Hist:    &pb.GameHistogram{Layout: b.Hist.Layout, Counts: b.Hist.Counts},
		Quality: b.Quality,
	}
	if b.DispFT != nil {
		out.DispFt = &pb.GameDispFT{Avg: b.DispFT.Avg, P95: b.DispFT.P95}
	}
	if b.Present != nil {
		out.Present = &pb.GamePresent{
			Mode:    b.Present.Mode,
			Sync:    intPtrToProto(b.Present.Sync),
			Tearing: b.Present.Tearing,
			Api:     b.Present.API,
			Changed: b.Present.Changed,
		}
	}
	// A stutter block with count 0 is a watched second that held no hitch, so the
	// nil check is on the block and never on its contents.
	if b.Stutter != nil {
		out.Stutter = &pb.GameStutter{Count: int32(b.Stutter.Count), ExcessMs: b.Stutter.ExcessMs}
	}
	if b.ProcRes != nil {
		out.ProcRes = &pb.GameProcRes{
			CpuPct:    b.ProcRes.CPUPct,
			WsBytes:   b.ProcRes.WSBytes,
			PrivBytes: b.ProcRes.PrivBytes,
		}
	}
	// The diag blocks. Each is nil-checked as a whole and copied field for field:
	// a block that exists was measured in full, so there is nothing to decide
	// inside one. The two exceptions carry their own pointers straight across.
	if c := b.CPUSplit; c != nil {
		out.CpuSplit = &pb.GameCPUSplit{
			BusyAvg: c.BusyAvg, BusyP95: c.BusyP95,
			WaitAvg: c.WaitAvg, WaitP95: c.WaitP95,
		}
	}
	if g := b.GPUSplit; g != nil {
		out.GpuSplit = &pb.GameGPUSplit{
			LatencyAvg:       g.LatencyAvg,
			TimeAvg:          g.TimeAvg,
			TimeP95:          g.TimeP95,
			BusyAvg:          g.BusyAvg,
			BusyP95:          g.BusyP95,
			WaitAvg:          g.WaitAvg,
			InPresentAvg:     g.InPresentAvg,
			RenderLatencyAvg: g.RenderLatencyAvg,
		}
	}
	if l := b.Latency; l != nil {
		out.Lat = &pb.GameLatency{
			DisplayAvg: l.DisplayAvg,
			AnimErrAvg: l.AnimErrAvg,
			AnimErrP95: l.AnimErrP95,
		}
	}
	if v := b.ProcVRAM; v != nil {
		out.ProcVram = &pb.GameProcVRAM{Used: v.Used, Budget: v.Budget}
	}
	return out
}

func gameBucketFromProto(b *pb.GameBucket) gamesense.Bucket {
	if b == nil {
		return gamesense.Bucket{}
	}
	out := gamesense.Bucket{RunID: b.RunId, TS: tsFromProto(b.Ts)}
	if f := b.Frames; f != nil {
		out.Frames = gamesense.Frames{
			Presented: int(f.Presented),
			Displayed: intPtrFromProto(f.Displayed),
			Dropped:   intPtrFromProto(f.Dropped),
			App:       intPtrFromProto(f.App),
			Generated: intPtrFromProto(f.Generated),
		}
	}
	if ft := b.Ft; ft != nil {
		out.FT = gamesense.FrameTimes{
			Avg: ft.Avg, P50: ft.P50, P95: ft.P95,
			P99: ft.P99, Max: ft.Max, SD: ft.Sd,
		}
	}
	if h := b.Hist; h != nil {
		out.Hist = gamesense.Histogram{Layout: h.Layout, Counts: h.Counts}
	}
	if d := b.DispFt; d != nil {
		out.DispFT = &gamesense.DispFT{Avg: d.Avg, P95: d.P95}
	}
	if pr := b.Present; pr != nil {
		out.Present = &gamesense.Present{
			Mode:    pr.Mode,
			Sync:    intPtrFromProto(pr.Sync),
			Tearing: pr.Tearing,
			API:     pr.Api,
			Changed: pr.Changed,
		}
	}
	if s := b.Stutter; s != nil {
		out.Stutter = &gamesense.Stutter{Count: int(s.Count), ExcessMs: s.ExcessMs}
	}
	if p := b.ProcRes; p != nil {
		out.ProcRes = &gamesense.ProcRes{
			CPUPct:    p.CpuPct,
			WSBytes:   p.WsBytes,
			PrivBytes: p.PrivBytes,
		}
	}
	if c := b.CpuSplit; c != nil {
		out.CPUSplit = &gamesense.CPUSplit{
			BusyAvg: c.BusyAvg, BusyP95: c.BusyP95,
			WaitAvg: c.WaitAvg, WaitP95: c.WaitP95,
		}
	}
	if g := b.GpuSplit; g != nil {
		out.GPUSplit = &gamesense.GPUSplit{
			LatencyAvg:       g.LatencyAvg,
			TimeAvg:          g.TimeAvg,
			TimeP95:          g.TimeP95,
			BusyAvg:          g.BusyAvg,
			BusyP95:          g.BusyP95,
			WaitAvg:          g.WaitAvg,
			InPresentAvg:     g.InPresentAvg,
			RenderLatencyAvg: g.RenderLatencyAvg,
		}
	}
	if l := b.Lat; l != nil {
		out.Latency = &gamesense.Latency{
			DisplayAvg: l.DisplayAvg,
			AnimErrAvg: l.AnimErrAvg,
			AnimErrP95: l.AnimErrP95,
		}
	}
	if v := b.ProcVram; v != nil {
		out.ProcVRAM = &gamesense.ProcVRAM{Used: v.Used, Budget: v.Budget}
	}
	out.Quality = b.Quality
	return out
}

// ---- Game gaps ----

func gameGapToProto(g gamesense.Gap) *pb.GameGap {
	return &pb.GameGap{
		Id:        g.ID,
		RunId:     g.RunID,
		Reason:    g.Reason,
		StartedAt: tsToProto(g.StartedAt),
		EndedAt:   tsToProto(g.EndedAt),
	}
}

func gameGapFromProto(g *pb.GameGap) gamesense.Gap {
	if g == nil {
		return gamesense.Gap{}
	}
	return gamesense.Gap{
		ID:        g.Id,
		RunID:     g.RunId,
		Reason:    g.Reason,
		StartedAt: tsFromProto(g.StartedAt),
		EndedAt:   tsFromProto(g.EndedAt),
	}
}

// ---- Machine-level seconds ----

func gameHostSecondToProto(h gamesense.HostSecond) *pb.GameHostSecond {
	out := &pb.GameHostSecond{Ts: tsToProto(h.TS), Quality: h.Quality}
	if c := h.CPU; c != nil {
		out.Cpu = &pb.GameHostCPU{TotalPct: c.TotalPct, BusiestPct: c.BusiestPct}
	}
	if m := h.Mem; m != nil {
		out.Mem = &pb.GameHostMem{Used: m.Used, Total: m.Total}
	}
	if c := h.CPUClock; c != nil {
		out.CpuClock = &pb.GameHostCPUClock{CurrentMhz: c.CurrentMHz, MaxMhz: c.MaxMHz}
	}
	if g := h.GPU; g != nil {
		out.Gpu = &pb.GameGPUTel{
			UtilPct: g.UtilPct, MemUsed: g.MemUsed, MemSize: g.MemSize,
			CoreMhz: g.CoreMHz, MemMhz: g.MemMHz,
		}
	}
	return out
}

func gameHostSecondFromProto(h *pb.GameHostSecond) gamesense.HostSecond {
	if h == nil {
		return gamesense.HostSecond{}
	}
	out := gamesense.HostSecond{TS: tsFromProto(h.Ts)}
	out.Quality = h.Quality
	if c := h.Cpu; c != nil {
		out.CPU = &gamesense.HostCPU{TotalPct: c.TotalPct, BusiestPct: c.BusiestPct}
	}
	if m := h.Mem; m != nil {
		out.Mem = &gamesense.HostMem{Used: m.Used, Total: m.Total}
	}
	if c := h.CpuClock; c != nil {
		out.CPUClock = &gamesense.HostCPUClock{CurrentMHz: c.CurrentMhz, MaxMHz: c.MaxMhz}
	}
	if g := h.Gpu; g != nil {
		out.GPU = &gamesense.GPUTel{
			UtilPct: g.UtilPct, MemUsed: g.MemUsed, MemSize: g.MemSize,
			CoreMHz: g.CoreMhz, MemMHz: g.MemMhz,
		}
	}
	return out
}

// ---- Metric ----

func metricToProto(m telemetry.Metric) *pb.Metric {
	return &pb.Metric{
		Ts:           tsToProto(m.TS),
		Kind:         string(m.Kind),
		Target:       m.Target,
		Layer:        string(m.Layer),
		Value:        m.Value,
		Unit:         m.Unit,
		Labels:       m.Labels,
		MonitorId:    m.MonitorID,
		ConfigSerial: int32(m.ConfigSerial),
	}
}

func metricFromProto(m *pb.Metric) telemetry.Metric {
	return telemetry.Metric{
		TS:           tsFromProto(m.Ts),
		Kind:         telemetry.MetricKind(m.Kind),
		Target:       m.Target,
		Layer:        telemetry.HealthLayer(m.Layer),
		Value:        m.Value,
		Unit:         m.Unit,
		Labels:       m.Labels,
		MonitorID:    m.MonitorId,
		ConfigSerial: int(m.ConfigSerial),
	}
}

// ---- Event ----

func eventToProto(e telemetry.Event) *pb.Event {
	return &pb.Event{
		Id:       e.ID,
		Ts:       tsToProto(e.TS),
		Type:     string(e.Type),
		Layer:    string(e.Layer),
		Severity: string(e.Severity),
		Message:  e.Message,
		Attrs:    e.Attrs,
	}
}

func eventFromProto(e *pb.Event) telemetry.Event {
	return telemetry.Event{
		ID:       e.Id,
		TS:       tsFromProto(e.Ts),
		Type:     telemetry.EventType(e.Type),
		Layer:    telemetry.HealthLayer(e.Layer),
		Severity: telemetry.Severity(e.Severity),
		Message:  e.Message,
		Attrs:    e.Attrs,
	}
}

// ---- InventoryItem (device-only) ----

func inventoryToProto(it telemetry.InventoryItem) *pb.InventoryItem {
	return &pb.InventoryItem{
		Kind:     string(it.Kind),
		Op:       string(it.Op),
		Id:       it.ID,
		Mac:      it.MAC,
		Ip:       it.IP,
		Hostname: it.Hostname,
		Vendor:   it.Vendor,
		LastSeen: tsToProto(it.LastSeen),
	}
}

func inventoryFromProto(it *pb.InventoryItem) telemetry.InventoryItem {
	return telemetry.InventoryItem{
		Kind:     telemetry.InventoryKind(it.Kind),
		Op:       telemetry.DeltaOp(it.Op),
		ID:       it.Id,
		MAC:      it.Mac,
		IP:       it.Ip,
		Hostname: it.Hostname,
		Vendor:   it.Vendor,
		LastSeen: tsFromProto(it.LastSeen),
	}
}

// ---- InterfaceSnapshot / InterfaceState / WiFiInfo ----

func interfaceSnapshotToProto(s telemetry.InterfaceSnapshot) *pb.InterfaceSnapshot {
	out := &pb.InterfaceSnapshot{
		SampledAt:  tsToProto(s.SampledAt),
		WifiState:  string(s.WiFiState),
		WifiReason: string(s.WiFiReason),
	}
	if s.DefaultRoute != nil {
		out.DefaultRoute = &pb.SnapshotRoute{
			Gateway:   s.DefaultRoute.Gateway,
			Interface: s.DefaultRoute.Interface,
		}
	}
	if len(s.Interfaces) > 0 {
		out.Interfaces = make([]*pb.InterfaceState, len(s.Interfaces))
		for i, ifs := range s.Interfaces {
			out.Interfaces[i] = interfaceStateToProto(ifs)
		}
	}
	return out
}

func interfaceSnapshotFromProto(s *pb.InterfaceSnapshot) telemetry.InterfaceSnapshot {
	if s == nil {
		return telemetry.InterfaceSnapshot{}
	}
	// Interfaces is intentionally non-omitempty (telemetry.InterfaceSnapshot): a
	// zero-interface round must decode to a non-nil empty slice so it serializes
	// as `interfaces: []` (the authoritative empty set that clears server rows),
	// never `null`, and reflect.DeepEqual matches the collector's explicit empty
	// slice. Protobuf repeated fields carry no nil-vs-empty distinction, so
	// allocate unconditionally rather than guarding on len > 0.
	out := telemetry.InterfaceSnapshot{
		SampledAt:  tsFromProto(s.SampledAt),
		WiFiState:  telemetry.WiFiCollectionState(s.WifiState),
		WiFiReason: telemetry.WiFiReason(s.WifiReason),
		Interfaces: make([]telemetry.InterfaceState, len(s.Interfaces)),
	}
	if s.DefaultRoute != nil {
		out.DefaultRoute = &telemetry.SnapshotRoute{
			Gateway:   s.DefaultRoute.Gateway,
			Interface: s.DefaultRoute.Interface,
		}
	}
	for i, ifs := range s.Interfaces {
		out.Interfaces[i] = interfaceStateFromProto(ifs)
	}
	return out
}

func interfaceStateToProto(s telemetry.InterfaceState) *pb.InterfaceState {
	return &pb.InterfaceState{
		Name:       s.Name,
		Addrs:      s.Addrs,
		Gateway:    s.Gateway,
		Dns:        s.DNS,
		Up:         s.Up,
		IsWireless: s.IsWireless,
		Wifi:       wifiInfoToProto(s.WiFi),
	}
}

func interfaceStateFromProto(s *pb.InterfaceState) telemetry.InterfaceState {
	if s == nil {
		return telemetry.InterfaceState{}
	}
	return telemetry.InterfaceState{
		Name:       s.Name,
		Addrs:      s.Addrs,
		Gateway:    s.Gateway,
		DNS:        s.Dns,
		Up:         s.Up,
		IsWireless: s.IsWireless,
		WiFi:       wifiInfoFromProto(s.Wifi),
	}
}

func wifiInfoToProto(w *telemetry.WiFiInfo) *pb.WiFiInfo {
	if w == nil {
		return nil
	}
	return &pb.WiFiInfo{
		State:   string(w.State),
		Reason:  string(w.Reason),
		Ssid:    w.SSID,
		Band:    string(w.Band),
		Channel: int32(w.Channel),
	}
}

func wifiInfoFromProto(w *pb.WiFiInfo) *telemetry.WiFiInfo {
	if w == nil {
		return nil
	}
	return &telemetry.WiFiInfo{
		State:   telemetry.WiFiLinkState(w.State),
		Reason:  telemetry.WiFiReason(w.Reason),
		SSID:    w.Ssid,
		Band:    telemetry.WiFiBand(w.Band),
		Channel: int(w.Channel),
	}
}

// ---- HostSnapshot ----

// intPtrToInt32Ptr / int32PtrToIntPtr bridge the Go *int (ProcessTotal) and the
// generated *int32 while preserving nil (absence).
func intPtrToInt32Ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func int32PtrToIntPtr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

func hostSnapshotToProto(h telemetry.HostSnapshot) *pb.HostSnapshot {
	out := &pb.HostSnapshot{
		Ts:           tsToProto(h.TS),
		RequestId:    h.RequestID,
		ProcessTotal: intPtrToInt32Ptr(h.ProcessTotal),
	}
	if len(h.Scopes) > 0 {
		out.Scopes = make([]*pb.SnapshotScopeResult, len(h.Scopes))
		for i, s := range h.Scopes {
			out.Scopes[i] = &pb.SnapshotScopeResult{Scope: s.Scope, Status: s.Status, Reason: s.Reason}
		}
	}
	if len(h.Processes) > 0 {
		out.Processes = make([]*pb.ProcessInfo, len(h.Processes))
		for i, p := range h.Processes {
			out.Processes[i] = &pb.ProcessInfo{
				Pid:            p.PID,
				Name:           p.Name,
				Status:         p.Status,
				User:           p.User,
				CpuPct:         p.CPUPct,
				RssBytes:       p.RSSBytes,
				VirtBytes:      p.VirtBytes,
				DiskReadBytes:  p.DiskReadBytes,
				DiskWriteBytes: p.DiskWriteBytes,
				RunTimeSeconds: p.RunTimeSeconds,
			}
		}
	}
	if len(h.Connections) > 0 {
		out.Connections = make([]*pb.ConnectionInfo, len(h.Connections))
		for i, c := range h.Connections {
			out.Connections[i] = &pb.ConnectionInfo{
				Proto:       c.Proto,
				State:       c.State,
				LocalAddr:   c.LocalAddr,
				RemoteAddr:  c.RemoteAddr,
				Pid:         c.PID,
				ProcessName: c.ProcessName,
			}
		}
	}
	return out
}

func hostSnapshotFromProto(h *pb.HostSnapshot) telemetry.HostSnapshot {
	out := telemetry.HostSnapshot{
		TS:           tsFromProto(h.Ts),
		RequestID:    h.RequestId,
		ProcessTotal: int32PtrToIntPtr(h.ProcessTotal),
	}
	if len(h.Scopes) > 0 {
		out.Scopes = make([]telemetry.SnapshotScopeResult, len(h.Scopes))
		for i, s := range h.Scopes {
			out.Scopes[i] = telemetry.SnapshotScopeResult{Scope: s.Scope, Status: s.Status, Reason: s.Reason}
		}
	}
	if len(h.Processes) > 0 {
		out.Processes = make([]telemetry.ProcessInfo, len(h.Processes))
		for i, p := range h.Processes {
			out.Processes[i] = telemetry.ProcessInfo{
				PID:            p.Pid,
				Name:           p.Name,
				Status:         p.Status,
				User:           p.User,
				CPUPct:         p.CpuPct,
				RSSBytes:       p.RssBytes,
				VirtBytes:      p.VirtBytes,
				DiskReadBytes:  p.DiskReadBytes,
				DiskWriteBytes: p.DiskWriteBytes,
				RunTimeSeconds: p.RunTimeSeconds,
			}
		}
	}
	if len(h.Connections) > 0 {
		out.Connections = make([]telemetry.ConnectionInfo, len(h.Connections))
		for i, c := range h.Connections {
			out.Connections[i] = telemetry.ConnectionInfo{
				Proto:       c.Proto,
				State:       c.State,
				LocalAddr:   c.LocalAddr,
				RemoteAddr:  c.RemoteAddr,
				PID:         c.Pid,
				ProcessName: c.ProcessName,
			}
		}
	}
	return out
}

// ---- MonitorStatus ----

func monitorStatusToProto(m MonitorStatus) *pb.MonitorStatus {
	out := &pb.MonitorStatus{
		ConfigVersion:         int32(m.ConfigVersion),
		PolicyHash:            m.PolicyHash,
		UploadIntervalSeconds: int32(m.UploadIntervalSeconds),
	}
	if len(m.Statuses) > 0 {
		out.Statuses = make([]*pb.MonitorStatusEntry, len(m.Statuses))
		for i, e := range m.Statuses {
			out.Statuses[i] = &pb.MonitorStatusEntry{
				MonitorId:                e.MonitorID,
				Status:                   e.Status,
				MissingPermissions:       e.MissingPermissions,
				MatchedSelector:          e.MatchedSelector,
				Reason:                   e.Reason,
				EffectiveIntervalSeconds: int32(e.EffectiveIntervalSeconds),
				CycleDeadlineMs:          int32(e.CycleDeadlineMs),
				TargetConfigSerial:       int32(e.TargetConfigSerial),
			}
		}
	}
	return out
}

func monitorStatusFromProto(m *pb.MonitorStatus) MonitorStatus {
	if m == nil {
		return MonitorStatus{}
	}
	out := MonitorStatus{
		ConfigVersion:         int(m.ConfigVersion),
		PolicyHash:            m.PolicyHash,
		UploadIntervalSeconds: int(m.UploadIntervalSeconds),
	}
	if len(m.Statuses) > 0 {
		out.Statuses = make([]MonitorStatusEntry, len(m.Statuses))
		for i, e := range m.Statuses {
			out.Statuses[i] = MonitorStatusEntry{
				MonitorID:                e.MonitorId,
				Status:                   e.Status,
				MissingPermissions:       e.MissingPermissions,
				MatchedSelector:          e.MatchedSelector,
				Reason:                   e.Reason,
				EffectiveIntervalSeconds: int(e.EffectiveIntervalSeconds),
				CycleDeadlineMs:          int(e.CycleDeadlineMs),
				TargetConfigSerial:       int(e.TargetConfigSerial),
			}
		}
	}
	return out
}

// ---- PermissionReport ----

// UnsupportedReasons is carried by reference like every other map here (Metric
// labels, Event attrs, ProbeParams headers). An empty map needs no special case:
// protobuf encodes no entries for it and JSON omits it, so both formats decode
// back to nil — which is the shape that means "nothing was probed" and is what
// an agent with nothing to explain should be sending anyway.
func permissionReportToProto(r permission.PermissionReport) *pb.PermissionReport {
	return &pb.PermissionReport{
		Supported:          r.Supported,
		Granted:            r.Granted,
		Effective:          r.Effective,
		Source:             r.Source,
		PolicyHash:         r.PolicyHash,
		UnsupportedReasons: r.UnsupportedReasons,
	}
}

func permissionReportFromProto(r *pb.PermissionReport) permission.PermissionReport {
	if r == nil {
		return permission.PermissionReport{}
	}
	return permission.PermissionReport{
		Supported:          r.Supported,
		Granted:            r.Granted,
		Effective:          r.Effective,
		Source:             r.Source,
		PolicyHash:         r.PolicyHash,
		UnsupportedReasons: r.UnsupportedReasons,
	}
}

// ---- Ack / DesiredState ----

func ackToProto(a Ack) *pb.TelemetryAck {
	return &pb.TelemetryAck{
		HighestSequence: a.HighestSequence,
		ServerTime:      tsToProto(a.ServerTime),
	}
}

func ackFromProto(a *pb.TelemetryAck) Ack {
	if a == nil {
		return Ack{}
	}
	return Ack{
		HighestSequence: a.HighestSequence,
		ServerTime:      tsFromProto(a.ServerTime),
	}
}

func desiredStateToProto(d config.DesiredState) *pb.DesiredState {
	out := &pb.DesiredState{
		ConfigVersion: int32(d.ConfigVersion),
		Intervals: &pb.Intervals{
			BaseSeconds:    int32(d.Intervals.BaseSeconds),
			RegularSeconds: int32(d.Intervals.RegularSeconds),
		},
	}
	if len(d.ProbeTargets) > 0 {
		out.ProbeTargets = make([]*pb.ProbeTarget, len(d.ProbeTargets))
		for i, t := range d.ProbeTargets {
			out.ProbeTargets[i] = &pb.ProbeTarget{
				MonitorId:    t.MonitorID,
				Kind:         t.Kind,
				Name:         t.Name,
				Target:       t.Target,
				ConfigSerial: int32(t.ConfigSerial),
				ProxyId:      t.ProxyID,
				Params: &pb.ProbeParams{
					IntervalSeconds:  int32(t.Params.IntervalSeconds),
					TimeoutMs:        int32(t.Params.TimeoutMs),
					PacketSize:       int32(t.Params.PacketSize),
					PacketCount:      int32(t.Params.PacketCount),
					GlobalTimeoutMs:  int32(t.Params.GlobalTimeoutMs),
					RecordType:       t.Params.RecordType,
					ResolverServer:   t.Params.ResolverServer,
					ResolverPort:     int32(t.Params.ResolverPort),
					ResolverProtocol: t.Params.ResolverProtocol,
					Method:           t.Params.Method,
					AcceptedStatuses: t.Params.AcceptedStatuses,
					Keyword:          t.Params.Keyword,
					KeywordInvert:    t.Params.KeywordInvert,
					Headers:          t.Params.Headers,
					Body:             t.Params.Body,
					MaxRedirects:     int32(t.Params.MaxRedirects),
					IgnoreTls:        t.Params.IgnoreTLS,
					MaxResponseBytes: int32(t.Params.MaxResponseBytes),
					Port:             int32(t.Params.Port),
					Tls:              t.Params.TLS,
					NatTransport:     t.Params.NATTransport,
					StunServer2:      t.Params.STUNServer2,
					Interface:        t.Params.Interface,
				},
			}
		}
	}
	if len(d.Proxies) > 0 {
		out.Proxies = make([]*pb.ProxySpec, len(d.Proxies))
		for i, p := range d.Proxies {
			out.Proxies[i] = proxySpecToProto(p)
		}
	}
	if d.Game != nil {
		out.Game = gameConfigToProto(*d.Game)
	}
	if d.Diag != nil {
		out.Diag = diagPolicyToProto(*d.Diag)
	}
	return out
}

// diagPolicyToProto mirrors the path-diagnostic policy block. Presence carries
// meaning here as it does for the game block: an absent block is a server with
// nothing to say, which leaves the agent's built-in defaults standing, while a
// present one with Enabled=false is an explicit "do not trace".
func diagPolicyToProto(d config.DiagPolicy) *pb.DiagPolicy {
	return &pb.DiagPolicy{
		Enabled:             d.Enabled,
		ConsecutiveFailures: int32(d.ConsecutiveFailures),
		CooldownSeconds:     int32(d.CooldownSeconds),
		MaxHops:             int32(d.MaxHops),
		Attempts:            int32(d.Attempts),
		PerHopTimeoutMs:     int32(d.PerHopTimeoutMs),
		BudgetMs:            int32(d.BudgetMs),
		Serial:              d.Serial,
	}
}

func diagPolicyFromProto(d *pb.DiagPolicy) config.DiagPolicy {
	if d == nil {
		return config.DiagPolicy{}
	}
	return config.DiagPolicy{
		Enabled:             d.Enabled,
		ConsecutiveFailures: int(d.ConsecutiveFailures),
		CooldownSeconds:     int(d.CooldownSeconds),
		MaxHops:             int(d.MaxHops),
		Attempts:            int(d.Attempts),
		PerHopTimeoutMs:     int(d.PerHopTimeoutMs),
		BudgetMs:            int(d.BudgetMs),
		Serial:              d.Serial,
	}
}

// gameConfigToProto mirrors the game block. Presence carries the meaning at this
// level too: a nil Game leaves the field absent, which is a different fact from
// a present block that happens to hold no profiles.
func gameConfigToProto(g config.GameConfig) *pb.GameConfig {
	out := &pb.GameConfig{
		Version:         int32(g.Version),
		RecordUnmatched: g.RecordUnmatched,
	}
	if len(g.Profiles) > 0 {
		out.Profiles = make([]*pb.GameProfile, len(g.Profiles))
		for i, p := range g.Profiles {
			out.Profiles[i] = &pb.GameProfile{
				Id:        p.ID,
				Name:      p.Name,
				Exe:       p.Exe,
				TargetFps: int32(p.TargetFPS),
				Tier:      p.Tier,
			}
		}
	}
	return out
}

func gameConfigFromProto(g *pb.GameConfig) config.GameConfig {
	out := config.GameConfig{
		Version:         int(g.Version),
		RecordUnmatched: g.RecordUnmatched,
	}
	if len(g.Profiles) > 0 {
		out.Profiles = make([]config.GameProfile, len(g.Profiles))
		for i, p := range g.Profiles {
			out.Profiles[i] = config.GameProfile{
				ID:        p.Id,
				Name:      p.Name,
				Exe:       p.Exe,
				TargetFPS: int(p.TargetFps),
				Tier:      p.Tier,
			}
		}
	}
	return out
}

func proxySpecToProto(p config.ProxySpec) *pb.ProxySpec {
	return &pb.ProxySpec{
		Id:                 p.ID,
		Name:               p.Name,
		Type:               p.Type,
		ConfigSerial:       int32(p.ConfigSerial),
		Host:               p.Host,
		Port:               int32(p.Port),
		Username:           p.Username,
		Password:           p.Password,
		DnsMode:            p.DNSMode,
		ConnectTimeoutMs:   int32(p.ConnectTimeoutMs),
		WgPrivateKey:       p.WGPrivateKey,
		WgPeerPublicKey:    p.WGPeerPublicKey,
		WgPresharedKey:     p.WGPresharedKey,
		WgEndpoint:         p.WGEndpoint,
		WgAllowedIps:       p.WGAllowedIPs,
		WgLocalAddrs:       p.WGLocalAddrs,
		WgDns:              p.WGDNS,
		WgMtu:              int32(p.WGMTU),
		WgKeepaliveSeconds: int32(p.WGKeepaliveSeconds),
	}
}

func proxySpecFromProto(p *pb.ProxySpec) config.ProxySpec {
	if p == nil {
		return config.ProxySpec{}
	}
	return config.ProxySpec{
		ID:                 p.Id,
		Name:               p.Name,
		Type:               p.Type,
		ConfigSerial:       int(p.ConfigSerial),
		Host:               p.Host,
		Port:               int(p.Port),
		Username:           p.Username,
		Password:           p.Password,
		DNSMode:            p.DnsMode,
		ConnectTimeoutMs:   int(p.ConnectTimeoutMs),
		WGPrivateKey:       p.WgPrivateKey,
		WGPeerPublicKey:    p.WgPeerPublicKey,
		WGPresharedKey:     p.WgPresharedKey,
		WGEndpoint:         p.WgEndpoint,
		WGAllowedIPs:       p.WgAllowedIps,
		WGLocalAddrs:       p.WgLocalAddrs,
		WGDNS:              p.WgDns,
		WGMTU:              int(p.WgMtu),
		WGKeepaliveSeconds: int(p.WgKeepaliveSeconds),
	}
}

func desiredStateFromProto(d *pb.DesiredState) config.DesiredState {
	out := config.DesiredState{
		ConfigVersion: int(d.ConfigVersion),
	}
	if d.Intervals != nil {
		out.Intervals = config.Intervals{
			BaseSeconds:    int(d.Intervals.BaseSeconds),
			RegularSeconds: int(d.Intervals.RegularSeconds),
		}
	}
	if len(d.ProbeTargets) > 0 {
		out.ProbeTargets = make([]config.ProbeTarget, len(d.ProbeTargets))
		for i, t := range d.ProbeTargets {
			pt := config.ProbeTarget{
				MonitorID:    t.MonitorId,
				Kind:         t.Kind,
				Name:         t.Name,
				Target:       t.Target,
				ConfigSerial: int(t.ConfigSerial),
				ProxyID:      t.ProxyId,
			}
			if t.Params != nil {
				pt.Params = config.ProbeParams{
					IntervalSeconds:  int(t.Params.IntervalSeconds),
					TimeoutMs:        int(t.Params.TimeoutMs),
					PacketSize:       int(t.Params.PacketSize),
					PacketCount:      int(t.Params.PacketCount),
					GlobalTimeoutMs:  int(t.Params.GlobalTimeoutMs),
					RecordType:       t.Params.RecordType,
					ResolverServer:   t.Params.ResolverServer,
					ResolverPort:     int(t.Params.ResolverPort),
					ResolverProtocol: t.Params.ResolverProtocol,
					Method:           t.Params.Method,
					AcceptedStatuses: t.Params.AcceptedStatuses,
					Keyword:          t.Params.Keyword,
					KeywordInvert:    t.Params.KeywordInvert,
					Headers:          t.Params.Headers,
					Body:             t.Params.Body,
					MaxRedirects:     int(t.Params.MaxRedirects),
					IgnoreTLS:        t.Params.IgnoreTls,
					MaxResponseBytes: int(t.Params.MaxResponseBytes),
					Port:             int(t.Params.Port),
					TLS:              t.Params.Tls,
					NATTransport:     t.Params.NatTransport,
					STUNServer2:      t.Params.StunServer2,
					Interface:        t.Params.Interface,
				}
			}
			out.ProbeTargets[i] = pt
		}
	}
	if len(d.Proxies) > 0 {
		out.Proxies = make([]config.ProxySpec, len(d.Proxies))
		for i, p := range d.Proxies {
			out.Proxies[i] = proxySpecFromProto(p)
		}
	}
	if d.Game != nil {
		g := gameConfigFromProto(d.Game)
		out.Game = &g
	}
	if d.Diag != nil {
		dp := diagPolicyFromProto(d.Diag)
		out.Diag = &dp
	}
	return out
}

// ---- SnapshotRequest (standalone push frame) ----

func snapshotRequestToProto(r config.SnapshotRequest) *pb.SnapshotRequest {
	return &pb.SnapshotRequest{
		RequestId: r.RequestID,
		Scopes:    r.Scopes,
	}
}

func snapshotRequestFromProto(r *pb.SnapshotRequest) config.SnapshotRequest {
	if r == nil {
		return config.SnapshotRequest{}
	}
	return config.SnapshotRequest{
		RequestID: r.RequestId,
		Scopes:    r.Scopes,
	}
}

// ---- SceneReport (agent-triggered incident scene, rides Packet) ----

func sceneReportToProto(s telemetry.SceneReport) *pb.SceneReport {
	out := &pb.SceneReport{
		ReportId:    s.ReportID,
		CollectedAt: tsToProto(s.CollectedAt),
		Network:     snapshotNetworkToProto(s.Network),
		Agent:       snapshotAgentInfoToProto(s.Agent),
		Resources:   snapshotResourcesToProto(s.Resources),
	}
	if len(s.Triggers) > 0 {
		out.Triggers = make([]*pb.SceneTrigger, len(s.Triggers))
		for i, t := range s.Triggers {
			out.Triggers[i] = &pb.SceneTrigger{
				Kind:           t.Kind,
				MonitorId:      t.MonitorID,
				ConfigSerial:   int32(t.ConfigSerial),
				TriggerStreak:  int32(t.TriggerStreak),
				FirstFailedAt:  tsToProto(t.FirstFailedAt),
				DisconnectedAt: tsToProto(t.DisconnectedAt),
				Reason:         t.Reason,
				EdgeCount:      int32(t.EdgeCount),
			}
		}
	}
	if len(s.Groups) > 0 {
		out.Groups = make([]*pb.SnapshotGroupResult, len(s.Groups))
		for i, g := range s.Groups {
			out.Groups[i] = &pb.SnapshotGroupResult{
				Group:       g.Group,
				Status:      g.Status,
				Reason:      g.Reason,
				CollectedAt: tsToProto(g.CollectedAt),
			}
		}
	}
	if len(s.Targets) > 0 {
		out.Targets = make([]*pb.SnapshotTargetResult, len(s.Targets))
		for i, t := range s.Targets {
			out.Targets[i] = &pb.SnapshotTargetResult{
				MonitorId:   t.MonitorID,
				Kind:        t.Kind,
				Target:      t.Target,
				ResolvedIps: t.ResolvedIPs,
				Endpoints:   t.Endpoints,
				ErrorClass:  t.ErrorClass,
			}
		}
	}
	return out
}

func sceneReportFromProto(s *pb.SceneReport) telemetry.SceneReport {
	if s == nil {
		return telemetry.SceneReport{}
	}
	out := telemetry.SceneReport{
		ReportID:    s.ReportId,
		CollectedAt: tsFromProto(s.CollectedAt),
		Network:     snapshotNetworkFromProto(s.Network),
		Agent:       snapshotAgentInfoFromProto(s.Agent),
		Resources:   snapshotResourcesFromProto(s.Resources),
	}
	if len(s.Triggers) > 0 {
		out.Triggers = make([]telemetry.SceneTrigger, len(s.Triggers))
		for i, t := range s.Triggers {
			out.Triggers[i] = telemetry.SceneTrigger{
				Kind:           t.Kind,
				MonitorID:      t.MonitorId,
				ConfigSerial:   int(t.ConfigSerial),
				TriggerStreak:  int(t.TriggerStreak),
				FirstFailedAt:  tsFromProto(t.FirstFailedAt),
				DisconnectedAt: tsFromProto(t.DisconnectedAt),
				Reason:         t.Reason,
				EdgeCount:      int(t.EdgeCount),
			}
		}
	}
	if len(s.Groups) > 0 {
		out.Groups = make([]telemetry.SnapshotGroupResult, len(s.Groups))
		for i, g := range s.Groups {
			out.Groups[i] = telemetry.SnapshotGroupResult{
				Group:       g.Group,
				Status:      g.Status,
				Reason:      g.Reason,
				CollectedAt: tsFromProto(g.CollectedAt),
			}
		}
	}
	if len(s.Targets) > 0 {
		out.Targets = make([]telemetry.SnapshotTargetResult, len(s.Targets))
		for i, t := range s.Targets {
			out.Targets[i] = telemetry.SnapshotTargetResult{
				MonitorID:   t.MonitorId,
				Kind:        t.Kind,
				Target:      t.Target,
				ResolvedIPs: t.ResolvedIps,
				Endpoints:   t.Endpoints,
				ErrorClass:  t.ErrorClass,
			}
		}
	}
	return out
}

func snapshotNetworkToProto(n *telemetry.SnapshotNetwork) *pb.SnapshotNetwork {
	if n == nil {
		return nil
	}
	out := &pb.SnapshotNetwork{DnsServers: n.DNSServers}
	if len(n.Interfaces) > 0 {
		out.Interfaces = make([]*pb.SnapshotInterface, len(n.Interfaces))
		for i, ifc := range n.Interfaces {
			out.Interfaces[i] = &pb.SnapshotInterface{
				Name:       ifc.Name,
				Addrs:      ifc.Addrs,
				Up:         ifc.Up,
				IsWireless: ifc.IsWireless,
			}
		}
	}
	if n.DefaultRoute != nil {
		out.DefaultRoute = &pb.SnapshotRoute{
			Gateway:   n.DefaultRoute.Gateway,
			Interface: n.DefaultRoute.Interface,
		}
	}
	return out
}

func snapshotNetworkFromProto(n *pb.SnapshotNetwork) *telemetry.SnapshotNetwork {
	if n == nil {
		return nil
	}
	out := &telemetry.SnapshotNetwork{DNSServers: n.DnsServers}
	if len(n.Interfaces) > 0 {
		out.Interfaces = make([]telemetry.SnapshotInterface, len(n.Interfaces))
		for i, ifc := range n.Interfaces {
			out.Interfaces[i] = telemetry.SnapshotInterface{
				Name:       ifc.Name,
				Addrs:      ifc.Addrs,
				Up:         ifc.Up,
				IsWireless: ifc.IsWireless,
			}
		}
	}
	if n.DefaultRoute != nil {
		out.DefaultRoute = &telemetry.SnapshotRoute{
			Gateway:   n.DefaultRoute.Gateway,
			Interface: n.DefaultRoute.Interface,
		}
	}
	return out
}

func snapshotAgentInfoToProto(a *telemetry.SnapshotAgentInfo) *pb.SnapshotAgentInfo {
	if a == nil {
		return nil
	}
	return &pb.SnapshotAgentInfo{
		AgentId:      a.AgentID,
		Hostname:     a.Hostname,
		Platform:     a.Platform,
		AgentVersion: a.AgentVersion,
	}
}

func snapshotAgentInfoFromProto(a *pb.SnapshotAgentInfo) *telemetry.SnapshotAgentInfo {
	if a == nil {
		return nil
	}
	return &telemetry.SnapshotAgentInfo{
		AgentID:      a.AgentId,
		Hostname:     a.Hostname,
		Platform:     a.Platform,
		AgentVersion: a.AgentVersion,
	}
}

// snapshotResources pointer fields (*float64/*uint64) map directly to the
// generated proto3-optional pointers, so nil (unreadable) stays nil.
func snapshotResourcesToProto(r *telemetry.SnapshotResources) *pb.SnapshotResources {
	if r == nil {
		return nil
	}
	return &pb.SnapshotResources{
		CpuPercent:       r.CPUPercent,
		MemoryTotalBytes: r.MemoryTotalBytes,
		MemoryUsedBytes:  r.MemoryUsedBytes,
	}
}

func snapshotResourcesFromProto(r *pb.SnapshotResources) *telemetry.SnapshotResources {
	if r == nil {
		return nil
	}
	return &telemetry.SnapshotResources{
		CPUPercent:       r.CpuPercent,
		MemoryTotalBytes: r.MemoryTotalBytes,
		MemoryUsedBytes:  r.MemoryUsedBytes,
	}
}

// ---- TraceResult (agent->server, carried inside Packet) ----

func traceResultToProto(r telemetry.TraceResult) *pb.TraceResult {
	out := &pb.TraceResult{
		ReportId:      r.ReportID,
		Mode:          r.Mode,
		Status:        r.Status,
		Reason:        r.Reason,
		DestinationIp: r.DestinationIP,
		Reached:       r.Reached,
		ReachedTtl:    int32(r.ReachedTTL),
		StartedAt:     tsToProto(r.StartedAt),
		CompletedAt:   tsToProto(r.CompletedAt),

		PathScope:          r.PathScope,
		EgressProxyId:      r.EgressProxyID,
		EgressConfigSerial: int32(r.EgressConfigSerial),

		DestKey:       r.DestKey,
		DestHost:      r.DestHost,
		Port:          int32(r.Port),
		SubjectKind:   r.SubjectKind,
		SubjectReason: r.SubjectReason,

		FallbackFrom:   r.FallbackFrom,
		FallbackReason: r.FallbackReason,

		TriggerReason: r.TriggerReason,
		TriggerStreak: int32(r.TriggerStreak),
		FirstFailedAt: tsToProto(r.FirstFailedAt),

		MaxHops:        int32(r.MaxHops),
		AttemptsPerHop: int32(r.AttemptsPerHop),
	}
	if len(r.Hops) > 0 {
		out.Hops = make([]*pb.TraceHop, len(r.Hops))
		for i, h := range r.Hops {
			hop := &pb.TraceHop{Ttl: int32(h.TTL)}
			if len(h.Attempts) > 0 {
				hop.Attempts = make([]*pb.TraceAttempt, len(h.Attempts))
				for j, a := range h.Attempts {
					hop.Attempts[j] = &pb.TraceAttempt{
						ResponderAddr: a.ResponderAddr,
						Hostname:      a.Hostname,
						RttMs:         a.RTTMs,
						Timeout:       a.Timeout,
					}
				}
			}
			out.Hops[i] = hop
		}
	}
	return out
}

func traceResultFromProto(r *pb.TraceResult) telemetry.TraceResult {
	if r == nil {
		return telemetry.TraceResult{}
	}
	out := telemetry.TraceResult{
		ReportID:      r.ReportId,
		Mode:          r.Mode,
		Status:        r.Status,
		Reason:        r.Reason,
		DestinationIP: r.DestinationIp,
		Reached:       r.Reached,
		ReachedTTL:    int(r.ReachedTtl),
		StartedAt:     tsFromProto(r.StartedAt),
		CompletedAt:   tsFromProto(r.CompletedAt),

		PathScope:          r.PathScope,
		EgressProxyID:      r.EgressProxyId,
		EgressConfigSerial: int(r.EgressConfigSerial),

		DestKey:       r.DestKey,
		DestHost:      r.DestHost,
		Port:          int(r.Port),
		SubjectKind:   r.SubjectKind,
		SubjectReason: r.SubjectReason,

		FallbackFrom:   r.FallbackFrom,
		FallbackReason: r.FallbackReason,

		TriggerReason: r.TriggerReason,
		TriggerStreak: int(r.TriggerStreak),
		FirstFailedAt: tsFromProto(r.FirstFailedAt),

		MaxHops:        int(r.MaxHops),
		AttemptsPerHop: int(r.AttemptsPerHop),
	}
	if len(r.Hops) > 0 {
		out.Hops = make([]telemetry.TraceHop, len(r.Hops))
		for i, h := range r.Hops {
			hop := telemetry.TraceHop{TTL: int(h.Ttl)}
			if len(h.Attempts) > 0 {
				hop.Attempts = make([]telemetry.TraceAttempt, len(h.Attempts))
				for j, a := range h.Attempts {
					hop.Attempts[j] = telemetry.TraceAttempt{
						ResponderAddr: a.ResponderAddr,
						Hostname:      a.Hostname,
						RTTMs:         a.RttMs,
						Timeout:       a.Timeout,
					}
				}
			}
			out.Hops[i] = hop
		}
	}
	return out
}

// ---- Hello / Frame ----

func helloToProto(h Hello) *pb.Hello {
	return &pb.Hello{
		SchemaVersion: int32(h.SchemaVersion),
		Hostname:      h.Hostname,
		Platform:      h.Platform,
		AgentVersion:  h.AgentVersion,
		Permissions:   permissionReportToProto(h.Permissions),
	}
}

func helloFromProto(h *pb.Hello) Hello {
	if h == nil {
		return Hello{}
	}
	return Hello{
		SchemaVersion: int(h.SchemaVersion),
		Hostname:      h.Hostname,
		Platform:      h.Platform,
		AgentVersion:  h.AgentVersion,
		Permissions:   permissionReportFromProto(h.Permissions),
	}
}

// frameToProto assumes the Frame carries exactly one payload (validated by
// MarshalFrame).
func frameToProto(f Frame) *pb.Frame {
	out := &pb.Frame{}
	switch {
	case f.Hello != nil:
		out.Msg = &pb.Frame_Hello{Hello: helloToProto(*f.Hello)}
	case f.Packet != nil:
		out.Msg = &pb.Frame_Packet{Packet: packetToProto(*f.Packet)}
	case f.HostSnapshot != nil:
		out.Msg = &pb.Frame_HostSnapshot{HostSnapshot: hostSnapshotToProto(*f.HostSnapshot)}
	case f.MonitorStatus != nil:
		out.Msg = &pb.Frame_MonitorStatus{MonitorStatus: monitorStatusToProto(*f.MonitorStatus)}
	case f.Ack != nil:
		out.Msg = &pb.Frame_Ack{Ack: ackToProto(*f.Ack)}
	case f.DesiredState != nil:
		out.Msg = &pb.Frame_DesiredState{DesiredState: desiredStateToProto(*f.DesiredState)}
	case f.SnapshotRequest != nil:
		out.Msg = &pb.Frame_SnapshotRequest{SnapshotRequest: snapshotRequestToProto(*f.SnapshotRequest)}
	}
	return out
}

func frameFromProto(f *pb.Frame) Frame {
	if f == nil {
		return Frame{}
	}
	var out Frame
	switch m := f.Msg.(type) {
	case *pb.Frame_Hello:
		h := helloFromProto(m.Hello)
		out.Hello = &h
	case *pb.Frame_Packet:
		p := packetFromProto(m.Packet)
		out.Packet = &p
	case *pb.Frame_HostSnapshot:
		hs := hostSnapshotFromProto(m.HostSnapshot)
		out.HostSnapshot = &hs
	case *pb.Frame_MonitorStatus:
		ms := monitorStatusFromProto(m.MonitorStatus)
		out.MonitorStatus = &ms
	case *pb.Frame_Ack:
		a := ackFromProto(m.Ack)
		out.Ack = &a
	case *pb.Frame_DesiredState:
		ds := desiredStateFromProto(m.DesiredState)
		out.DesiredState = &ds
	case *pb.Frame_SnapshotRequest:
		sr := snapshotRequestFromProto(m.SnapshotRequest)
		out.SnapshotRequest = &sr
	}
	return out
}

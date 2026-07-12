package wire

import (
	"time"

	"github.com/nettact/protocol/config"
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

// ---- Packet ----

func packetToProto(p telemetry.Packet) *pb.Packet {
	out := &pb.Packet{
		SchemaVersion:         int32(p.SchemaVersion),
		AgentId:               p.AgentID,
		SiteId:                p.SiteID,
		Sequence:              p.Sequence,
		SentAt:                tsToProto(p.SentAt),
		ReportedConfigVersion: int32(p.ReportedConfigVersion),
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
	return out
}

func packetFromProto(p *pb.Packet) telemetry.Packet {
	if p == nil {
		return telemetry.Packet{}
	}
	out := telemetry.Packet{
		SchemaVersion:         int(p.SchemaVersion),
		AgentID:               p.AgentId,
		SiteID:                p.SiteId,
		Sequence:              p.Sequence,
		SentAt:                tsFromProto(p.SentAt),
		ReportedConfigVersion: int(p.ReportedConfigVersion),
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
	return out
}

// ---- Metric ----

func metricToProto(m telemetry.Metric) *pb.Metric {
	return &pb.Metric{
		Ts:        tsToProto(m.TS),
		Kind:      string(m.Kind),
		Target:    m.Target,
		Layer:     string(m.Layer),
		Value:     m.Value,
		Unit:      m.Unit,
		Labels:    m.Labels,
		MonitorId: m.MonitorID,
	}
}

func metricFromProto(m *pb.Metric) telemetry.Metric {
	return telemetry.Metric{
		TS:        tsFromProto(m.Ts),
		Kind:      telemetry.MetricKind(m.Kind),
		Target:    m.Target,
		Layer:     telemetry.HealthLayer(m.Layer),
		Value:     m.Value,
		Unit:      m.Unit,
		Labels:    m.Labels,
		MonitorID: m.MonitorId,
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

// ---- InventoryItem ----

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
		Name:     it.Name,
		Addrs:    it.Addrs,
		Gateway:  it.Gateway,
		Dns:      it.DNS,
		Up:       it.Up,
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
		Name:     it.Name,
		Addrs:    it.Addrs,
		Gateway:  it.Gateway,
		DNS:      it.Dns,
		Up:       it.Up,
	}
}

// ---- HostSnapshot ----

func hostSnapshotToProto(h telemetry.HostSnapshot) *pb.HostSnapshot {
	out := &pb.HostSnapshot{
		Ts:           tsToProto(h.TS),
		RequestId:    h.RequestID,
		ProcessTotal: int32(h.ProcessTotal),
	}
	if len(h.Processes) > 0 {
		out.Processes = make([]*pb.ProcessInfo, len(h.Processes))
		for i, p := range h.Processes {
			out.Processes[i] = &pb.ProcessInfo{
				Pid:            p.PID,
				Name:           p.Name,
				User:           p.User,
				Status:         p.Status,
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
				LocalAddr:   c.LocalAddr,
				RemoteAddr:  c.RemoteAddr,
				State:       c.State,
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
		ProcessTotal: int(h.ProcessTotal),
	}
	if len(h.Processes) > 0 {
		out.Processes = make([]telemetry.ProcessInfo, len(h.Processes))
		for i, p := range h.Processes {
			out.Processes[i] = telemetry.ProcessInfo{
				PID:            p.Pid,
				Name:           p.Name,
				User:           p.User,
				Status:         p.Status,
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
				LocalAddr:   c.LocalAddr,
				RemoteAddr:  c.RemoteAddr,
				State:       c.State,
				PID:         c.Pid,
				ProcessName: c.ProcessName,
			}
		}
	}
	return out
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
				MonitorId: t.MonitorID,
				Kind:      t.Kind,
				Name:      t.Name,
				Target:    t.Target,
				Params: &pb.ProbeParams{
					IntervalSeconds:  int32(t.Params.IntervalSeconds),
					TimeoutMs:        int32(t.Params.TimeoutMs),
					PacketSize:       int32(t.Params.PacketSize),
					Retries:          int32(t.Params.Retries),
					PacketCount:      int32(t.Params.PacketCount),
					GlobalTimeoutMs:  int32(t.Params.GlobalTimeoutMs),
					RecordType:       t.Params.RecordType,
					ResolverServer:   t.Params.ResolverServer,
					ResolverPort:     int32(t.Params.ResolverPort),
					ResolverProtocol: t.Params.ResolverProtocol,
					Method:           t.Params.Method,
					ExpectedStatus:   int32(t.Params.ExpectedStatus),
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
				},
			}
		}
	}
	return out
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
				MonitorID: t.MonitorId,
				Kind:      t.Kind,
				Name:      t.Name,
				Target:    t.Target,
			}
			if t.Params != nil {
				pt.Params = config.ProbeParams{
					IntervalSeconds:  int(t.Params.IntervalSeconds),
					TimeoutMs:        int(t.Params.TimeoutMs),
					PacketSize:       int(t.Params.PacketSize),
					Retries:          int(t.Params.Retries),
					PacketCount:      int(t.Params.PacketCount),
					GlobalTimeoutMs:  int(t.Params.GlobalTimeoutMs),
					RecordType:       t.Params.RecordType,
					ResolverServer:   t.Params.ResolverServer,
					ResolverPort:     int(t.Params.ResolverPort),
					ResolverProtocol: t.Params.ResolverProtocol,
					Method:           t.Params.Method,
					ExpectedStatus:   int(t.Params.ExpectedStatus),
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
				}
			}
			out.ProbeTargets[i] = pt
		}
	}
	return out
}

// ---- SnapshotRequest (standalone push frame) ----

func snapshotRequestToProto(r config.SnapshotRequest) *pb.SnapshotRequest {
	return &pb.SnapshotRequest{
		RequestId:       r.RequestID,
		WantProcesses:   r.WantProcesses,
		WantConnections: r.WantConnections,
	}
}

func snapshotRequestFromProto(r *pb.SnapshotRequest) config.SnapshotRequest {
	if r == nil {
		return config.SnapshotRequest{}
	}
	return config.SnapshotRequest{
		RequestID:       r.RequestId,
		WantProcesses:   r.WantProcesses,
		WantConnections: r.WantConnections,
	}
}

// ---- Hello / Frame ----

func helloToProto(h Hello) *pb.Hello {
	return &pb.Hello{
		SchemaVersion:         int32(h.SchemaVersion),
		Hostname:              h.Hostname,
		Platform:              h.Platform,
		AgentVersion:          h.AgentVersion,
		Capabilities:          h.Capabilities,
		ReportedConfigVersion: int32(h.ReportedConfigVersion),
	}
}

func helloFromProto(h *pb.Hello) Hello {
	if h == nil {
		return Hello{}
	}
	return Hello{
		SchemaVersion:         int(h.SchemaVersion),
		Hostname:              h.Hostname,
		Platform:              h.Platform,
		AgentVersion:          h.AgentVersion,
		Capabilities:          h.Capabilities,
		ReportedConfigVersion: int(h.ReportedConfigVersion),
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

package wire

import (
	"encoding/json"
	"errors"

	"github.com/nettact/protocol/config"
	"github.com/nettact/protocol/permission"
	"github.com/nettact/protocol/telemetry"
	"github.com/nettact/protocol/wire/pb"
	"google.golang.org/protobuf/proto"
)

// Hello is the first frame an agent sends after the WebSocket upgrade. It
// replaces the per-request X-Agent-* headers of the old POST transport and
// carries the config watermark so the server knows what to push on connect.
// Permissions is the agent's authoritative supported/granted/effective view,
// refreshed on every (re)connect.
type Hello struct {
	SchemaVersion         int                         `json:"schema_version"`
	Hostname              string                      `json:"hostname"`
	Platform              string                      `json:"platform"`
	AgentVersion          string                      `json:"agent_version"`
	Permissions           permission.PermissionReport `json:"permissions"`
	ReportedConfigVersion int                         `json:"reported_config_version"`
}

// MonitorStatus is the agent's full-state report of how it evaluated every
// pushed monitor against its effective permissions and target-access policy. It
// is always the complete set for the given ConfigVersion (never a delta): the
// server upserts these rows and deletes any monitor absent from the frame. The
// agent sends it after every DesiredState apply and on any runtime transition
// (DNS flip, redirect block, recovery), coalesced latest-wins.
type MonitorStatus struct {
	ConfigVersion int                  `json:"config_version"`
	PolicyHash    string               `json:"policy_hash"`
	Statuses      []MonitorStatusEntry `json:"statuses"`
}

// MonitorStatusEntry is one monitor's execution status.
type MonitorStatusEntry struct {
	MonitorID          string   `json:"monitor_id"`
	Status             string   `json:"status"` // active | permission_blocked | target_blocked | unsupported
	MissingPermissions []string `json:"missing_permissions,omitempty"`
	MatchedSelector    string   `json:"matched_selector,omitempty"`
	Reason             string   `json:"reason,omitempty"` // literal_denied | resolved_denied | redirect_denied | method_requires_extended | …
}

// Monitor execution status values (MonitorStatusEntry.Status).
const (
	MonitorStatusActive            = "active"
	MonitorStatusPermissionBlocked = "permission_blocked"
	MonitorStatusTargetBlocked     = "target_blocked"
	MonitorStatusUnsupported       = "unsupported"
)

// Frame is the envelope for every message on the agent <-> server WebSocket.
// Exactly one field is non-nil; MarshalFrame/UnmarshalFrame enforce this.
type Frame struct {
	// agent -> server
	Hello            *Hello                      `json:"hello,omitempty"`
	Packet           *telemetry.Packet           `json:"packet,omitempty"`
	HostSnapshot     *telemetry.HostSnapshot     `json:"host_snapshot,omitempty"`
	MonitorStatus    *MonitorStatus              `json:"monitor_status,omitempty"`
	IncidentSnapshot *telemetry.IncidentSnapshot `json:"incident_snapshot,omitempty"`
	TraceResult      *telemetry.TraceResult      `json:"trace_result,omitempty"`
	// server -> agent
	Ack                     *Ack                            `json:"ack,omitempty"`
	DesiredState            *config.DesiredState            `json:"desired_state,omitempty"`
	SnapshotRequest         *config.SnapshotRequest         `json:"snapshot_request,omitempty"`
	IncidentSnapshotRequest *config.IncidentSnapshotRequest `json:"incident_snapshot_request,omitempty"`
	TraceRequest            *config.TraceRequest            `json:"trace_request,omitempty"`
}

// ErrFrameVariant reports a frame with zero or more than one payload set.
var ErrFrameVariant = errors.New("wire: frame must carry exactly one payload")

// variants returns how many payload fields are set.
func (f Frame) variants() int {
	n := 0
	for _, set := range []bool{
		f.Hello != nil, f.Packet != nil, f.HostSnapshot != nil, f.MonitorStatus != nil,
		f.IncidentSnapshot != nil, f.TraceResult != nil,
		f.Ack != nil, f.DesiredState != nil, f.SnapshotRequest != nil,
		f.IncidentSnapshotRequest != nil, f.TraceRequest != nil,
	} {
		if set {
			n++
		}
	}
	return n
}

// Validate reports ErrFrameVariant unless exactly one payload field is set. It
// is the invariant MarshalFrame/UnmarshalFrame enforce at the socket boundary,
// exported so the in-memory Pipe transport (which never serializes) can enforce
// the same contract on send.
func (f Frame) Validate() error {
	if f.variants() != 1 {
		return ErrFrameVariant
	}
	return nil
}

// MarshalFrame encodes a Frame in the format named by contentType (canonical
// constant, raw header value, or a value from SubprotocolContentType).
func MarshalFrame(f Frame, contentType string) ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	if Negotiate(contentType) == ContentTypeProtobuf {
		return proto.Marshal(frameToProto(f))
	}
	return json.Marshal(f)
}

// UnmarshalFrame decodes bytes produced by MarshalFrame for the given format.
func UnmarshalFrame(data []byte, contentType string) (Frame, error) {
	var f Frame
	if Negotiate(contentType) == ContentTypeProtobuf {
		var m pb.Frame
		if err := proto.Unmarshal(data, &m); err != nil {
			return Frame{}, err
		}
		f = frameFromProto(&m)
	} else if err := json.Unmarshal(data, &f); err != nil {
		return Frame{}, err
	}
	if err := f.Validate(); err != nil {
		return Frame{}, err
	}
	return f, nil
}

package wire

import (
	"encoding/json"
	"errors"

	"github.com/nettact/protocol/config"
	"github.com/nettact/protocol/telemetry"
	"github.com/nettact/protocol/wire/pb"
	"google.golang.org/protobuf/proto"
)

// Hello is the first frame an agent sends after the WebSocket upgrade. It
// replaces the per-request X-Agent-* headers of the old POST transport and
// carries the config watermark so the server knows what to push on connect.
type Hello struct {
	SchemaVersion         int      `json:"schema_version"`
	Hostname              string   `json:"hostname"`
	Platform              string   `json:"platform"`
	AgentVersion          string   `json:"agent_version"`
	Capabilities          []string `json:"capabilities,omitempty"`
	ReportedConfigVersion int      `json:"reported_config_version"`
}

// Frame is the envelope for every message on the agent <-> server WebSocket.
// Exactly one field is non-nil; MarshalFrame/UnmarshalFrame enforce this.
type Frame struct {
	// agent -> server
	Hello        *Hello                  `json:"hello,omitempty"`
	Packet       *telemetry.Packet       `json:"packet,omitempty"`
	HostSnapshot *telemetry.HostSnapshot `json:"host_snapshot,omitempty"`
	// server -> agent
	Ack             *Ack                    `json:"ack,omitempty"`
	DesiredState    *config.DesiredState    `json:"desired_state,omitempty"`
	SnapshotRequest *config.SnapshotRequest `json:"snapshot_request,omitempty"`
}

// ErrFrameVariant reports a frame with zero or more than one payload set.
var ErrFrameVariant = errors.New("wire: frame must carry exactly one payload")

// variants returns how many payload fields are set.
func (f Frame) variants() int {
	n := 0
	for _, set := range []bool{
		f.Hello != nil, f.Packet != nil, f.HostSnapshot != nil,
		f.Ack != nil, f.DesiredState != nil, f.SnapshotRequest != nil,
	} {
		if set {
			n++
		}
	}
	return n
}

// MarshalFrame encodes a Frame in the format named by contentType (canonical
// constant, raw header value, or a value from SubprotocolContentType).
func MarshalFrame(f Frame, contentType string) ([]byte, error) {
	if f.variants() != 1 {
		return nil, ErrFrameVariant
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
	if f.variants() != 1 {
		return Frame{}, ErrFrameVariant
	}
	return f, nil
}

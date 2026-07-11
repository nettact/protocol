// Package wire is the opt-in serialization codec for the NetTact telemetry hop.
// It offers a compact protobuf encoding alongside the existing JSON one for the
// agent -> server telemetry Packet and its ack (config downlink), selected per
// request via HTTP Content-Type / Accept.
//
// Unlike the rest of the protocol module (which is deliberately stdlib-only),
// this package imports google.golang.org/protobuf. Consumers that stay on JSON
// need not import it — the protocol/telemetry, protocol/config, protocol/enroll
// and protocol/capability type packages remain dependency-free.
package wire

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/nettact/protocol/config"
	"github.com/nettact/protocol/telemetry"
	"github.com/nettact/protocol/wire/pb"
	"google.golang.org/protobuf/proto"
)

// Canonical content-type values for the two supported wire formats.
const (
	ContentTypeJSON     = "application/json"
	ContentTypeProtobuf = "application/x-protobuf"
)

// Ack is the server -> agent telemetry response (sequence watermark + optional
// config downlink). It carries the same fields and JSON tags as the agent's
// uploader.Ack and server-core's telemetryResponse, so the JSON branch is
// byte-compatible with the pre-protobuf format.
type Ack struct {
	HighestSequence uint64               `json:"highest_sequence"`
	ServerTime      time.Time            `json:"server_time"`
	ConfigVersion   int                  `json:"config_version"`
	DesiredState    *config.DesiredState `json:"desired_state,omitempty"`
}

// Negotiate maps a raw HTTP Content-Type or Accept header value to a canonical
// content-type. Protobuf is chosen when the header advertises it with a non-zero
// quality; everything else (including an empty header, or protobuf explicitly
// rejected via q=0) falls back to JSON, preserving compatibility with agents and
// servers that predate protobuf support. A single Content-Type value (no q) is a
// media range with the default q=1, so this also handles the request-encoding case.
func Negotiate(header string) string {
	for _, part := range strings.Split(header, ",") {
		mediaType, q := parseMediaRange(part)
		if q > 0 && strings.Contains(mediaType, "protobuf") {
			return ContentTypeProtobuf
		}
	}
	return ContentTypeJSON
}

// parseMediaRange splits one comma-separated Accept/Content-Type entry into its
// lowercased media type and quality value (RFC 7231 q-param; default 1.0).
func parseMediaRange(s string) (mediaType string, q float64) {
	q = 1.0
	segs := strings.Split(s, ";")
	mediaType = strings.ToLower(strings.TrimSpace(segs[0]))
	for _, p := range segs[1:] {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(strings.ToLower(p), "q=") {
			if v, err := strconv.ParseFloat(strings.TrimSpace(p[2:]), 64); err == nil {
				q = v
			}
		}
	}
	return mediaType, q
}

// MarshalPacket encodes a telemetry.Packet in the format named by contentType
// (raw header value or canonical constant; anything non-protobuf is JSON).
func MarshalPacket(p telemetry.Packet, contentType string) ([]byte, error) {
	if Negotiate(contentType) == ContentTypeProtobuf {
		return proto.Marshal(packetToProto(p))
	}
	return json.Marshal(p)
}

// UnmarshalPacket decodes bytes produced by MarshalPacket for the given format.
func UnmarshalPacket(data []byte, contentType string) (telemetry.Packet, error) {
	if Negotiate(contentType) == ContentTypeProtobuf {
		var m pb.Packet
		if err := proto.Unmarshal(data, &m); err != nil {
			return telemetry.Packet{}, err
		}
		return packetFromProto(&m), nil
	}
	var p telemetry.Packet
	if err := json.Unmarshal(data, &p); err != nil {
		return telemetry.Packet{}, err
	}
	return p, nil
}

// MarshalAck encodes a telemetry Ack in the format named by contentType.
func MarshalAck(a Ack, contentType string) ([]byte, error) {
	if Negotiate(contentType) == ContentTypeProtobuf {
		return proto.Marshal(ackToProto(a))
	}
	return json.Marshal(a)
}

// UnmarshalAck decodes bytes produced by MarshalAck for the given format.
func UnmarshalAck(data []byte, contentType string) (Ack, error) {
	if Negotiate(contentType) == ContentTypeProtobuf {
		var m pb.TelemetryAck
		if err := proto.Unmarshal(data, &m); err != nil {
			return Ack{}, err
		}
		return ackFromProto(&m), nil
	}
	var a Ack
	if err := json.Unmarshal(data, &a); err != nil {
		return Ack{}, err
	}
	return a, nil
}

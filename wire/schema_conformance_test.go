package wire

// Schema conformance to root adr/0003 (Agent wire schema 8 与 N/N-1 兼容窗口).
//
// WS-3 says the enforcer must be a machine rather than memory: `.proto` reserved
// statements make protoc reject reuse at compile time, and G-WIRE's no-drift check
// backstops it. Neither of those, however, checks that the field numbers and
// reserved sets still match the ones the ADR froze — until this file, that was
// enforced only by review. W2-01's exit evidence requires per-item agreement with
// WS-1..WS-5, so it is asserted here.
//
// These assertions run against the *generated descriptors*, not the .proto text,
// because the generated code is what ships; a .proto edit that failed to regenerate
// would slip past a text-level check.
//
// Per the ADR's own rule: where this file and the ADR disagree, the code wins and
// the ADR is amended as a defect. A failure here means the two have diverged, which
// is exactly the condition W2-01 says to treat as a defect.

import (
	"testing"

	"github.com/nettact/protocol/wire/pb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// msgByName finds a message anywhere in the file, including nested definitions.
func msgByName(t *testing.T, name string) protoreflect.MessageDescriptor {
	t.Helper()
	var found protoreflect.MessageDescriptor
	var walk func(protoreflect.MessageDescriptors)
	walk = func(mds protoreflect.MessageDescriptors) {
		for i := 0; i < mds.Len(); i++ {
			md := mds.Get(i)
			if string(md.Name()) == name {
				found = md
			}
			walk(md.Messages())
		}
	}
	walk(pb.File_telemetry_proto.Messages())
	if found == nil {
		t.Fatalf("message %s not found in telemetry.proto descriptors", name)
	}
	return found
}

func reservedNumbers(md protoreflect.MessageDescriptor) map[protoreflect.FieldNumber]bool {
	out := map[protoreflect.FieldNumber]bool{}
	rr := md.ReservedRanges()
	for i := 0; i < rr.Len(); i++ {
		r := rr.Get(i) // [start, end)
		for n := r[0]; n < r[1]; n++ {
			out[n] = true
		}
	}
	return out
}

func reservedNames(md protoreflect.MessageDescriptor) map[string]bool {
	out := map[string]bool{}
	rn := md.ReservedNames()
	for i := 0; i < rn.Len(); i++ {
		out[string(rn.Get(i))] = true
	}
	return out
}

// WS-1: the schema 8 increment, frozen field by field.
func TestWS1Schema8FieldNumbers(t *testing.T) {
	hello := msgByName(t, "Hello")

	caps := hello.Fields().ByName("caps")
	if caps == nil {
		t.Fatal("Hello.caps missing")
	}
	if got := caps.Number(); got != 8 {
		t.Errorf("WS-1: Hello.caps = %d, ADR froze 8", got)
	}
	if caps.Kind() != protoreflect.StringKind || caps.Cardinality() != protoreflect.Repeated {
		t.Errorf("WS-1: Hello.caps is %v %v, ADR froze repeated string", caps.Cardinality(), caps.Kind())
	}

	epoch := hello.Fields().ByName("enrollment_epoch")
	if epoch == nil {
		t.Fatal("Hello.enrollment_epoch missing")
	}
	if got := epoch.Number(); got != 9 {
		t.Errorf("WS-1: Hello.enrollment_epoch = %d, ADR froze 9", got)
	}
	if epoch.Kind() != protoreflect.Uint64Kind {
		t.Errorf("WS-1: Hello.enrollment_epoch is %v, ADR froze uint64", epoch.Kind())
	}

	// The six schema 8 control frames, in the Frame oneof.
	frame := msgByName(t, "Frame")
	for name, want := range map[string]protoreflect.FieldNumber{
		"sequence_floor":                   12,
		"sequence_floor_applied":           13,
		"epoch_rotation_challenge":         14,
		"epoch_rotation_request":           15,
		"epoch_rotation_result":            16,
		"epoch_rotation_challenge_request": 17,
	} {
		fd := frame.Fields().ByName(protoreflect.Name(name))
		if fd == nil {
			t.Errorf("WS-1: Frame.%s missing", name)
			continue
		}
		if got := fd.Number(); got != want {
			t.Errorf("WS-1: Frame.%s = %d, ADR froze %d", name, got, want)
		}
		if fd.ContainingOneof() == nil {
			t.Errorf("WS-1: Frame.%s is not in a oneof; ADR describes the Frame oneof", name)
		}
	}
}

// WS-3: reserved numbers and names are never reused.
func TestWS3ReservedNeverReused(t *testing.T) {
	hello := msgByName(t, "Hello")
	hn, hnm := reservedNumbers(hello), reservedNames(hello)
	for _, n := range []protoreflect.FieldNumber{5, 6} {
		if !hn[n] {
			t.Errorf("WS-3: Hello does not reserve %d", n)
		}
	}
	for _, s := range []string{"capabilities", "reported_config_version"} {
		if !hnm[s] {
			t.Errorf("WS-3: Hello does not reserve name %q", s)
		}
	}

	fn := reservedNumbers(msgByName(t, "Frame"))
	for n := protoreflect.FieldNumber(8); n <= 11; n++ {
		if !fn[n] {
			t.Errorf("WS-3: Frame does not reserve %d", n)
		}
	}

	// Every message the ADR names must still carry a reserved segment.
	for _, name := range []string{
		"Packet", "GameBucket", "InventoryItem", "TelemetryAck",
		"DesiredState", "SnapshotRequest", "ProbeTarget", "ProbeParams",
	} {
		md := msgByName(t, name)
		if len(reservedNumbers(md)) == 0 && len(reservedNames(md)) == 0 {
			t.Errorf("WS-3: %s carries no reserved segment, ADR lists one", name)
		}
	}
}

// WS-3, the invariant behind it: a reserved number or name must never also be live.
// protoc rejects this within one file, but the check is cheap and it states the
// invariant the ADR actually cares about rather than trusting a side effect.
func TestWS3NoReservedCollidesWithLiveField(t *testing.T) {
	var walk func(protoreflect.MessageDescriptors)
	walk = func(mds protoreflect.MessageDescriptors) {
		for i := 0; i < mds.Len(); i++ {
			md := mds.Get(i)
			rn, rnm := reservedNumbers(md), reservedNames(md)
			fields := md.Fields()
			for j := 0; j < fields.Len(); j++ {
				fd := fields.Get(j)
				if rn[fd.Number()] {
					t.Errorf("WS-3: %s.%s uses reserved number %d", md.Name(), fd.Name(), fd.Number())
				}
				if rnm[string(fd.Name())] {
					t.Errorf("WS-3: %s.%s uses reserved name", md.Name(), fd.Name())
				}
			}
			walk(md.Messages())
		}
	}
	walk(pb.File_telemetry_proto.Messages())
}

// WS-4: subprotocol selects the encoding only; version negotiation goes through
// Hello.schema_version. A subprotocol name carrying a schema version would be the
// design the ADR rejected.
func TestWS4SubprotocolIsVersionOrthogonal(t *testing.T) {
	if SubprotocolProtobuf != "nettact.v1.protobuf" || SubprotocolJSON != "nettact.v1.json" {
		t.Fatalf("WS-4: subprotocols are %q/%q", SubprotocolProtobuf, SubprotocolJSON)
	}
	if msgByName(t, "Hello").Fields().ByName("schema_version") == nil {
		t.Error("WS-4: Hello.schema_version missing; it is the version negotiation carrier")
	}
}

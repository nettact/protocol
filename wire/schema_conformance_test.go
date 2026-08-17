package wire

// Wire-schema freeze conformance.
//
// The schema-8 wire surface — field numbers, types, and reserved sets — is
// frozen. `.proto` reserved statements make protoc reject reuse at compile
// time, and the no-drift CI check keeps the generated code in sync with the
// .proto — but neither checks that the field numbers and reserved sets still
// match the frozen surface. Until this file, that was enforced only by
// review; it is asserted here so the enforcer is a machine rather than
// memory.
//
// These assertions run against the *generated descriptors*, not the .proto
// text, because the generated code is what ships; a .proto edit that failed
// to regenerate would slip past a text-level check.
//
// A failure here means the shipped schema drifted from the freeze. Treat it
// as a defect in the change that moved it — never re-number, retype, or
// un-reserve to make the test pass.

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

// The schema 8 increment, frozen field by field.
func TestSchema8FieldNumbersFrozen(t *testing.T) {
	hello := msgByName(t, "Hello")

	caps := hello.Fields().ByName("caps")
	if caps == nil {
		t.Fatal("Hello.caps missing")
	}
	if got := caps.Number(); got != 8 {
		t.Errorf("Hello.caps = %d, frozen at 8", got)
	}
	if caps.Kind() != protoreflect.StringKind || caps.Cardinality() != protoreflect.Repeated {
		t.Errorf("Hello.caps is %v %v, frozen as repeated string", caps.Cardinality(), caps.Kind())
	}

	epoch := hello.Fields().ByName("enrollment_epoch")
	if epoch == nil {
		t.Fatal("Hello.enrollment_epoch missing")
	}
	if got := epoch.Number(); got != 9 {
		t.Errorf("Hello.enrollment_epoch = %d, frozen at 9", got)
	}
	if epoch.Kind() != protoreflect.Uint64Kind {
		t.Errorf("Hello.enrollment_epoch is %v, frozen as uint64", epoch.Kind())
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
			t.Errorf("Frame.%s missing", name)
			continue
		}
		if got := fd.Number(); got != want {
			t.Errorf("Frame.%s = %d, frozen at %d", name, got, want)
		}
		if fd.ContainingOneof() == nil {
			t.Errorf("Frame.%s is not in a oneof; the schema 8 control frames live in the Frame oneof", name)
		}
	}
}

// Reserved numbers and names are never reused.
func TestReservedNeverReused(t *testing.T) {
	hello := msgByName(t, "Hello")
	hn, hnm := reservedNumbers(hello), reservedNames(hello)
	for _, n := range []protoreflect.FieldNumber{5, 6} {
		if !hn[n] {
			t.Errorf("Hello does not reserve %d", n)
		}
	}
	for _, s := range []string{"capabilities", "reported_config_version"} {
		if !hnm[s] {
			t.Errorf("Hello does not reserve name %q", s)
		}
	}

	fn := reservedNumbers(msgByName(t, "Frame"))
	for n := protoreflect.FieldNumber(8); n <= 11; n++ {
		if !fn[n] {
			t.Errorf("Frame does not reserve %d", n)
		}
	}

	// Every message that has retired fields must still carry its reserved
	// segment.
	for _, name := range []string{
		"Packet", "GameBucket", "InventoryItem", "TelemetryAck",
		"DesiredState", "SnapshotRequest", "ProbeTarget", "ProbeParams",
	} {
		md := msgByName(t, name)
		if len(reservedNumbers(md)) == 0 && len(reservedNames(md)) == 0 {
			t.Errorf("%s carries no reserved segment; it retired fields and must keep one", name)
		}
	}
}

// The invariant behind the reserved discipline: a reserved number or name must
// never also be live. protoc rejects this within one file, but the check is
// cheap and it states the invariant directly rather than trusting a side
// effect.
func TestNoReservedCollidesWithLiveField(t *testing.T) {
	var walk func(protoreflect.MessageDescriptors)
	walk = func(mds protoreflect.MessageDescriptors) {
		for i := 0; i < mds.Len(); i++ {
			md := mds.Get(i)
			rn, rnm := reservedNumbers(md), reservedNames(md)
			fields := md.Fields()
			for j := 0; j < fields.Len(); j++ {
				fd := fields.Get(j)
				if rn[fd.Number()] {
					t.Errorf("%s.%s uses reserved number %d", md.Name(), fd.Name(), fd.Number())
				}
				if rnm[string(fd.Name())] {
					t.Errorf("%s.%s uses reserved name", md.Name(), fd.Name())
				}
			}
			walk(md.Messages())
		}
	}
	walk(pb.File_telemetry_proto.Messages())
}

// Subprotocol selects the encoding only; version negotiation goes through
// Hello.schema_version. A subprotocol name carrying a schema version would
// re-open a negotiation design that was deliberately rejected.
func TestSubprotocolIsVersionOrthogonal(t *testing.T) {
	if SubprotocolProtobuf != "nettact.v1.protobuf" || SubprotocolJSON != "nettact.v1.json" {
		t.Fatalf("subprotocols are %q/%q", SubprotocolProtobuf, SubprotocolJSON)
	}
	if msgByName(t, "Hello").Fields().ByName("schema_version") == nil {
		t.Error("Hello.schema_version missing; it is the version negotiation carrier")
	}
}

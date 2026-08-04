// Package protocol defines the shared wire contract used by the agent, the
// server, and (later) the cloud server. The type packages (telemetry,
// config, enroll, permission) deliberately have no external dependencies so
// every consumer can import them without pulling in server or agent code
// (architecture §15.2-3: one shared protocol, no forks).
//
// The one exception is the opt-in protocol/wire codec, which adds protobuf as
// a compact alternative to JSON on the telemetry hop. Only consumers that
// import protocol/wire pull google.golang.org/protobuf; the type packages stay
// dependency-free.
package protocol

import "fmt"

// SchemaVersion is the current wire schema version. Bump it for breaking
// changes; additive changes (new optional fields, new string enum values) keep
// the same version and must be tolerated by consumers.
//
// 3 removed two fields from GameBucket — the whole-adapter telemetry and the
// busiest logical core — and moved them to a stream of their own keyed by
// (agent, second), alongside a new collection for the stretches a game produced
// no frames. Removals are what make it breaking: a peer still speaking 2 sends
// readings this build has no field for, and expects none of what replaced them.
const SchemaVersion = 3

// ValidateSchema reports whether v is a schema version this build understands.
// The server calls this at ingress, and the agent on the reply, so a mismatched
// pair fails loudly at the handshake instead of quietly at the data.
//
// # Why this is an exact match and not a range
//
// It used to accept 1..SchemaVersion, which is the shape a system with deployed
// agents needs. This one has none, and the project keeps no compatibility paths
// (AGENTS.md) — so a range here would not buy tolerance, it would buy silence:
// a peer speaking 2 would be admitted and every field 3 moved would be dropped
// on the floor with nothing anywhere saying so. That is the failure mode the
// version exists to prevent, and it is worse than a refused connection, which at
// least names the half of the install that needs upgrading.
func ValidateSchema(v int) error {
	if v != SchemaVersion {
		return fmt.Errorf("unsupported schema_version %d (this build speaks %d; upgrade the other side)", v, SchemaVersion)
	}
	return nil
}

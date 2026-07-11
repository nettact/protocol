// Package protocol defines the shared wire contract used by the agent,
// server-lite, and (later) the cloud server. The type packages (telemetry,
// config, enroll, capability) deliberately have no external dependencies so
// every consumer can import them without pulling in server or agent code
// (architecture §15.2-3: one shared protocol, no forks).
//
// The one exception is the opt-in protocol/wire codec, which adds protobuf as
// a compact alternative to JSON on the telemetry hop. Only consumers that
// import protocol/wire pull google.golang.org/protobuf; the type packages stay
// dependency-free.
package protocol

import "fmt"

// SchemaVersion is the current wire schema version. Bump the major only for
// breaking changes; additive changes (new optional fields, new string enum
// values) keep the same version and must be tolerated by older consumers.
const SchemaVersion = 1

// ValidateSchema reports whether v is a schema version this build understands.
// The server calls this at ingress so old servers / new agents degrade
// predictably instead of failing to decode.
func ValidateSchema(v int) error {
	if v < 1 || v > SchemaVersion {
		return fmt.Errorf("unsupported schema_version %d (supported: 1..%d)", v, SchemaVersion)
	}
	return nil
}

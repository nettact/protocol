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
//
// 4 redefined what probe.icmp.loss_pct is a ratio OF. It used to be the target's
// configured packet count; it is now the echoes the agent actually sent, which
// it reports alongside as probe.icmp.sent. The field did not move and no field
// was removed, which is exactly why this needs the version: nothing about the
// wire shape betrays the change, so a 3 server would read a 4 agent's rounds
// happily and read them wrong. It would take a round truncated by the agent's
// probe-concurrency budget — one echo of five, reporting 100% because the one
// was lost — as a full round's 100% and confirm an outage, which is the precise
// false fault the sent count was added to prevent. Bumping refuses that pairing
// at the handshake instead.
//
// 5 moved traceroute from a server-issued command to an agent-local trigger. Two
// frame variants went away (trace_request, trace_result), the result now rides
// Packet as a self-describing record, and DesiredState gained the policy block
// that governs the agent's own trigger. A 4 server paired with a 5 agent would
// push commands nothing listens for and would drop the results arriving inside
// packets it otherwise parses perfectly — a mismatch whose only symptom is a
// diagnostic feature that silently never produces anything.
//
// 6 removed reported_config_version from Hello and Packet (Hello field 6,
// Packet field 9 — both reserved, never to be reused). The server stored it and
// nothing ever read it back; the live "what config has the agent applied"
// signal is the MonitorStatus frame's config-version echo. The removal is what
// makes it breaking: a 5 agent still stamps the field on every packet and a 6
// server has no slot for it — protobuf would skip it silently, but the JSON
// codec's strict decode would not, and a silent skip is exactly the ambiguity
// an exact-match handshake exists to refuse.
const SchemaVersion = 6

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

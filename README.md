<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://nettact.org/brand/nettact-logo-horizontal-reverse.svg">
    <source media="(prefers-color-scheme: light)" srcset="https://nettact.org/brand/nettact-logo-horizontal.svg">
    <img alt="NetTact" src="https://nettact.org/brand/nettact-logo-horizontal.svg" width="280">
  </picture>
</p>

# NetTact Protocol

English | [简体中文](./README-zh.md)

NetTact Protocol defines the shared data model and wire contract between the Agent, Server, and Desktop. It gives collectors, servers, and embedded runtimes one set of Go types so the same field cannot acquire different meanings in different repositories.

This is a Go library, not an executable service, and does not need to be deployed separately. Most NetTact users consume it indirectly through the Agent, NetTact Server, or Desktop.

## What It Solves

- Shared types for telemetry, events, inventory, configuration delivery, permissions, and enrollment
- Message frames, acknowledgements, and error semantics for the persistent Agent connection
- Human-readable JSON and bandwidth-efficient Protobuf encodings
- Idempotent uploads and disconnection recovery through sequence numbers and acknowledgement watermarks
- Explicit schema validation that fails early when components are incompatible
- Standard-library-only core type packages; Protobuf is pulled in only by the optional `wire` package

## Packages

| Package | Purpose |
|---|---|
| `telemetry` | Metrics, events, interface snapshots, inventory, incident evidence, and upload packets |
| `config` | Probe targets, schedules, egress proxies, game collection, and diagnostic requests sent from Server to Agent |
| `enroll` | Requests, signatures, and credentials used during initial Agent enrollment |
| `permission` | Capability catalog, local grant policy, and permission reports |
| `gamesense` | Game runs, frame-time buckets, collection gaps, and host-level per-second data |
| `wire` | WebSocket frames, JSON/Protobuf codecs, links, and pipe abstractions |
| `wire/pb` | Generated Go code from the Protobuf schema |

## Installation

```bash
go get github.com/nettact/protocol@latest
```

Import only the type packages you need:

```go
import (
    "github.com/nettact/protocol/config"
    "github.com/nettact/protocol/permission"
    "github.com/nettact/protocol/telemetry"
)
```

Import `wire` only when you need JSON/Protobuf encoding or an Agent transport. Consumers of the core types do not pull in the Protobuf runtime.

## Encoding Example

```go
package main

import (
    "fmt"
    "time"

    "github.com/nettact/protocol"
    "github.com/nettact/protocol/telemetry"
    "github.com/nettact/protocol/wire"
)

func main() {
    packet := telemetry.Packet{
        SchemaVersion: protocol.SchemaVersion,
        AgentID:       "agent-01",
        SiteID:        "default",
        Sequence:      42,
        SentAt:        time.Now().UTC(),
        Metrics: []telemetry.Metric{{
            TS:     time.Now().UTC(),
            Kind:   telemetry.ICMPRTTms,
            Target: "gateway",
            Value:  3.8,
            Unit:   "ms",
        }},
    }

    raw, err := wire.MarshalPacket(packet, wire.ContentTypeProtobuf)
    if err != nil {
        panic(err)
    }

    decoded, err := wire.UnmarshalPacket(raw, wire.ContentTypeProtobuf)
    if err != nil {
        panic(err)
    }
    fmt.Println(decoded.AgentID, decoded.Sequence)
}
```

WebSocket connections can negotiate the `nettact.v1.protobuf` or `nettact.v1.json` subprotocol. HTTP uses `application/x-protobuf` or `application/json`; JSON is the fallback when Protobuf is not explicitly selected.

## Versions and Upgrades

`protocol.SchemaVersion` is the wire-schema version. The Agent and Server validate it during connection setup and data ingestion. An incompatible peer is rejected explicitly instead of silently losing fields.

Use mutually compatible official releases of the Agent, Server, and Desktop. Custom integrations should set the current `SchemaVersion` on outgoing data and call `protocol.ValidateSchema` on incoming data.

Server components may serve an adjacent schema version alongside the current one through an explicit per-version adapter registry (one adapter per schema, selected by exact membership); `ValidateSchema` itself stays the exact native-schema check and is never relaxed into a range.

## Regenerating Protobuf

Regular consumers do not need the Protobuf toolchain because generated code is committed under `wire/pb/`. Regeneration is required only when the protocol source changes:

```bash
cd wire
buf generate
```

The schema is in [`wire/proto/telemetry.proto`](./wire/proto/telemetry.proto), and generation settings are in [`wire/buf.gen.yaml`](./wire/buf.gen.yaml). Install [Buf](https://buf.build/) and `protoc-gen-go` first.

Protocol changes must also update `wire/convert.go` and the codec tests. Never reuse a Protobuf field number that has already been assigned.

## Local Development

```bash
go test ./...
go build ./...
```

In the NetTact multi-repository workspace, the root `go.work` points the Agent, Server, and Desktop at this local source tree.

## License

[Apache License 2.0](./LICENSE)

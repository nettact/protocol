# protocol

NetTact 的共享线缆协议（telemetry / capability / …）。**Apache-2.0，仅依赖 Go 标准库**，由 agent、server-core、server-lite 与未来 Cloud 共用——同一套协议，不 fork（架构 §15.2-3）。

- `version.go` — `SchemaVersion` + `ValidateSchema`
- `telemetry/` — `Packet` / `Metric` / `Event` / `InventoryItem` / `HealthLayer`
- `capability/` — Agent 能力枚举（§3.4）

```
import "github.com/nettact/protocol/telemetry"
```

本地多仓开发使用 `go.work`（见组织内工作区说明）。

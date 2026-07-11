# protocol

NetTact 的共享线缆协议（telemetry / capability / …）。**Apache-2.0**，由 agent、server-core、server-lite 与未来 Cloud 共用——同一套协议，不 fork（架构 §15.2-3）。

类型包（`telemetry` / `config` / `enroll` / `capability`）**仅依赖 Go 标准库**；唯一例外是可选的 `wire` 编解码包，它额外引入 `google.golang.org/protobuf`——只有导入 `wire` 的消费者才会拉入该依赖。

- `version.go` — `SchemaVersion` + `ValidateSchema`
- `telemetry/` — `Packet` / `Metric` / `Event` / `InventoryItem` / `HealthLayer`
- `capability/` — Agent 能力枚举（§3.4）
- `wire/` — telemetry 上行包与其 ack 的 **JSON / Protobuf** 双格式编解码（按 HTTP `Content-Type` / `Accept` 协商，默认回退 JSON）。Protobuf 显著减小上行流量（架构 §5.1：JSON/Gzip → Protobuf）。

```
import "github.com/nettact/protocol/telemetry"
import "github.com/nettact/protocol/wire" // 需要 protobuf 编解码时
```

### wire 编解码

```go
raw, _ := wire.MarshalPacket(pkt, wire.ContentTypeProtobuf) // 或 ContentTypeJSON
pkt, _ := wire.UnmarshalPacket(raw, r.Header.Get("Content-Type"))
```

`.proto` 源码位于 `wire/proto/telemetry.proto`，生成代码在 `wire/pb/`（已提交，消费者无需 protoc 工具链）。重新生成：`cd wire && buf generate`（需 `buf` 与 `protoc-gen-go`）。字段号一经发布不得复用或重排。

本地多仓开发使用 `go.work`（见组织内工作区说明）。

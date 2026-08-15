<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://nettact.org/brand/nettact-logo-horizontal-reverse.svg">
    <source media="(prefers-color-scheme: light)" srcset="https://nettact.org/brand/nettact-logo-horizontal.svg">
    <img alt="NetTact" src="https://nettact.org/brand/nettact-logo-horizontal.svg" width="280">
  </picture>
</p>

# NetTact Protocol

[English](./README.md) | 简体中文

NetTact Protocol 定义 Agent、Server 和 Desktop 之间共享的数据模型与线缆协议。它让采集端、服务端和嵌入式运行时使用同一套 Go 类型，避免同一个字段在不同仓库里出现含义不一致的问题。

这是一个供 Go 程序引用的库，不是可执行服务，因此不需要单独部署。普通 NetTact 用户会通过 Agent、NetTact Server 或 Desktop 间接使用它。

## 它解决什么问题

- 为遥测、事件、设备清单、配置下发、权限和注册提供统一类型
- 为 Agent 的持久连接定义消息帧、确认与错误语义
- 同时支持易读的 JSON 和更省带宽的 Protobuf
- 通过序列号和确认水位支持幂等上传与断线重传
- 使用明确的 Schema 版本，在组件版本不匹配时尽早报错
- 除可选的 `wire` 包外，核心类型包只依赖 Go 标准库

## 包结构

| 包 | 用途 |
|---|---|
| `telemetry` | 指标、事件、接口快照、设备清单、故障现场和数据批次 |
| `config` | Server 下发给 Agent 的目标、周期、代理出口、游戏采集与诊断请求 |
| `enroll` | Agent 首次注册所需的请求、签名和凭据类型 |
| `permission` | 能力目录、本地授权策略和权限报告 |
| `gamesense` | 游戏运行、帧时间分桶、采集间隙和主机级秒数据 |
| `wire` | WebSocket 帧、JSON/Protobuf 编解码、连接与管道抽象 |
| `wire/pb` | 由 `.proto` 生成的 Go 代码 |

## 安装

```bash
go get github.com/nettact/protocol@latest
```

只需要共享类型时直接导入对应包：

```go
import (
    "github.com/nettact/protocol/config"
    "github.com/nettact/protocol/permission"
    "github.com/nettact/protocol/telemetry"
)
```

只有需要实际进行 JSON/Protobuf 编解码或建立 Agent 链路时才导入 `wire`，这样普通类型消费者不会额外引入 Protobuf 运行时。

## 编解码示例

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

WebSocket 连接可协商 `nettact.v1.protobuf` 或 `nettact.v1.json` 子协议。HTTP 场景则使用 `application/x-protobuf` 或 `application/json`；未明确选择 Protobuf 时会使用 JSON。

## 版本与升级

`protocol.SchemaVersion` 是线缆结构版本。Agent 和 Server 会在连接及数据入口校验它；不兼容时会明确拒绝连接，而不是静默丢弃字段。

升级 NetTact 时，应使用彼此匹配的 Agent、Server 和 Desktop 正式版本。若自行集成此库，请在发送数据时填入当前 `SchemaVersion`，并在接收数据时调用 `protocol.ValidateSchema`。

## 重新生成 Protobuf

普通使用者不需要安装 Protobuf 工具链，生成结果已经提交在 `wire/pb/`。修改协议源码时才需要重新生成：

```bash
cd wire
buf generate
```

源码位于 [`wire/proto/telemetry.proto`](./wire/proto/telemetry.proto)，生成配置位于 [`wire/buf.gen.yaml`](./wire/buf.gen.yaml)。需要预先安装 [Buf](https://buf.build/) 和 `protoc-gen-go`。

修改协议时还应同步更新 `wire/convert.go` 和编解码测试。已经使用过的 Protobuf 字段号不得重复分配。

## 本地开发

```bash
go test ./...
go build ./...
```

在 NetTact 多仓工作区中，根目录 `go.work` 会把 Agent、Server 和 Desktop 指向这份本地源码。

## 许可证

[Apache License 2.0](./LICENSE)

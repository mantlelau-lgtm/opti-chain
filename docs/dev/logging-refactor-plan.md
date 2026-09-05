# 日志规范化改造方案（zap）

> 目标：用 `go.uber.org/zap` 替换 `log` 包，符合团队规范第 11 条。

## 1. 当前状态

仅 18 处日志调用，集中分布在 4 个文件：

| 文件 | 调用 | 类型 |
|---|---|---|
| `cmd/server/main.go` | 11 | `log.Fatalf`（启动失败） + `log.Printf`（启动提示） |
| `cmd/scm-mcp/main.go` | 2 | `log.Fatal`（参数校验） + `log.Fatalf`（stdio 失败） |
| `internal/database/migrate_tenant.go` | 1 | `log.Printf`（迁移索引错误） |
| `internal/service/rbac_seed.go` | 1 | `log.Printf`（种子数据错误） |

范围很小，改动风险低。

## 2. 改造方案

### 2.1 新增依赖

```bash
go get go.uber.org/zap
```

### 2.2 Logger 初始化（`cmd/server/main.go`）

```go
import "go.uber.org/zap"

func main() {
    logger, _ := zap.NewProduction()
    defer logger.Sync()
    zap.ReplaceGlobals(logger) // 全局可用 zap.L()

    // 启动致命错误
    zap.L().Fatal("db open", zap.Error(err))

    // 启动提示
    zap.L().Info("SCM server listening",
        zap.String("addr", cfg.Server.Addr),
        zap.String("driver", cfg.DB.Driver),
        zap.Bool("auth", cfg.Auth.Enabled),
    )
}
```

### 2.3 各文件改动

**`cmd/server/main.go`（11 处）**

```go
// 改前
log.Fatalf("db open: %v", err)
// 改后
zap.L().Fatal("db open", zap.Error(err))

// 改前
log.Printf("WARNING: SCM_AUTH=off — authentication disabled (dev only)")
// 改后
zap.L().Warn("SCM_AUTH=off — authentication disabled (dev only)")

// 改前
log.Printf("SCM server listening on %s (driver=%s, auth=%v)", ...)
// 改后
zap.L().Info("SCM server listening",
    zap.String("addr", cfg.Server.Addr),
    zap.String("driver", cfg.DB.Driver),
    zap.Bool("auth", cfg.Auth.Enabled),
)
```

**`cmd/scm-mcp/main.go`（2 处）**

```go
// 改前
log.Fatal("SCM_MCP_AK and SCM_MCP_SK are required ...")
// 改后
zap.L().Fatal("SCM_MCP_AK and SCM_MCP_SK are required")

// 改前
log.Fatalf("stdio server: %v", err)
// 改后
zap.L().Fatal("stdio server", zap.Error(err))
```

**`internal/database/migrate_tenant.go`（1 处）**

await 包 `database` 不依赖 `cmd/server`，需要自己创建 logger 或接收注入。

方案 A：包内创建 `var logger = zap.NewExample()`（lightweight）
方案 B：main.go 注入 `SetLogger(logger)`

推荐方案 A——包内自给自足，不改 main.go 签名。

```go
import "go.uber.org/zap"

var log = zap.NewExample()

// 改前
log.Printf("migrate index %s: %v", composite, err)
// 改后
log.Warn("migrate index", zap.String("idx", composite), zap.Error(err))
```

**`internal/service/rbac_seed.go`（1 处）**

同 database 包，方案 A：

```go
// 改前
log.Printf("seed: adopt %s: %v", t, err)
// 改后
log.Warn("seed adopt", zap.String("tenant", t), zap.Error(err))
```

### 2.4 日志级别映射

| 原 `log` 调用 | zap 级别 |
|---|---|
| `log.Fatal` / `log.Fatalf` | `zap.L().Fatal` |
| `log.Printf`（WARNING/错误） | `zap.L().Warn` |
| `log.Printf`（启动信息） | `zap.L().Info` |

## 3. 实施步骤

| 步骤 | 内容 | 验证 |
|---|---|---|
| 1 | `go get go.uber.org/zap` | — |
| 2 | 改 `cmd/server/main.go`（11 处） | `go build ./cmd/server` |
| 3 | 改 `cmd/scm-mcp/main.go`（2 处） | `go build ./cmd/scm-mcp` |
| 4 | 改 `internal/database/migrate_tenant.go`（1 处） | `go build ./internal/database/...` |
| 5 | 改 `internal/service/rbac_seed.go`（1 处） | `go build ./...` |
| 6 | 移除 `import "log"`（不再使用） | `go build ./...` |
| 7 | `go mod tidy` | — |

## 4. 无需改动

- `internal/service/audit_service.go` 的 `Log` 写入 `OperationLog` 表，不是打印日志——不动
- `internal/service/inventory_service.go` 注释中提到 "audit log"——是注释，不动
- 前端的 `message.error()` / `message.success()` ——前端 antd 的 message，不是 Go 日志
# Context 规范化改造方案

> 目标：所有业务函数第一个参数改为 `ctx context.Context`，符合团队规范第 7 条。

## 1. 改动范围

| 层 | 文件数 | 方法数 | 调用方 |
|---|---|---|---|
| service | 16 | ~143 | handler、assistant、service 内部互调 |
| repo | 12 | ~108 | service |
| memory | 1 | ~4 | service、handler |
| handler | 17 | — | 传 `c.Request.Context()` |
| cmd/server/main.go | 1 | — | 传 `context.Background()` |

**总计约 250 个方法签名 + 约 500 个调用点。**

## 2. 失败教训

上一轮用 sed 批量替换，两个致命问题：

1. **签名和调用点无法区分**——`(t uint,` 这串字符同时出现在方法签名和调用点，sed 统统替换，导致 `s.repo.List(ctx context.Context, t, ...)` 这种把类型名当参数传的语法错误。

2. **级联错误**——改了一个方法签名，所有调用方都得同步改，逐层传递。repo 改完 service 要改，service 改完 handler 要改，handler 改完 router 要改。sed 逐层改的时候，改到一半服务层编译不过，改不了调用方。

## 3. 正确做法：逐文件、自上而下

### 3.1 改造顺序

```
第 1 层：repo（被调用方，无下层依赖）
  → 改 repo 方法签名
  → 改 service 对 repo 的调用

第 2 层：service（调用 repo，被 handler/assistant 调用）
  → 改 service 方法签名
  → 改 handler 对 service 的调用
  → 改 service 内部互调

第 3 层：memory / middleware
  → 改签名 + 调用方

第 4 层：handler / assistant
  → 传 c.Request.Context()
```

### 3.2 每个文件的操作步骤

以 `internal/repo/base_data_repo.go` 为例：

```
1. 读文件
2. 对每个方法签名，手工加 ctx context.Context：
   func (r *MaterialRepo) Create(t uint, m *model.Material) error
   → func (r *MaterialRepo) Create(ctx context.Context, t uint, m *model.Material) error
3. 对每个方法签名，同样加 ctx
4. go build ./internal/repo/... — 确认 repo 层编译通过
5. 找出所有调用方：grep -rn 'MaterialRepo).Create(' internal/service/
6. 逐个修改调用方：
   s.repo.Create(t, m) → s.repo.Create(ctx, t, m)
7. go build ./... — 确认全量编译通过
8. git commit
```

### 3.3 批量辅助

部分机械替换可以用带上下文的 sed 加固：

```bash
# 只改方法签名（匹配 func (s *ServiceName) 前缀）
sed -i '' 's/\(func (s \*MaterialService) Create\)(t uint,/\1(ctx context.Context, t uint,/g' internal/service/base_data_service.go
```

这比无前缀的 `(t uint,` 安全得多——不会误伤调用点。

## 4. 分阶段实施计划

| 阶段 | 文件 | 预计工作量 |
|---|---|---|
| P1 | repo 层全部（12 文件） | 1 次 |
| P2 | service 层（16 文件，按依赖顺序） | 3-4 次 |
| P3 | handler 层（17 文件） | 1 次 |
| P4 | memory / middleware / main.go | 1 次 |

每个阶段结束后 `go build ./...` + `go test ./...` 验证，再进入下一阶段。

## 5. Handler 调用方改动示例

```go
// 改前
h.svc.Create(tenantOf(c), m)
// 改后
h.svc.Create(c.Request.Context(), tenantOf(c), m)
```

handler 不需要额外 `import "context"`——`c.Request.Context()` 来自 gin。

## 6. Assistant 调用方改动示例

```go
// 改前
deps.Materials.Create(actor.TenantID, m)
// 改后
deps.Materials.Create(ctx, actor.TenantID, m)
```

`ctx` 来自 `run(ctx context.Context, ...)` 方法签名，已经在线程中。

## 7. 风险点

- `storage_service.go`：存储迁移用到反射和事务，`ctx` 需要传入 `db.WithContext(ctx)`
- `approval_service.go`：`evaluateTx` 使用 `*gorm.DB` 事务，需要 `tx.WithContext(ctx)`
- `rbac_seed.go`：种子函数在 main.go 启动时调用，传 `context.Background()`
- 泛型 `tenantRepo`：`base.go` 中的 `listT`/`paginate` 需要改签名，影响所有 repo
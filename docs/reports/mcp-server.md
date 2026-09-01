# MCP Server 与 AK/SK 接入方案

> 面向：需要接入 agent / 大模型客户端（本地 llm-gw）的开发者与租户管理员。

## 1. 目标

为 SCM 提供一条**面向智能体（agent）**的接入通道：

1. 完整 **AK/SK 认证机制**（HMAC-SHA256 请求签名，防重放）。
2. **每个用户自助签发密钥**，密钥绑定到个人（租户 + 用户），权限随个人角色自动授予。
3. **MCP（Model Context Protocol）server**，把「下单 + 新建基础数据」能力暴露为工具，
   每个工具维护好 description，让 LLM 能识别并快速调用，减少对 LLM 判断的依赖。

## 2. 架构

```
┌────────────┐  stdio(MCP)   ┌──────────────┐  HTTP + AK/SK签名   ┌──────────────┐
│  agent /    │ ────────────▶ │  scm-mcp     │ ──────────────────▶ │  SCM backend │
│  llm-gw     │               │  (cmd/scm-mcp)│                     │  (Gin + GORM) │
└────────────┘                └──────────────┘                     └──────────────┘
```

- **scm-mcp** 是独立二进制（`go build ./cmd/scm-mcp`），通过 **stdio** 提供 MCP 协议；
  配置 AK/SK 后，把每个 MCP 工具调用翻译为带签名的 HTTP 请求打到 SCM 后端。
- **SCM 后端** 新增一套认证中间件，兼容两种身份：
  - `Authorization: Bearer <JWT>`（浏览器会话，原有路径不变）；
  - `X-Api-Key` / `X-Timestamp` / `X-Signature`（agent / MCP 客户端）。
  两者统一落到 `authx.Actor`，走同一套 `RequirePerm` 权限校验。

## 3. AK/SK 机制

### 3.1 数据模型（sys_api_key）

| 字段 | 说明 |
| --- | --- |
| tenant_id | 所属租户（行级隔离） |
| user_id | 密钥所有者（绑定个人，仅本人可见/管理自己的密钥） |
| name | 密钥名称（用于留痕 created_by） |
| ak | 访问密钥（全局唯一索引，`ak_` 前缀 + base64url 随机串） |
| sk_cipher | **AES-GCM 加密后的 Secret Key**（用服务端 JWT secret 派生密钥加密，永不落明文） |
| permissions | 逗号分隔的权限点；**签发时按用户当前角色自动授予**（空 = 全部权限） |
| status | 1 启用 / 0 禁用 |
| expires_at | 可选过期时间 |

Secret Key 仅在签发时明文返回一次，此后不可恢复（库里只存密文，但服务端可解密用于验签）。

### 3.2 签名算法（HMAC-SHA256）

请求头：

```
X-Api-Key:    <ak>
X-Timestamp:  <unix 秒>
X-Signature:  <小写 hex HMAC-SHA256>
```

待签名的规范字符串（canonical string，字段以 `\n` 连接）：

```
<ak>\n<timestamp>\n<METHOD>\n<path>\n<body-sha256-hex>
```

- `METHOD`：大写 HTTP 动词。
- `path`：URL 路径（**不含** query string）。
- `body-sha256-hex`：请求体原始字节的 SHA-256（无 body 时为空串的哈希）。

签名：`hex( HMAC-SHA256(sk, canonical) )`。

防重放：`|now - timestamp| ≤ 300s`。

### 3.3 权限三层校验

1. **签名校验**：AK 存在、状态启用、未过期、时间戳在窗口内、签名匹配。
2. **租户隔离**：AK 解析出的 `TenantID` 贯穿所有数据访问（与 JWT 一致）。
3. **权限点**：`HasPerm` 对 AK/SK actor 走「KeyPerms」分支——空集合 = 全部权限，
   否则按逗号分隔的权限点精确匹配（复用 `routePerms` 路由→权限映射）。

## 4. 密钥管理

- **入口**：业务侧「工作台 → 密钥签发」（每个登录用户都可见）。
- **能力**：填写密钥名称后按**个人当前角色权限自动签发**（无需手动选权限），列表 / 禁用 / 启用 / 删除，
  所有操作只作用于**本人**的密钥。
- **端点**（JWT 认证，需真实用户，AK/SK actor 不可签发）：

| Method | Path | 说明 |
| --- | --- | --- |
| GET | `/api/v1/api-keys` | 列表 |
| POST | `/api/v1/api-keys` | 签发（返回 `{key, sk}`，sk 只显示一次） |
| PUT | `/api/v1/api-keys/:id/disable` | 禁用 |
| PUT | `/api/v1/api-keys/:id/enable` | 启用 |
| DELETE | `/api/v1/api-keys/:id` | 删除 |

## 5. MCP Server（scm-mcp）

### 5.1 启动

```bash
SCM_MCP_BASE_URL=http://127.0.0.1:8088 \
SCM_MCP_AK=ak_xxx \
SCM_MCP_SK=sk_xxx \
./bin/scm-mcp
```

- `SCM_MCP_AK` / `SCM_MCP_SK`：必填，来自「系统 → API 密钥」签发的密钥。
- `SCM_MCP_BASE_URL`：SCM 后端地址（默认 `http://127.0.0.1:8088`）。

### 5.2 工具清单（20 个）

**查询（用于先查 ID 再创建）**：`material_list`、`supplier_list`、`warehouse_list`、
`customer_list`、`product_list`、`bom_list`、`po_list`、`stock_list`。

**新建基础数据**：`material_create`、`supplier_create`、`customer_create`、
`warehouse_create`、`location_create`、`product_create`、`bom_create`、
`supplier_material_bind`。

**下单**：`po_create`、`so_create`、`bom_order_preview`（预览拆单）、
`bom_order_confirm`（确认拆单）。

每个工具的 `description` 都写明了：用途、何时使用、关键约束（如「供应商须 APPROVED 才能下单」）、
以及复杂参数（details / items 数组）的字段结构与示例，便于 LLM 直接识别调用。

### 5.3 接入 llm-gw / agent

把 `scm-mcp` 以 stdio 方式注册到本地 agent / llm-gw 的 MCP 配置即可（协议版本 `2024-11-05`）。
工具调用的权限范围由签发的 AK/SK 决定，无需在客户端再配一次权限。

## 6. 落盘位置

- 模型：`internal/model/api_key.go`
- 仓库：`internal/repository/api_key_repo.go`
- 服务：`internal/service/api_key_service.go`（签发 / 验签 / 生命周期 / SK 加密）
- 中间件：`internal/middleware/api_key.go`（`AuthOrKey` 兼容 JWT + AK/SK）
- 处理器：`internal/handler/api_key_handler.go`，路由：`internal/router/router.go`
- 签名契约：`internal/pkg/aksk/aksk.go`（服务端与 MCP 客户端共用）
- MCP 二进制：`cmd/scm-mcp/`（main.go / client.go / tools.go）
- 前端：`web/src/pages/ApiKeyPage.jsx`（入口在「工作台 → 密钥签发」）

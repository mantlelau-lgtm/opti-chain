# v0.2 设计方案：认证权限与多租户

## 目标

为 opti-chain 引入认证（AuthN）、基于 RACI 的角色权限（AuthZ）与公共多租户 SaaS 能力，保持轻量定位。

## 已拍板决策（2026-08-30）

1. **SaaS 形态**：公共多租户 SaaS → 共享 schema + `tenant_id` 行级隔离
2. **角色体系**：固定六角色（v0.2 不做可配置 RBAC UI）
3. **认证方式**：自建 JWT（不接 OIDC/SSO）

## 认证（P1）

- `sys_user`：username(UQ) / password_hash(bcrypt) / name / status
- `POST /auth/login`（公开）→ JWT（golang-jwt/v5），claims：`uid / username / name`，TTL 24h
- 中间件 `Auth`：解析 `Authorization: Bearer`，失败 401 + code 40100；成功写入 `Actor{UserID, Username, Name}` 到 gin.Context
- 配置：`SCM_AUTH`（on/off，默认 on，off 仅开发用）、`SCM_JWT_SECRET`（默认开发值，生产必设）、`SCM_JWT_TTL`
- 启动种子：无用户时创建 `admin / admin123`
- 前端：登录页、token 存 localStorage、client 注入 header、401 清 token 跳登录、布局显示用户+退出
- 验收：未登录 401；错误密码 40100；登录后全部模块可用

## RBAC（P2）

六角色（种子，不可删）：

| 角色 | 编码 | 关键权限 |
| --- | --- | --- |
| 管理员 | admin | 全部 + 用户管理 |
| 品类经理 | category_manager | 主数据/PO 审批决策、supplier:audit |
| 采购助理 | procurement_assistant | po:create、收货协调、寻源执行 |
| 采购决策委员会 | committee | po:approve（定标）、大额审批 |
| 仓库/质检 | qc_wh | po:receive、库存调整、盘点 |
| 财务 | finance | 对账查看、信用管控 so:approve |

- 表：`sys_role / sys_user_role / sys_role_perm`；权限点 `资源:动作` 挂路由表，中间件 `RequirePerm`
- `created_by` 字符串 → `created_by_id`（+姓名冗余展示）
- 前端按钮级隐藏（usePerm）+ 用户管理页（admin）
- 验收负例：助理调 approve 403；qc 调 po:create 403

## 多租户（P3）

- `sys_tenant`：code(UQ) / name / plan / status / expires_at；平台超管 `tid=0`
- 全部业务表加 `tenant_id`；唯一约束改复合 `(tenant_id, xxx)`——SQLite AutoMigrate 不改索引，需一次性迁移脚本
- 登录带租户码 → token 携带 `tid`；全链路以 token 为准（不做子域名）
- GORM 插件：查询回调对实现 `Tenanted` 的模型自动 `WHERE tenant_id=?`；`BeforeCreate` 自动填值；context 传递 Actor
- 验收：两租户同单号不冲突、互相不可见（E2E）

## 运营骨架（P4）

- 租户启停（middleware 校验 token 租户状态，停用 403）
- 套餐字段预留 feature flags（如免费版无 MRP）
- 租户级数据导出/清空脚本；计费放 v0.3

## 影响范围与风险

- 最大机械改造：Actor/context 贯穿 service（P1 仅 handler 层取 Actor，P2/P3 下沉）
- JWT 默认密钥仅开发用，文档与启动日志需告警
- SQLite 并发写为单写者，SaaS 上量后切 MySQL（配置已支持）

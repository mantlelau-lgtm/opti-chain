# 系统架构设计

## 概述

opti-chain 是轻量级供应链管理系统：Go（Gin + GORM）后端 + React（Vite + antd）前端，SQLite/MySQL 双引擎。覆盖「主数据 → 采购收货 → 销售履约 → 仓储 → 计划 MRP」闭环。

## 架构图

```
┌────────────────────────  web (React 18 + antd 5 + Vite)  ────────────────────────┐
│  pages/*  ── api/index.js (axios, 信封解包)  ── vite proxy /api → :8088           │
└──────────────────────────────────────────┬───────────────────────────────────────┘
                                           │ HTTP JSON
┌──────────────────────────────────────────▼───────────────────────────────────────┐
│ cmd/server/main.go            装配: config → database → repository → service → handler → router
│                                                                                   │
│  router (Gin, CORS+Recovery)                                                      │
│    └─ handler/*        参数绑定/信封响应（无业务逻辑）                              │
│        └─ service/*    业务规则与事务边界                                           │
│            └─ repository/*  GORM 数据访问；*InTx 方法参与调用方事务                   │
│                └─ database  SQLite(glebarez) / MySQL 切换，AutoMigrate              │
└───────────────────────────────────────────────────────────────────────────────────┘
```

## 模块划分

| 模块 | 包/文件 | 职责 |
| --- | --- | --- |
| 基础数据 | model/base_data.go, service/base_data_service.go | 物料/供应商(审核)/客户(信用)/仓库/库位 |
| 采购 | model/procurement.go, service/procurement_service.go, receiving_service.go | PO 状态机；收货闭环 |
| 销售 | model/sales.go, service/sales_service.go | SO 审批锁库/取消释放/信用控制 |
| 仓储 | model/inventory.go, service/inventory_service.go | 库存原子调整、流水审计 |
| 计划 | model/planning.go, service/planning_service.go | 需求聚合、MRP 运算、转 PO |

## 关键机制

### 响应信封

`{code, message, data}`；`code=0` 成功。业务错误 HTTP 仍 200，靠 code 区分：40000 参数/规则、40400 不存在、40900 冲突、50000 内部错误。前端 `client.js` 非 0 即 reject。

### 收货闭环（receiving_service.go）

单事务内：收货单落库 → `received_qty` 仅累加**合格量**（SQL 守卫防超收）→ 合格量走 `applyMovementInTx` 入库存+流水 → 按累计合格推进 PO 状态（IN_PROGRESS / COMPLETED）。拒收量不碰库存，保留为在途。

### 防超卖（sales_service.go + inventory_repo.go）

- 可用库存 = `quantity − locked_quantity`。
- 审批：跨库存行从大到小分配锁定，逐行 `UPDATE ... WHERE id=? AND quantity >= locked_quantity + ?`，0 行即回滚。
- 出库：`AdjustGated` 守卫实物不为负。
- **SQLite 类型亲和性陷阱**：绑定参数（decimal 序列化为 TEXT）与**表达式**比较时不做数值转换（INTEGER 恒小于 TEXT），守卫会恒假。守卫谓词必须让**列**位于比较左侧（列的 NUMERIC 亲和性会转换另一侧）。见 `LockRowInTx` 注释。

### 事务与并发模式

- 服务层持有 `*gorm.DB` 开启事务；repository 提供 `*InTx(tx, ...)` 变体参与外部事务。
- 并发正确性靠**带守卫的原子 UPDATE + RowsAffected 判定**，严禁先 SELECT 后 UPDATE。
- 头+明细写入统一 `Omit(clause.Associations)` 防止 GORM 级联双插主键冲突。

### 准入与信用管控

- 供应商 `audit_status != APPROVED` → 禁止创建/改绑 PO。
- 客户 `audit_status != APPROVED` → 禁止创建 SO；`credit_limit > 0` 时审批校验 `used_credit + 单额 ≤ 额度`。

### MRP 公式

`建议采购量 = Σ(OPEN 需求) + min_stock − Σ 现有库存 − Σ 在途`，在途 = 未完结 PO 的 `order_qty − received_qty`（含拒收待补），结果下限 0。

## 部署架构

单机：SQLite 文件 + 单进程后端 + 静态前端。扩展：`SCMDB_DRIVER=mysql` 切 MySQL（DSN 需 `parseTime=True`），表结构 AutoMigrate 兼容两库。

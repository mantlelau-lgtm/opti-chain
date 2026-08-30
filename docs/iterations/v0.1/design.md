# v0.1 设计方案

## 目标

打通供应链最小闭环：主数据 → 采购收货（含质检拒收）→ 销售履约（防超卖）→ 仓储 → 计划 MRP，全部端到端可验证。

## 方案概述

1. **收货闭环**：收货单（pur_receipt）一行三量——到货/合格/拒收。合格入库存+流水+累计 received_qty；拒收不入库、留原因作退换依据，并保留为在途待补。单事务保证收货单、PO 进度、库存、流水一致。
2. **防超卖**：销售审批按「可用 = 实物 − 锁定」跨仓分配锁定；取消释放。全部带守卫原子 UPDATE，RowsAffected 判成败。
3. **管控门槛**：供应商准入（未核准禁下 PO）、客户准入 + 信用额度（审批占用/取消释放）。
4. **MRP**：`建议 = 毛需求 + min_stock − 库存 − 在途`；在途用 `order_qty − received_qty`，天然包含拒收待补量。

## 详细设计

见 `docs/dev/architecture.md`（机制）、`docs/dev/data-model.md`（不变量）、`docs/dev/api.md`（接口契约）。

关键取舍：

- **received_qty 只累计合格量**：让 MRP 在途语义自动正确，无需额外补偿逻辑。
- **拒收不做独立质检单**：v0.1 将质检内嵌收货行（合格/拒收/原因），降低复杂度；独立 `inv_qc_order`（含让步接收）列入 v0.2。
- **锁定不记账到订单级台账**：locked 是物料级聚合量，取消时按行贪心释放（锁定单位同质），避免新增台账表。
- **编辑仅表头**：防止编辑抹掉收货进度；全量替换路径保留给 API 消费者。

## 影响范围

- 新增表 6：pur_receipt(_detail)、base_customer、sale_order(_detail)；sys_supplier 增 audit_status。
- 后端新增 receiving/sales 两个 service、sales/receiving 两个 handler；router 增 /so、/customers、/po/:id/receive|receipts、/suppliers/:id/audit。
- 前端新增客户页、销售订单页；采购页增收货/收货记录弹窗；供应商页增核准操作。

## 风险与对策

| 风险 | 对策 |
| --- | --- |
| SQLite 无行锁，并发审批可能超锁 | 守卫 UPDATE + 事务回滚；单机场景足够，MySQL 下天然行锁 |
| decimal 绑定参数在 SQLite 的比较亲和性陷阱 | 守卫谓词列在左侧；代码注释固化 |
| 无 BOM，MRP 单层 | 贸易型定位可接受；制造型需 v0.2+ 引入 BOM 展开 |

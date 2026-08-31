# v0.3 设计方案：研发产品与 BOM

## 目标

新增研发模块：管理研发产品及其物料清单（BOM），为 MRP 按产品展开原料需求打基础。

## 已拍板决策（2026-08-31）

1. **产品独立表** `base_product`（不复用物料表）
2. **研发领料复用库存单** `inv_order`（新增 `RND_ISSUE` 类型，P2 落地）

## 概念模型（单层 BOM）

```
产品(base_product) ──1:N── BOM(base_bom，版本化) ──1:N── BOM明细(base_bom_detail → 原料 sys_material)
```

- **组件 = 原料**（sys_material），单层展开；多级 BOM（产品可作为子件）列为后续扩展
- 产品是规格，不进入 inv_stock（成品库存为未来扩展）

## 表结构（新增 3 张，租户隔离）

- `base_product`：product_code(UQ per tenant) / name / spec / unit / cost_price(参考成本) / status
- `base_bom`：bom_no(UQ per tenant) / product_id / version / status(DRAFT/RELEASED/OBSOLETE) / is_default / unit_qty / remark
- `base_bom_detail`：bom_id / component_id(→sys_material) / qty_per_unit / scrap_rate / remark

## 关键规则

1. 版本：DRAFT 可编辑/删除；发布 → RELEASED 并自动把旧默认版置 OBSOLETE（每产品仅一条默认有效版）；已发布不可改，改则开新版本
2. 单层 BOM 无环路风险（组件是原料、原料无 BOM），环检测推迟到多级 BOM 阶段
3. 发布校验：明细非空、用量 > 0

## MRP（P2）

- `plan_demand` 增加可选 `product_id`：产品需求 → 按默认 BOM 展开为原料毛需求（×unit_qty），原料需求沿用现有库存/在途/安全库存逻辑
- `plan_mrp_result` 增加 `parent_mrp_id`（追溯）、`bom_id`（来源）
- 产品自身暂不生成采购建议（无库存维度），只驱动组件采购

## 成本核算（P2，顺带）

- `sys_material` 增加 `cost_price`；BOM 详情页递归汇总「产品参考成本 = Σ 组件用量 × 组件成本」

## 权限（表存储）

- 新模块 `rnd`（研发）
- 新权限点：`bom:view` / `bom:edit` / `bom:release`
- 矩阵：采购助理/品类经理 view+edit；品类经理/委员会 release；仓质/财务 view

## 分阶段

| 阶段 | 内容 |
|---|---|
| P1 | 产品 CRUD + BOM CRUD + 版本发布/生效替换 + 前端 BOM 页（明细编辑、版本历史） |
| P2 | MRP 产品需求展开 + 研发领料类型 + 成本汇总 |
| P3 | 多级 BOM（产品作子件）+ 替代料/损耗率应用 |

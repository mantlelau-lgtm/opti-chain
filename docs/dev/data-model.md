# 数据模型

GORM AutoMigrate 维护；SQLite/MySQL 双兼容。所有表含 `id / created_at / updated_at`（BaseModel）。

## 表清单（17）

### 基础数据

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `sys_material` | sku_code(UQ), name, unit, min_stock, max_stock | min_stock = MRP 安全库存 |
| `sys_supplier` | supplier_code(UQ), name, **audit_status**(默认 PENDING), status | 准入管控 |
| `base_customer` | customer_code(UQ), name, credit_limit, used_credit, audit_status(默认 APPROVED), status | 信用控制 |
| `sys_warehouse` | warehouse_code(UQ), name | |
| `sys_location` | warehouse_id, location_code(UQ), name | |

### 采购

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `pur_order` | po_number(UQ), supplier_id, order_date, expected_delivery_date, total_amount, status, created_by | 状态：DRAFT/APPROVED/IN_PROGRESS/COMPLETED/CANCELLED |
| `pur_order_detail` | po_id, material_id, order_qty, unit_price, **received_qty**, total_price | received_qty 只累计合格量 |
| `pur_receipt` | receipt_number(UQ, 默认 `RCV-<po>-<seq>`), po_id, warehouse_id, receipt_date, remark | 一轮收货一张 |
| `pur_receipt_detail` | receipt_id, po_detail_id, material_id, location_id, received_qty, **passed_qty, rejected_qty, reject_reason** | 拒收不入库 |

### 销售

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `sale_order` | so_number(UQ), customer_id, order_date, total_amount, status, created_by | DRAFT/APPROVED/IN_SHIPPING/COMPLETED/CANCELLED |
| `sale_order_detail` | so_id, material_id, qty, unit_price, shipped_qty, total_price | shipped_qty 预留（发货执行未实现） |

### 仓储

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `inv_stock` | warehouse_id, location_id, material_id, **quantity, locked_quantity** | 三元组唯一；可用 = quantity − locked |
| `inv_order` | order_number(UQ), order_type, ref_order_number, warehouse_id, status | PURCHASE_IN/SALE_OUT/TRANSFER |
| `inv_order_detail` | inv_order_id, material_id, location_id, qty | |
| `inv_transaction_log` | order_id, material_id, action_type(IN/OUT), change_qty, before_qty, after_qty | 不可变审计 |

### 计划

| 表 | 关键字段 | 说明 |
| --- | --- | --- |
| `plan_demand` | demand_number(UQ), material_id, demand_qty, demand_date, source_type(FORECAST/SALES_ORDER), status(OPEN/GENERATED) | |
| `plan_mrp_result` | batch_number, material_id, gross_demand, current_stock, on_order_qty, suggested_po_qty, status(SUGGESTED/CONVERTED), po_id | |

## 关键不变量

1. `pur_order_detail.received_qty` 只增合格量；超收由 `received_qty + ? <= order_qty` 守卫拒绝。
2. `inv_stock.locked_quantity` 只在 SO 审批/取消时增减；任何时刻 `locked ≤ quantity`。
3. 库存调整全部走带守卫的原子 UPDATE（`AdjustGated` / `LockRowInTx` / `UnlockRowInTx`），以 RowsAffected 判定成败。
4. 头+明细表写入一律 `Omit(clause.Associations)` + 显式明细循环，避免 GORM 级联双插。
5. 金额 `decimal(14,2)`、数量 `decimal(12,4)`，Go 侧 `shopspring/decimal`，JSON 以字符串传输。

## 数据流

```
需求(plan_demand) ─compute→ MRP(plan_mrp_result) ─convert→ pur_order(DRAFT)
pur_order ─approve→ 收货(pur_receipt) ─passed→ inv_stock(+)/inv_transaction_log
sale_order ─approve→ inv_stock.locked(+)/base_customer.used_credit(+)
sale_order ─cancel → 反向释放
```

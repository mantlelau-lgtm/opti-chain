# API 接口文档

## 基础信息

- Base URL：`http://<host>:8088/api/v1`（前端开发模式经 vite 代理 `/api`）
- 认证方式：无（v0.1 单用户场景，`created_by` 由调用方传入）
- 内容类型：`application/json`
- 数量/单价以**字符串**传输 decimal（如 `"6.5"`），避免浮点精度问题

## 响应信封

```json
{ "code": 0, "message": "ok", "data": ... }
```

| code | 含义 |
| --- | --- |
| 0 | 成功 |
| 40000 | 参数/业务规则错误（HTTP 200） |
| 40400 | 资源不存在（HTTP 404） |
| 40900 | 冲突：超卖/超限/并发状态变更（HTTP 409） |
| 50000 | 内部错误（HTTP 500） |

分页接口查询参数：`page`（默认 1）、`size`（默认 10）、`keyword`；响应 `data: {total, list}`。

## 基础数据

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET/POST | `/materials` | 列表（分页）/ 创建 |
| GET/PUT/DELETE | `/materials/:id` | 详情 / 更新 / 删除 |
| GET/POST | `/suppliers` | 列表 / 创建（audit_status 默认 PENDING） |
| GET/PUT/DELETE | `/suppliers/:id` | 详情 / 更新 / 删除 |
| PUT | `/suppliers/:id/audit` | 准入审核：`{"audit_status": "APPROVED|PENDING|REJECTED"}` |
| GET/POST | `/customers` | 列表 / 创建 |
| GET/PUT/DELETE | `/customers/:id` | 详情 / 更新（含 credit_limit、audit_status）/ 删除 |
| GET/POST | `/warehouses`、`/locations` | 同上 CRUD |

## 采购

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/po` | 列表（预载明细） |
| GET | `/po/:id` | 详情（含 details） |
| POST | `/po` | 创建（供应商须已核准；自动算 total） |
| PUT | `/po/:id` | 编辑：无 details → 仅表头；有 details → 全量替换 |
| PUT | `/po/:id/status` | 状态流转：`{"status": "..."}`（无方向校验，v0.2 加状态机） |
| DELETE | `/po/:id` | 删除 |
| POST | `/po/:id/receive` | **收货**（见下） |
| GET | `/po/:id/receipts` | 收货记录列表（含明细） |

收货请求：

```json
{
  "warehouse_id": 1, "remark": "第一车",
  "details": [
    { "po_detail_id": 3, "location_id": 0,
      "passed_qty": "6", "rejected_qty": "2", "reject_reason": "外包装破损" }
  ]
}
```

规则：仅 APPROVED/IN_PROGRESS 可收；`rejected_qty>0` 必须带原因；合格量累计 ≤ 订购量（超收 40000）；合格入库+流水，拒收不入库；全明细收满自动 COMPLETED。

## 销售

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET/POST | `/so` | 列表 / 创建（客户须已核准；DRAFT） |
| GET | `/so/:id` | 详情 |
| PUT | `/so/:id/approve` | 审批：锁定可用库存 + 占用信用（不足分别 40000/40900） |
| PUT | `/so/:id/cancel` | 取消：释放锁定 + 信用 |
| DELETE | `/so/:id` | 仅 DRAFT 可删 |

## 仓储

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/inventory/stock` | 实时库存（quantity / locked_quantity） |
| POST | `/inventory/move-in` | 入库：`{order_number, order_type, warehouse_id, details:[{material_id, location_id, qty}]}` |
| POST | `/inventory/move-out` | 出库（SALE_OUT 守卫实物不为负，不足 40000） |
| GET | `/inventory/orders`、`/inventory/orders/:id` | 出入库单据列表/详情 |
| DELETE | `/inventory/orders/:id` | 删单据（**不回冲库存**） |
| GET | `/inventory/logs` | 库存变动流水 |

## 计划

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET/POST | `/planning/demands` | 需求列表 / 创建（`demand_date` 支持 `YYYY-MM-DD`） |
| GET/PUT/DELETE | `/planning/demands/:id` | 详情 / 更新 / 删除 |
| GET | `/planning/mrp` | MRP 结果列表 |
| GET/DELETE | `/planning/mrp/:id` | 详情 / 删除 |
| POST | `/planning/mrp/compute` | 运算（OPEN 需求置 GENERATED） |
| POST | `/planning/mrp/:id/convert` | 建议转草稿 PO |

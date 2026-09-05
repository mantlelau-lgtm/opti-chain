package service

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"scm/internal/model"
	"scm/pkg/authx"
	"scm/pkg/query"
)

// ---- argument coercion helpers ----

func asStr(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func asUint(v any) uint {
	switch t := v.(type) {
	case float64:
		return uint(t)
	case int:
		return uint(t)
	case int64:
		return uint(t)
	case string:
		n, _ := strconv.ParseUint(t, 10, 64)
		return uint(n)
	case json.Number:
		n, _ := t.Int64()
		return uint(n)
	}
	return 0
}

func asDecimal(v any) decimal.Decimal {
	switch t := v.(type) {
	case string:
		d, err := decimal.NewFromString(t)
		if err != nil {
			return decimal.Zero
		}
		return d
	case float64:
		return decimal.NewFromFloat(t)
	case int:
		return decimal.NewFromInt(int64(t))
	case json.Number:
		d, _ := decimal.NewFromString(t.String())
		return d
	}
	return decimal.Zero
}

func asArr(m map[string]any, k string) []any {
	if v, ok := m[k].([]any); ok {
		return v
	}
	return nil
}

func pageFrom(args map[string]any) PageInput {
	p := int(asUint(args["page"]))
	sz := int(asUint(args["size"]))
	if p < 1 {
		p = 1
	}
	if sz < 1 {
		sz = 20
	}
	if sz > 200 {
		sz = 200
	}
	return PageInput{Page: query.Page{Page: p, Size: sz}, Keyword: asStr(args, "keyword")}
}

var listSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"keyword": map[string]any{"type": "string", "description": "关键字搜索（可选），匹配名称或编码"},
		"page":    map[string]any{"type": "integer", "description": "页码，默认 1"},
		"size":    map[string]any{"type": "integer", "description": "每页数量，默认 20"},
	},
}

// listTool builds a paginated list tool backed by a service List method.
func listTool[T any](name, desc, perm string, listFn func(uint, PageInput) ([]T, int64, error)) *AssistantTool {
	return &AssistantTool{
		Name:        name,
		Description: desc,
		Perm:        perm,
		Schema:      listSchema,
		Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
			items, total, err := listFn(actor.TenantID, pageFrom(args))
			if err != nil {
				return nil, err
			}
			return map[string]any{"total": total, "list": items}, nil
		},
	}
}

// registerAssistantTools builds the shared tool registry. Each tool carries a
// permission code; the agent offers a role-appropriate subset and the executor
// re-checks the caller's permission at execution time.
func registerAssistantTools(deps AssistantDeps) []*AssistantTool {
	return []*AssistantTool{
		listTool[model.Material]("material_list",
			"查询物料主数据，返回 id/sku_code/name/category/unit 等。创建采购单或 BOM 前先查物料拿到 id。",
			"material:view", deps.Materials.List),
		listTool[model.Supplier]("supplier_list",
			"查询供应商，返回 id/supplier_code/name/audit_status 等。注意：只有 audit_status=APPROVED 的供应商才能用于采购下单。",
			"supplier:view", deps.Suppliers.List),
		listTool[model.Product]("product_list",
			"查询产品主档，返回 id/product_code/name/unit 等。新建 BOM 前先查产品拿到 id。",
			"bom:view", deps.Products.List),
		listTool[model.PurchaseOrder]("po_list",
			"查询采购订单，返回 id/po_number/supplier_id/status/total_amount 等。",
			"po:view", deps.POs.List),
		listTool[model.Stock]("stock_list",
			"查询实时库存，返回 material_id/quantity/locked_quantity 等，用于判断物料库存。",
			"stock:view", deps.Stock.List),

		{
			Name:        "bom_list",
			Description: "查询 BOM（物料清单），返回 id/bom_no/product_id/version/status/unit_qty 及明细。可指定 product_id 只看某产品的 BOM。",
			Perm:        "bom:view",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"keyword":    map[string]any{"type": "string", "description": "关键字搜索（可选）"},
					"product_id": map[string]any{"type": "integer", "description": "产品 ID（可选），传入时返回该产品的 BOM 列表"},
					"page":       map[string]any{"type": "integer", "description": "页码，默认 1"},
					"size":       map[string]any{"type": "integer", "description": "每页数量，默认 20"},
				},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				if pid := asUint(args["product_id"]); pid > 0 {
					boms, err := deps.BOMs.ListByProduct(actor.TenantID, pid)
					if err != nil {
						return nil, err
					}
					return map[string]any{"total": len(boms), "list": boms}, nil
				}
				items, total, err := deps.BOMs.List(actor.TenantID, pageFrom(args))
				if err != nil {
					return nil, err
				}
				return map[string]any{"total": total, "list": items}, nil
			},
		},

		{
			Name:        "material_create",
			Description: "新建物料。必填 sku_code（租户内唯一）、name、category（分类）、unit（计量单位）。",
			Perm:        "material:manage",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"sku_code":  map[string]any{"type": "string", "description": "物料编码，租户内唯一"},
					"name":      map[string]any{"type": "string", "description": "物料名称"},
					"category":  map[string]any{"type": "string", "description": "物料分类"},
					"unit":      map[string]any{"type": "string", "description": "计量单位"},
					"min_stock": map[string]any{"type": "string", "description": "安全库存下限（可选）"},
					"max_stock": map[string]any{"type": "string", "description": "库存上限（可选）"},
				},
				"required": []string{"sku_code", "name", "category", "unit"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				m := &model.Material{
					SKUCode:  asStr(args, "sku_code"),
					Name:     asStr(args, "name"),
					Category: asStr(args, "category"),
					Unit:     asStr(args, "unit"),
					MinStock: asDecimal(args["min_stock"]),
					MaxStock: asDecimal(args["max_stock"]),
					Status:   1,
				}
				if err := deps.Materials.Create(actor.TenantID, m); err != nil {
					return nil, err
				}
				return m, nil
			},
		},

		{
			Name:        "material_update",
			Description: "更新物料。必填 id；其它字段只更新传入的（未传入的保留原值）。",
			Perm:        "material:manage",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":        map[string]any{"type": "integer", "description": "物料 ID"},
					"sku_code":  map[string]any{"type": "string", "description": "物料编码"},
					"name":      map[string]any{"type": "string", "description": "物料名称"},
					"category":  map[string]any{"type": "string", "description": "物料分类"},
					"unit":      map[string]any{"type": "string", "description": "计量单位"},
					"min_stock": map[string]any{"type": "string", "description": "安全库存下限"},
					"max_stock": map[string]any{"type": "string", "description": "库存上限"},
				},
				"required": []string{"id"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				id := asUint(args["id"])
				if id == 0 {
					return nil, errorsBadRequest("id is required")
				}
				old, err := deps.Materials.Get(actor.TenantID, id)
				if err != nil {
					return nil, err
				}
				if old == nil {
					return nil, errNotFound(id)
				}
				m := *old
				if v := asStr(args, "sku_code"); v != "" {
					m.SKUCode = v
				}
				if v := asStr(args, "name"); v != "" {
					m.Name = v
				}
				if v := asStr(args, "category"); v != "" {
					m.Category = v
				}
				if v := asStr(args, "unit"); v != "" {
					m.Unit = v
				}
				if v, ok := args["min_stock"]; ok {
					m.MinStock = asDecimal(v)
				}
				if v, ok := args["max_stock"]; ok {
					m.MaxStock = asDecimal(v)
				}
				if err := deps.Materials.Update(actor.TenantID, id, &m); err != nil {
					return nil, err
				}
				return m, nil
			},
		},

		{
			Name:        "bom_create",
			Description: "新建 BOM（物料清单）。必填 product_id（产品）和 details 组件明细数组；details 每项：component_id（组件物料 ID）、qty_per_unit（单位用量）、scrap_rate（损耗率，可选）。",
			Perm:        "bom:edit",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"bom_no":     map[string]any{"type": "string", "description": "BOM 编号，留空自动生成"},
					"product_id": map[string]any{"type": "integer", "description": "产品 ID"},
					"unit_qty":   map[string]any{"type": "string", "description": "单位成品数量，默认 1"},
					"remark":     map[string]any{"type": "string", "description": "备注（可选）"},
					"details":    map[string]any{"type": "array", "description": `组件明细，每项形如 {"component_id":1,"qty_per_unit":"2","scrap_rate":"0.02"}`},
				},
				"required": []string{"product_id", "details"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				var details []BOMDetailInput
				for _, it := range asArr(args, "details") {
					line, ok := it.(map[string]any)
					if !ok {
						continue
					}
					details = append(details, BOMDetailInput{
						ComponentID: asUint(line["component_id"]),
						QtyPerUnit:  asDecimal(line["qty_per_unit"]),
						ScrapRate:   asDecimal(line["scrap_rate"]),
						Remark:      asStr(line, "remark"),
					})
				}
				unitQty := asDecimal(args["unit_qty"])
				if unitQty.IsZero() {
					unitQty = decimal.NewFromInt(1)
				}
				bom, err := deps.BOMs.Create(actor.TenantID, BOMInput{
					BOMNo:     asStr(args, "bom_no"),
					ProductID: asUint(args["product_id"]),
					UnitQty:   unitQty,
					Remark:    asStr(args, "remark"),
					Details:   details,
				})
				if err != nil {
					return nil, err
				}
				return bom, nil
			},
		},

		{
			Name:        "po_create",
			Description: "新建采购订单（采购下单）。必填 supplier_id（须 APPROVED）和 details 明细数组；details 每项：material_id、order_qty（数量）、unit_price（单价）、location_id（可选库位）。",
			Perm:        "po:create",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"po_number":         map[string]any{"type": "string", "description": "采购单号，留空自动生成"},
					"supplier_id":       map[string]any{"type": "integer", "description": "供应商 ID"},
					"order_date":        map[string]any{"type": "string", "description": "下单日期 YYYY-MM-DD（可选，默认今天）"},
					"expected_delivery": map[string]any{"type": "string", "description": "期望交期 YYYY-MM-DD（可选）"},
					"details":           map[string]any{"type": "array", "description": `明细数组，每项形如 {"material_id":1,"order_qty":"100","unit_price":"5.5","location_id":2}`},
				},
				"required": []string{"supplier_id", "details"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				var details []PODetailInput
				for _, it := range asArr(args, "details") {
					line, ok := it.(map[string]any)
					if !ok {
						continue
					}
					details = append(details, PODetailInput{
						MaterialID: asUint(line["material_id"]),
						OrderQty:   asDecimal(line["order_qty"]),
						UnitPrice:  asDecimal(line["unit_price"]),
						LocationID: asUint(line["location_id"]),
					})
				}
				orderDate := time.Now()
				if v := asStr(args, "order_date"); v != "" {
					if t, err := time.Parse("2006-01-02", v); err == nil {
						orderDate = t
					}
				}
				var expected *time.Time
				if v := asStr(args, "expected_delivery"); v != "" {
					if t, err := time.Parse("2006-01-02", v); err == nil {
						expected = &t
					}
				}
				po, err := deps.POs.Create(actor.TenantID, CreatePOInput{
					PONumber:             asStr(args, "po_number"),
					SupplierID:           asUint(args["supplier_id"]),
					OrderDate:            orderDate,
					ExpectedDeliveryDate: expected,
					CreatedBy:            actor.Username,
					Details:              details,
				})
				if err != nil {
					return nil, err
				}
				return po, nil
			},
		},
	}
}

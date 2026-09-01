package main

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ---- argument helpers ----

func argMap(req mcp.CallToolRequest) map[string]any { return req.GetArguments() }

func strArg(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func boolArg(m map[string]any, k string) bool {
	if v, ok := m[k].(bool); ok {
		return v
	}
	return false
}

func numArg(m map[string]any, k string) float64 {
	switch v := m[k].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	}
	return 0
}

func uintArg(m map[string]any, k string) uint { return uint(numArg(m, k)) }

// intStr reads an integer arg and formats it as a string (with a default).
func intStr(m map[string]any, k string, def int) string {
	if _, ok := m[k]; !ok {
		return strconv.Itoa(def)
	}
	return strconv.Itoa(int(numArg(m, k)))
}

// decimalStr coerces a value to a decimal-friendly string. The backend accepts
// quantities/prices as strings (decimal columns), so numbers and strings both
// normalize here — the LLM may send either.
func decimalStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	}
	return ""
}

// uintVal coerces a value to a uint ID (LLM may send numbers or numeric strings).
func uintVal(v any) uint {
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

func arrArg(m map[string]any, k string) []any {
	if v, ok := m[k].([]any); ok {
		return v
	}
	return nil
}

// prettyData renders a successful payload as indented JSON text.
func prettyData(data json.RawMessage) *mcp.CallToolResult {
	return mcp.NewToolResultText(pretty(data))
}

func errResult(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

// registerTools wires every MCP tool. Descriptions are the primary interface
// for the LLM, so each one states what it does, when to use it, and the key
// constraints (e.g. supplier must be APPROVED before ordering).
func registerTools(s *server.MCPServer, c *Client) {
	registerQueryTools(s, c)
	registerBaseDataTools(s, c)
	registerOrderingTools(s, c)
}

// ---- query tools (reference lookups) ----

func registerQueryTools(s *server.MCPServer, c *Client) {
	registerList(s, c, "material_list",
		"查询物料主数据列表。返回物料的 id、sku_code(编码)、name(名称)、category(分类)、unit(单位)、min_stock/max_stock 等字段。"+
			"在新建采购订单、销售订单、BOM、供应关系之前，先用本工具按关键字查到目标物料，拿到其 id 再传给创建类工具。",
		"/api/v1/materials")

	registerList(s, c, "supplier_list",
		"查询供应商列表。返回 id、supplier_code(编码)、name(名称)、audit_status(准入状态)等。"+
			"注意：只有 audit_status=APPROVED 的供应商才能用于采购下单；新建供应商后默认为 PENDING，需人工审核通过后方可下单。",
		"/api/v1/suppliers")

	registerList(s, c, "warehouse_list",
		"查询仓库列表。返回 id、warehouse_code(编码)、name(名称)、address 等。用于新建库位时选择 warehouse_id。",
		"/api/v1/warehouses")

	registerList(s, c, "customer_list",
		"查询客户列表。返回 id、customer_code(编码)、name(名称)、credit_limit(信用额度)等。用于新建销售订单时确定 customer_id。",
		"/api/v1/customers")

	registerList(s, c, "product_list",
		"查询产品主档列表（R&D 成品）。返回 id、product_code(编码)、name(名称)、unit(单位)、cost_price 等。用于新建 BOM 或基于 BOM 下单时确定 product_id。",
		"/api/v1/products")

	registerList(s, c, "po_list",
		"查询采购订单列表。返回 id、po_number(单号)、supplier_id、status(状态)、total_amount 等。状态机：DRAFT 草稿 → APPROVED 已审批 → IN_PROGRESS 收货中 → COMPLETED 已完成 / CANCELLED 已取消。",
		"/api/v1/po")

	registerList(s, c, "stock_list",
		"查询实时库存列表。返回 material_id、quantity(可用数量)、locked_quantity(锁定数量，销售审批后锁定)等。用于判断某物料是否有足够库存。",
		"/api/v1/inventory/stock")

	// BOM list: supports an optional product filter via a dedicated endpoint.
	bomTool := mcp.NewTool("bom_list",
		mcp.WithDescription("查询 BOM（物料清单）列表。返回 id、bom_no(编号)、product_id、version(版本)、status(状态)、unit_qty 及明细 components。"+
			"当指定 product_id 时，只返回该产品的 BOM 版本列表；不指定则分页返回全部。新建 BOM 或基于 BOM 下单前先用本工具确认现有 BOM。"),
		mcp.WithNumber("product_id", mcp.Description("可选：产品 ID。传入时返回该产品的 BOM 列表，不传则分页返回全部 BOM")),
		mcp.WithNumber("page", mcp.Description("页码，默认 1")),
		mcp.WithNumber("size", mcp.Description("每页数量，默认 20")),
	)
	s.AddTool(bomTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		if pid := uintArg(m, "product_id"); pid > 0 {
			data, err := c.call("GET", "/api/v1/boms/product/"+strconv.Itoa(int(pid)), nil, nil)
			if err != nil {
				return errResult(err), nil
			}
			return prettyData(data), nil
		}
		q := url.Values{"page": {intStr(m, "page", 1)}, "size": {intStr(m, "size", 20)}}
		if kw := strArg(m, "keyword"); kw != "" {
			q.Set("keyword", kw)
		}
		data, err := c.call("GET", "/api/v1/boms", q, nil)
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})
}

// registerList registers a simple paginated GET list tool.
func registerList(s *server.MCPServer, c *Client, name, desc, path string) {
	tool := mcp.NewTool(name,
		mcp.WithDescription(desc),
		mcp.WithString("keyword", mcp.Description("可选：关键字搜索，匹配名称或编码")),
		mcp.WithNumber("page", mcp.Description("页码，默认 1")),
		mcp.WithNumber("size", mcp.Description("每页数量，默认 20")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		q := url.Values{"page": {intStr(m, "page", 1)}, "size": {intStr(m, "size", 20)}}
		if kw := strArg(m, "keyword"); kw != "" {
			q.Set("keyword", kw)
		}
		data, err := c.call("GET", path, q, nil)
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})
}

// ---- base-data creation tools ----

func registerBaseDataTools(s *server.MCPServer, c *Client) {
	s.AddTool(mcp.NewTool("material_create",
		mcp.WithDescription("新建物料主数据。物料是采购/销售/库存/BOM 的最小单元。必填 sku_code(物料编码，租户内唯一)、name、category(分类)、unit(计量单位)。"),
		mcp.WithString("sku_code", mcp.Required(), mcp.Description("物料编码，租户内唯一，如 MAT-001")),
		mcp.WithString("name", mcp.Required(), mcp.Description("物料名称")),
		mcp.WithString("category", mcp.Required(), mcp.Description("物料分类，如 电子料/结构件/包材")),
		mcp.WithString("unit", mcp.Required(), mcp.Description("计量单位，如 个/套/kg")),
		mcp.WithString("min_stock", mcp.Description("安全库存下限，数字字符串，如 '10'")),
		mcp.WithString("max_stock", mcp.Description("库存上限，数字字符串，如 '1000'")),
		mcp.WithNumber("status", mcp.Description("状态：1 启用(默认)，0 停用")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		body := map[string]any{
			"sku_code":  strArg(m, "sku_code"),
			"name":      strArg(m, "name"),
			"category":  strArg(m, "category"),
			"unit":      strArg(m, "unit"),
			"min_stock": decimalStr(m["min_stock"]),
			"max_stock": decimalStr(m["max_stock"]),
		}
		if v, ok := m["status"]; ok {
			body["status"] = uintVal(v)
		}
		data, err := c.call("POST", "/api/v1/materials", nil, body)
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("supplier_create",
		mcp.WithDescription("新建供应商。必填 supplier_code(供应商编码，租户内唯一)、name。新建后 audit_status 默认为 PENDING(待审核)，只有人工审核为 APPROVED 后该供应商才能用于采购下单。"),
		mcp.WithString("supplier_code", mcp.Required(), mcp.Description("供应商编码，租户内唯一，如 SUP-001")),
		mcp.WithString("name", mcp.Required(), mcp.Description("供应商名称")),
		mcp.WithString("contact_person", mcp.Description("联系人")),
		mcp.WithString("phone", mcp.Description("联系电话")),
		mcp.WithString("address", mcp.Description("地址")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		data, err := c.call("POST", "/api/v1/suppliers", nil, map[string]any{
			"supplier_code":  strArg(m, "supplier_code"),
			"name":           strArg(m, "name"),
			"contact_person": strArg(m, "contact_person"),
			"phone":          strArg(m, "phone"),
			"address":        strArg(m, "address"),
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("customer_create",
		mcp.WithDescription("新建客户。必填 customer_code(客户编码，租户内唯一)、name。credit_limit 为信用额度，<=0 表示不启用信用控制。"),
		mcp.WithString("customer_code", mcp.Required(), mcp.Description("客户编码，租户内唯一，如 CUS-001")),
		mcp.WithString("name", mcp.Required(), mcp.Description("客户名称")),
		mcp.WithString("contact_person", mcp.Description("联系人")),
		mcp.WithString("phone", mcp.Description("联系电话")),
		mcp.WithString("credit_limit", mcp.Description("信用额度，数字字符串，如 '100000'；留空或 0 表示不启用信用控制")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		data, err := c.call("POST", "/api/v1/customers", nil, map[string]any{
			"customer_code":  strArg(m, "customer_code"),
			"name":           strArg(m, "name"),
			"contact_person": strArg(m, "contact_person"),
			"phone":          strArg(m, "phone"),
			"credit_limit":   decimalStr(m["credit_limit"]),
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("warehouse_create",
		mcp.WithDescription("新建仓库。必填 warehouse_code(仓库编码，租户内唯一)、name。"),
		mcp.WithString("warehouse_code", mcp.Required(), mcp.Description("仓库编码，租户内唯一，如 WH-001")),
		mcp.WithString("name", mcp.Required(), mcp.Description("仓库名称")),
		mcp.WithString("address", mcp.Description("仓库地址")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		data, err := c.call("POST", "/api/v1/warehouses", nil, map[string]any{
			"warehouse_code": strArg(m, "warehouse_code"),
			"name":           strArg(m, "name"),
			"address":        strArg(m, "address"),
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("location_create",
		mcp.WithDescription("新建库位。必填 warehouse_id(所属仓库 ID，先查 warehouse_list 拿到)、location_code(库位编码)。"),
		mcp.WithNumber("warehouse_id", mcp.Required(), mcp.Description("所属仓库 ID")),
		mcp.WithString("location_code", mcp.Required(), mcp.Description("库位编码，如 A-01")),
		mcp.WithString("name", mcp.Description("库位名称")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		data, err := c.call("POST", "/api/v1/locations", nil, map[string]any{
			"warehouse_id":  uintArg(m, "warehouse_id"),
			"location_code": strArg(m, "location_code"),
			"name":          strArg(m, "name"),
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("product_create",
		mcp.WithDescription("新建产品主档（R&D 成品）。必填 product_code(产品编码，租户内唯一)、name、unit。产品不直接入库，其 BOM 驱动物料采购。"),
		mcp.WithString("product_code", mcp.Required(), mcp.Description("产品编码，租户内唯一，如 PRD-001")),
		mcp.WithString("name", mcp.Required(), mcp.Description("产品名称")),
		mcp.WithString("unit", mcp.Required(), mcp.Description("计量单位，如 台/套")),
		mcp.WithString("spec", mcp.Description("规格型号")),
		mcp.WithString("cost_price", mcp.Description("成本价，数字字符串")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		data, err := c.call("POST", "/api/v1/products", nil, map[string]any{
			"product_code": strArg(m, "product_code"),
			"name":         strArg(m, "name"),
			"unit":         strArg(m, "unit"),
			"spec":         strArg(m, "spec"),
			"cost_price":   decimalStr(m["cost_price"]),
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("bom_create",
		mcp.WithDescription("新建 BOM（物料清单，版本化）。必填 bom_no(编号)、product_id(产品 ID)、unit_qty(单位成品数量，默认 1)、以及 details 组件明细。"+
			"details 为数组，每个元素字段：component_id(组件物料 ID，必填)、qty_per_unit(单位用量，必填)、scrap_rate(损耗率，可选)。"+
			"新建的 BOM 为 DRAFT 草稿状态，需人工调用发布接口(bom:release 权限)转为 RELEASED 才能生效。"),
		mcp.WithString("bom_no", mcp.Required(), mcp.Description("BOM 编号")),
		mcp.WithNumber("product_id", mcp.Required(), mcp.Description("产品 ID（先查 product_list）")),
		mcp.WithString("unit_qty", mcp.Description("单位成品数量，默认 '1'")),
		mcp.WithString("remark", mcp.Description("备注")),
		mcp.WithArray("details", mcp.Required(), mcp.Description(`组件明细数组，每项形如 {"component_id":1,"qty_per_unit":"2","scrap_rate":"0.02"}`)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		var details []map[string]any
		for _, it := range arrArg(m, "details") {
			line, ok := it.(map[string]any)
			if !ok {
				continue
			}
			details = append(details, map[string]any{
				"component_id": uintVal(line["component_id"]),
				"qty_per_unit": decimalStr(line["qty_per_unit"]),
				"scrap_rate":   decimalStr(line["scrap_rate"]),
				"remark":       strArg(line, "remark"),
			})
		}
		data, err := c.call("POST", "/api/v1/boms", nil, map[string]any{
			"bom_no":     strArg(m, "bom_no"),
			"product_id": uintArg(m, "product_id"),
			"unit_qty":   decimalStr(m["unit_qty"]),
			"remark":     strArg(m, "remark"),
			"details":    details,
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("supplier_material_bind",
		mcp.WithDescription("绑定供应商与物料的供应关系（含单价、交期、是否首选）。同一个供应商+物料组合重复绑定会更新价格而非报错。绑定后，基于 BOM 下单时系统会自动按首选/最低价选择供应商。"),
		mcp.WithNumber("supplier_id", mcp.Required(), mcp.Description("供应商 ID（先查 supplier_list）")),
		mcp.WithNumber("material_id", mcp.Required(), mcp.Description("物料 ID（先查 material_list）")),
		mcp.WithString("unit_price", mcp.Required(), mcp.Description("供应单价，数字字符串，如 '12.5'")),
		mcp.WithNumber("lead_time_days", mcp.Description("交期天数")),
		mcp.WithBoolean("is_preferred", mcp.Description("是否首选供应商，默认 false")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		data, err := c.call("POST", "/api/v1/supplier-material", nil, map[string]any{
			"supplier_id":    uintArg(m, "supplier_id"),
			"material_id":    uintArg(m, "material_id"),
			"unit_price":     decimalStr(m["unit_price"]),
			"lead_time_days": uintArg(m, "lead_time_days"),
			"is_preferred":   boolArg(m, "is_preferred"),
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})
}

// ---- ordering tools ----

func registerOrderingTools(s *server.MCPServer, c *Client) {
	s.AddTool(mcp.NewTool("po_create",
		mcp.WithDescription("新建采购订单。必填 supplier_id(供应商 ID，须为 APPROVED 状态)和 details 明细数组。"+
			"details 每项：material_id(物料 ID)、order_qty(订购数量)、unit_price(单价)、location_id(可选库位)。"+
			"新建后为 DRAFT 草稿状态，走审批流生效。单号 po_number 可留空由系统自动生成。"),
		mcp.WithString("po_number", mcp.Description("采购单号，留空则系统自动生成")),
		mcp.WithNumber("supplier_id", mcp.Required(), mcp.Description("供应商 ID（先查 supplier_list，须 APPROVED）")),
		mcp.WithString("order_date", mcp.Description("下单日期，YYYY-MM-DD，默认今天")),
		mcp.WithString("expected_delivery", mcp.Description("期望交期，YYYY-MM-DD")),
		mcp.WithArray("details", mcp.Required(), mcp.Description(`明细数组，每项形如 {"material_id":1,"order_qty":"100","unit_price":"5.5","location_id":2}`)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		var details []map[string]any
		for _, it := range arrArg(m, "details") {
			line, ok := it.(map[string]any)
			if !ok {
				continue
			}
			details = append(details, map[string]any{
				"material_id": uintVal(line["material_id"]),
				"order_qty":   decimalStr(line["order_qty"]),
				"unit_price":  decimalStr(line["unit_price"]),
				"location_id": uintVal(line["location_id"]),
			})
		}
		data, err := c.call("POST", "/api/v1/po", nil, map[string]any{
			"po_number":         strArg(m, "po_number"),
			"supplier_id":       uintArg(m, "supplier_id"),
			"order_date":        strArg(m, "order_date"),
			"expected_delivery": strArg(m, "expected_delivery"),
			"details":           details,
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("so_create",
		mcp.WithDescription("新建销售订单。必填 customer_id(客户 ID)和 details 明细数组。"+
			"details 每项：material_id(物料 ID)、qty(数量)、unit_price(单价)。"+
			"新建后为 DRAFT 草稿，审批后锁库并占用信用额度。单号 so_number 可留空自动生成。"),
		mcp.WithString("so_number", mcp.Description("销售单号，留空则系统自动生成")),
		mcp.WithNumber("customer_id", mcp.Required(), mcp.Description("客户 ID（先查 customer_list）")),
		mcp.WithString("order_date", mcp.Description("下单日期，YYYY-MM-DD，默认今天")),
		mcp.WithArray("details", mcp.Required(), mcp.Description(`明细数组，每项形如 {"material_id":1,"qty":"10","unit_price":"99.9"}`)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		var details []map[string]any
		for _, it := range arrArg(m, "details") {
			line, ok := it.(map[string]any)
			if !ok {
				continue
			}
			details = append(details, map[string]any{
				"material_id": uintVal(line["material_id"]),
				"qty":         decimalStr(line["qty"]),
				"unit_price":  decimalStr(line["unit_price"]),
			})
		}
		data, err := c.call("POST", "/api/v1/so", nil, map[string]any{
			"so_number":   strArg(m, "so_number"),
			"customer_id": uintArg(m, "customer_id"),
			"order_date":  strArg(m, "order_date"),
			"details":     details,
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	// BOM ordering: preview + confirm share the same items payload.
	bomItemsDesc := `items 数组，每项形如 {"product_id":1,"qty":"4"}，表示按该产品默认 BOM 下单指定数量。可同时下多个产品（如 bom1×4 + bom2×5 + bom3×6），系统会聚合物料需求并按供应商拆单。`

	s.AddTool(mcp.NewTool("bom_order_preview",
		mcp.WithDescription("基于 BOM 下单的【预览拆单】：根据多个产品及数量展开默认 BOM、聚合物料需求，并自动为每个物料选择供应商（首选 > 最低价），返回按供应商聚合的预览结果，但不创建任何订单。"+
			"确认下单前先调用本工具让用户/agent 核对拆单结果，再用 bom_order_confirm 正式下单。"+bomItemsDesc),
		mcp.WithArray("items", mcp.Required(), mcp.Description(bomItemsDesc)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		items := bomItems(arrArg(m, "items"))
		data, err := c.call("POST", "/api/v1/bom-order/preview", nil, map[string]any{"items": items})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})

	s.AddTool(mcp.NewTool("bom_order_confirm",
		mcp.WithDescription("基于 BOM 下单的【确认下单】：根据多个产品及数量展开 BOM、选供应商，最终按供应商拆成多张草稿采购单并落库。"+
			"建议先调用 bom_order_preview 核对，确认无误后再调用本工具。"+bomItemsDesc),
		mcp.WithArray("items", mcp.Required(), mcp.Description(bomItemsDesc)),
		mcp.WithString("order_date", mcp.Description("下单日期，YYYY-MM-DD，默认今天")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		m := req.GetArguments()
		items := bomItems(arrArg(m, "items"))
		data, err := c.call("POST", "/api/v1/bom-order/confirm", nil, map[string]any{
			"items":      items,
			"order_date": strArg(m, "order_date"),
		})
		if err != nil {
			return errResult(err), nil
		}
		return prettyData(data), nil
	})
}

// bomItems normalizes the BOM-order items array (product_id + qty).
func bomItems(raw []any) []map[string]any {
	var out []map[string]any
	for _, it := range raw {
		line, ok := it.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"product_id": uintVal(line["product_id"]),
			"qty":        decimalStr(line["qty"]),
		})
	}
	return out
}

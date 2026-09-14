package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"scm/internal/model"
	"scm/pkg/authx"
	"scm/pkg/query"
)

// ---- argument coercion helpers ----

func asStr(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	v, ok := m[k]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case fmt.Stringer:
		return t.String()
	case nil:
		return ""
	case bool:
		if t {
			return "true"
		}
		return "false"
	case json.Number:
		return t.String()
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case float64:
		// Integers encoded as JSON numbers without decimal part should come out
		// without ".0" so SKU codes like "QJ00001" vs 1 (numeric) don't get
		// treated as different strings.
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.FormatInt(int64(t), 10)
	case int8:
		return strconv.FormatInt(int64(t), 10)
	case int16:
		return strconv.FormatInt(int64(t), 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint8:
		return strconv.FormatUint(uint64(t), 10)
	case uint16:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	}
	// Unknown type: try %v as last resort to never silently drop a value.
	return fmt.Sprintf("%v", v)
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

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	case bool:
		if t {
			return 1
		}
		return 0
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
	if sz > 50 {
		sz = 50
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

// extractListSample pulls a short summary (id + identifier + name) from a list
// item returned by listTool[T]. It is intentionally reflection-free and uses a
// best-effort type switch against the known list models.
func extractListSample(item any) map[string]any {
	switch v := item.(type) {
	case model.Material:
		return map[string]any{"id": v.ID, "sku_code": v.SKUCode, "name": v.Name, "category": v.Category}
	case model.Supplier:
		return map[string]any{"id": v.ID, "supplier_code": v.SupplierCode, "name": v.Name, "audit_status": v.AuditStatus}
	case model.Product:
		return map[string]any{"id": v.ID, "product_code": v.ProductCode, "name": v.Name, "spec": v.Spec}
	case model.PurchaseOrder:
		return map[string]any{"id": v.ID, "po_number": v.PONumber, "supplier_id": v.SupplierID, "status": v.Status}
	case model.Stock:
		return map[string]any{"id": v.ID, "warehouse_id": v.WarehouseID, "material_id": v.MaterialID, "quantity": v.Quantity}
	case model.SaleOrder:
		return map[string]any{"id": v.ID, "so_number": v.SONumber, "customer_id": v.CustomerID, "status": v.Status}
	case model.Customer:
		return map[string]any{"id": v.ID, "customer_code": v.CustomerCode, "name": v.Name, "phone": v.Phone}
	}
	// unknown T → fall back to reflection on common fields via map round-trip
	raw, err := json.Marshal(item)
	if err != nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	s := map[string]any{}
	for _, k := range []string{"id", "code", "sku_code", "supplier_code", "product_code", "po_number", "so_number", "customer_code", "name", "status", "material_id", "customer_id", "supplier_id"} {
		if v, ok := m[k]; ok && v != nil {
			s[k] = v
		}
	}
	if len(s) == 0 {
		return nil
	}
	return s
}

// listTool builds a paginated list tool backed by a service List method.
//
// To keep the LLM context compact and avoid repeated "翻页" (page-flip) loops
// (which were observed to hit assistantMaxIters with a single tool name
// repeated 12x), the tool returns a condensed response:
//
//   - total: the full count so the caller knows whether to narrow the search
//   - sample: the first 20 entries reduced to id+identifier+name (tiny)
//   - list: the first 8 full entries (enough to render a concrete answer)
//   - more_hint: a Chinese hint the model can surface to the user
//
// The LLM is expected to narrow via keyword instead of paging.
func listTool[T any](name, desc, perm string, listFn func(context.Context, uint, PageInput) ([]T, int64, error)) *AssistantTool {
	const (
		sampleSize = 20
		listSize   = 8
	)
	return &AssistantTool{
		Name:        name,
		Description: desc,
		Perm:        perm,
		Schema:      listSchema,
		Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
			items, total, err := listFn(context.Background(), actor.TenantID, pageFrom(args))
			if err != nil {
				return nil, err
			}
			sampleN := len(items)
			if sampleN > sampleSize {
				sampleN = sampleSize
			}
			listN := len(items)
			if listN > listSize {
				listN = listSize
			}
			sample := make([]map[string]any, 0, sampleN)
			for i := 0; i < sampleN; i++ {
				if s := extractListSample(items[i]); s != nil {
					sample = append(sample, s)
				}
			}
			listSubset := items[:listN]
			hint := ""
			if total > int64(listSize) {
				hint = fmt.Sprintf("共 %d 条，当前仅返回前 %d 条完整对象与前 %d 条摘要。若需要精确结果，请再报更具体的名称 / 编码（SKU、PO、SO、供应商编号等），通过 keyword 参数缩小范围，不要逐页翻。", total, listSize, sampleN)
			}
			return map[string]any{
				"total":      total,
				"returned":   listN,
				"sample":     sample,
				"list":       listSubset,
				"more_hint":  hint,
				"_page":      pageFrom(args).Page.Page,
				"_page_size": pageFrom(args).Page.Size,
			}, nil
		},
	}
}

// registerAssistantTools builds the shared tool registry. Each tool carries a
// permission code; the agent offers a role-appropriate subset and the executor
// re-checks the caller's permission at execution time.
func registerAssistantTools(deps AssistantDeps) []*AssistantTool {
	tools := []*AssistantTool{
		listTool[model.Material]("material_list",
			"查询物料主数据，返回 id/sku_code/name/category/unit 等。创建采购单或 BOM 前先查物料拿到 id。强烈建议：用 keyword 传 SKU 编码或名称的关键词精确匹配；不要逐页翻页。若返回结果偏多，请让用户再报具体的编码/名称，用 keyword 缩小范围。",
			"material:view", deps.Materials.List),
		listTool[model.Supplier]("supplier_list",
			"查询供应商，返回 id/supplier_code/name/audit_status 等。注意：只有 audit_status=APPROVED 的供应商才能用于采购下单。建议用 keyword 传供应商编号/名称关键词精确匹配；不要整页翻。",
			"supplier:view", deps.Suppliers.List),
		listTool[model.Product]("product_list",
			"查询产品主档，返回 id/product_code/name/unit 等。新建 BOM 前先查产品拿到 id。建议用 keyword 传产品编号或名称关键词精确匹配；不要整页翻。",
			"bom:view", deps.Products.List),
		listTool[model.PurchaseOrder]("po_list",
			"查询采购订单，返回 id/po_number/supplier_id/status/total_amount 等。建议用 keyword 传 PO 编号或供应商名关键词；不要逐页翻。",
			"po:view", deps.POs.List),
		listTool[model.Stock]("stock_list",
			"查询实时库存，返回 material_id/quantity/locked_quantity 等，用于判断物料库存。建议用 keyword 传物料名/SKU 编码；不要整页翻。",
			"stock:view", deps.Stock.List),
		listTool[model.SaleOrder]("so_list",
			"查询销售订单，返回 id/so_number/customer_id/status/total_amount 等。查物流时先用此工具拿到订单关联的 customer_id，再用 customer_get 取手机号。建议用 keyword 传 SO 号/客户名关键词；不要整页翻。",
			"so:view", deps.SOs.List),
		listTool[model.Customer]("customer_list",
			"查询客户列表，返回 id/customer_code/name/phone/audit_status/credit_limit 等。查物流时用客户 phone 字段（取后 4 位）配运单号调用顺丰接口。建议用 keyword 传客户编号/名称关键词；不要整页翻。",
			"customer:view", deps.Customers.List),

		{
			Name:        "customer_get",
			Description: "根据客户 id 查询客户详情，返回 contact_person/phone（手机号）等。查物流顺丰接口需要手机号后 4 位，可从这里取。",
			Perm:        "customer:view",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "integer", "description": "客户 ID，必填"},
				},
				"required": []string{"id"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				id := asUint(args["id"])
				if id == 0 {
					return nil, errorsBadRequest("id is required")
				}
				return deps.Customers.Get(context.Background(), actor.TenantID, id)
			},
		},

		{
			Name:        "logistics_query",
			Description: "调用顺丰接口查询运单轨迹。必填 tracking_no（运单号）；选填 phone_no（收件人手机号，建议传后 4 位以防信息泄露，也可传完整号）。若未直接拿到手机号，请先通过 so_list 找到对应销售订单的 customer_id，再用 customer_get 取客户 phone 字段。",
			Perm:        "logistics:view",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tracking_no": map[string]any{"type": "string", "description": "顺丰运单号，必填，如 SF1234567890"},
					"phone_no":    map[string]any{"type": "string", "description": "收件人手机号或后 4 位，选填（部分运单校验需要），例如 1380"},
				},
				"required": []string{"tracking_no"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				trackingNo := asStr(args, "tracking_no")
				if trackingNo == "" {
					return nil, errorsBadRequest("tracking_no is required")
				}
				return deps.Logistics.QueryOne(context.Background(), actor.TenantID, actor.UserID, trackingNo)
			},
		},

		{
			Name:        "logistics_batch_query",
			Description: "批量查询多个运单轨迹（一次 ≤ 10 个，更多自动分批）。必填 tracking_nos 数组；选填 phone_no（同批所有运单共用的手机号后 4 位）。",
			Perm:        "logistics:view",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"tracking_nos": map[string]any{"type": "array", "description": "运单号字符串数组，必填", "items": map[string]any{"type": "string"}},
					"phone_no":     map[string]any{"type": "string", "description": "共用的收件人手机号或后 4 位，选填"},
				},
				"required": []string{"tracking_nos"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				raw := asArr(args, "tracking_nos")
				if len(raw) == 0 {
					return nil, errorsBadRequest("tracking_nos is required")
				}
				var nos []string
				for _, r := range raw {
					if s, ok := r.(string); ok && s != "" {
						nos = append(nos, s)
					}
				}
				if len(nos) == 0 {
					return nil, errorsBadRequest("tracking_nos is empty")
				}
				return deps.Logistics.QueryBatch(context.Background(), actor.TenantID, actor.UserID, nos, 1, asStr(args, "phone_no"))
			},
		},

		{
			Name:        "logistics_history",
			Description: "查看本用户已查询过的物流历史记录（带分页），可回看运单最新状态与轨迹，无需再调顺丰接口。",
			Perm:        "logistics:view",
			Schema:      listSchema,
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				p := pageFrom(args)
				list, total, err := deps.Logistics.List(context.Background(), actor.TenantID, actor.UserID, p.Page.Page, p.Page.Size)
				if err != nil {
					return nil, err
				}
				return map[string]any{"total": total, "list": list}, nil
			},
		},

		{
			Name:        "document_list",
			Description: "列出当前用户最近上传过的文件（最近 50 个），返回 name/path/size/ext/mtime。不知道文件路径时先调这个拿 path 再喂给 document_parse / document_summary。",
			Perm:        "",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer", "description": "返回条数上限，默认 50"},
				},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				limit := asInt(args["limit"])
				if limit <= 0 {
					limit = 50
				}
				dir := getUploadDir()
				return deps.Docs.ListSessionAttachments(dir, limit)
			},
		},

		{
			Name:        "document_parse",
			Description: "解析单个上传文件为结构化文本，返回 kind/pages/sheets/paragraphs/tables/full_text/preview。支持 txt/md/csv/json/pdf/docx/xlsx/xls/log。参数：path 或 name 任选一个（从 document_list 的返回里取）；可选 include_tables=true 附带完整表格。",
			Perm:        "",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":           map[string]any{"type": "string", "description": "文件绝对路径（从 document_list 拿），优先"},
					"name":           map[string]any{"type": "string", "description": "文件名（用于展示和匹配）"},
					"include_tables": map[string]any{"type": "boolean", "description": "是否返回完整表格二维数组（默认 true，大表格可能费 token）"},
					"max_chars":      map[string]any{"type": "integer", "description": "返回的 full_text 最大字符数，默认不限制（后端仍有 2MB 硬上限）"},
				},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				path := asStr(args, "path")
				name := asStr(args, "name")
				path, err := resolveDocPath(path, name)
				if err != nil {
					return nil, errorsBadRequest(err.Error())
				}
				parsed, err := deps.Docs.ParseFile(context.Background(), path, name)
				if err != nil {
					return nil, err
				}
				maxChars := asInt(args["max_chars"])
				if maxChars > 0 && len(parsed.FullText) > maxChars {
					parsed.FullText = parsed.FullText[:maxChars]
				}
				if b, ok := args["include_tables"].(bool); !ok || !b {
					parsed.Tables = nil
				}
				return parsed, nil
			},
		},

		{
			Name:        "document_summary",
			Description: "对 document_parse 的结果做紧凑摘要：给出类型/页数/工作表列表+前 10 个段落+每个表的前 3 行样例，省 token。建议第一次看文档先调这个，再决定是否需要 parse 全文。",
			Perm:        "",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":      map[string]any{"type": "string", "description": "文件绝对路径，优先"},
					"name":      map[string]any{"type": "string", "description": "文件名"},
					"max_chars": map[string]any{"type": "integer", "description": "摘要字符上限，默认 6000"},
				},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				path, err := resolveDocPath(asStr(args, "path"), asStr(args, "name"))
				if err != nil {
					return nil, errorsBadRequest(err.Error())
				}
				parsed, err := deps.Docs.ParseFile(context.Background(), path, asStr(args, "name"))
				if err != nil {
					return nil, err
				}
				mc := asInt(args["max_chars"])
				return map[string]any{
					"file_name": parsed.FileName,
					"kind":      parsed.Kind,
					"size":      parsed.Size,
					"pages":     parsed.Pages,
					"sheets":    parsed.Sheets,
					"preview":   parsed.Preview,
					"summary":   deps.Docs.Summary(parsed, mc),
				}, nil
			},
		},

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
					boms, err := deps.BOMs.ListByProduct(context.Background(), actor.TenantID, pid)
					if err != nil {
						return nil, err
					}
					return map[string]any{"total": len(boms), "list": boms}, nil
				}
				items, total, err := deps.BOMs.List(context.Background(), actor.TenantID, pageFrom(args))
				if err != nil {
					return nil, err
				}
				return map[string]any{"total": total, "list": items}, nil
			},
		},

		{
			Name:        "material_create",
			Description: "新建【单条】物料。仅当明确只创建 1~2 条物料时使用；需要批量创建/批量导入请直接调用 material_batch_create（单事务保证要么全成要么全败，省 token 且不会因反复调用查询工具而卡壳）。",
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
				if err := deps.Materials.Create(context.Background(), actor.TenantID, m); err != nil {
					return nil, err
				}
				return m, nil
			},
		},

		{
			Name:        "material_batch_create",
			Description: "批量创建物料（单次最多 500 条）。【这是批量写入物料的首选/唯一推荐工具】，底层用单事务包裹：要么 N 条全部落库成功，要么全部回滚，不会出现半成功半失败导致的 SKU 冲突。传 items 数组，每一项：sku_code、name、category、unit、可选 min_stock/max_stock。提示：不要先调用 material_list 翻全量目录再用 material_create 逐条建——那样会大量浪费 token 并很容易超过处理步数。",
			Perm:        "material:manage",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type":        "array",
						"description": `物料数组，每一项形如 {"sku_code":"M-001","name":"螺钉","category":"五金","unit":"个","min_stock":"100","max_stock":"5000"}。长度 1~500，每一项必填 sku_code/name/category/unit。`,
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"sku_code":  map[string]any{"type": "string"},
								"name":      map[string]any{"type": "string"},
								"category":  map[string]any{"type": "string"},
								"unit":      map[string]any{"type": "string"},
								"min_stock": map[string]any{"type": "string"},
								"max_stock": map[string]any{"type": "string"},
							},
							"required": []string{"sku_code", "name", "category", "unit"},
						},
					},
				},
				"required": []string{"items"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				rawItems := asArr(args, "items")
				if len(rawItems) == 0 {
					return nil, errorsBadRequest("items is required and must be a non-empty array")
				}
				list := make([]model.Material, 0, len(rawItems))
				for i, it := range rawItems {
					line, ok := it.(map[string]any)
					if !ok {
						return nil, errorsBadRequest(fmt.Sprintf("items[%d] is not an object", i))
					}
					m := model.Material{
						SKUCode:  asStr(line, "sku_code"),
						Name:     asStr(line, "name"),
						Category: asStr(line, "category"),
						Unit:     asStr(line, "unit"),
						MinStock: asDecimal(line["min_stock"]),
						MaxStock: asDecimal(line["max_stock"]),
						Status:   1,
					}
					// Pre-validation with diagnostic logging: if a required field
					// is empty after coercion, dump the raw line value types so
					// we can immediately tell whether the LLM handed us a
					// numeric type that didn't stringify correctly, or a
					// genuinely missing key.
					if m.SKUCode == "" || m.Name == "" || m.Unit == "" {
						rawJSON, _ := json.Marshal(line)
						types := map[string]string{}
						for k, v := range line {
							types[k] = fmt.Sprintf("%T", v)
						}
						zap.L().Warn("[DBG material_batch_create coercion empty]",
							zap.Int("index", i),
							zap.String("sku_code", m.SKUCode),
							zap.String("name", m.Name),
							zap.String("unit", m.Unit),
							zap.String("raw_line_json", string(rawJSON)),
							zap.Any("value_types", types),
						)
						return nil, errorsBadRequest(fmt.Sprintf(
							"items[%d] 缺少必需字段 sku_code/name/unit 中的一个或多个（原始值：%s；各字段类型：%v），请补充后再试",
							i, string(rawJSON), types,
						))
					}
					list = append(list, m)
				}
				created, err := deps.Materials.CreateBatch(context.Background(), actor.TenantID, list)
				if err != nil {
					return nil, err
				}
				ids := make([]uint, 0, len(created))
				skus := make([]string, 0, len(created))
				for _, m := range created {
					ids = append(ids, m.ID)
					skus = append(skus, m.SKUCode)
				}
				return map[string]any{
					"count":      len(created),
					"ids":        ids,
					"sku_codes":  skus,
					"sample":     extractListSample(created[0]), // reuse helper, len>=1 per checks above
					"all_items":  created,
					"_tx_status": "atomic: all items committed or none",
				}, nil
			},
		},

		{
			Name:        "supplier_create",
			Description: "新建【单条】供应商。仅当 1~2 条时使用；批量创建/批量导入请直接调用 supplier_batch_create（单事务要么全成要么全败，省 token 防卡壳）。",
			Perm:        "supplier:manage",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"supplier_code":  map[string]any{"type": "string", "description": "供应商编码（必填，唯一）"},
					"name":           map[string]any{"type": "string", "description": "供应商名称（必填）"},
					"contact_person": map[string]any{"type": "string", "description": "联系人（可选）"},
					"phone":          map[string]any{"type": "string", "description": "联系电话（可选）"},
					"address":        map[string]any{"type": "string", "description": "地址（可选）"},
					"audit_status":   map[string]any{"type": "string", "description": "准入状态 PENDING/APPROVED，默认 PENDING（采购下单前必须 APPROVED）"},
				},
				"required": []string{"supplier_code", "name"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				m := &model.Supplier{
					SupplierCode:  asStr(args, "supplier_code"),
					Name:          asStr(args, "name"),
					ContactPerson: asStr(args, "contact_person"),
					Phone:         asStr(args, "phone"),
					Address:       asStr(args, "address"),
					AuditStatus:   asStr(args, "audit_status"),
					Status:        1,
				}
				if m.AuditStatus == "" {
					m.AuditStatus = model.AuditPending
				}
				if err := deps.Suppliers.Create(context.Background(), actor.TenantID, m); err != nil {
					return nil, err
				}
				return m, nil
			},
		},
		{
			Name:        "supplier_batch_create",
			Description: "【批量首选】一次性提交多条供应商，单事务保证要么全成要么全回滚。严禁先调 supplier_list 翻页再一条条 supplier_create 反复刷。items 上限 500 条。",
			Perm:        "supplier:manage",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type":        "array",
						"description": `供应商数组，每项形如 {"supplier_code":"SUP-001","name":"华东五金","contact_person":"张工","phone":"138...","address":"","audit_status":"PENDING"}。每条必填 supplier_code、name`,
					},
				},
				"required": []string{"items"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				rawItems := asArr(args, "items")
				if len(rawItems) == 0 {
					return nil, errorsBadRequest("items is required and must be non-empty array")
				}
				if len(rawItems) > 500 {
					return nil, errorsBadRequest(fmt.Sprintf("batch size exceeds 500: got %d", len(rawItems)))
				}
				list := make([]model.Supplier, 0, len(rawItems))
				for i, it := range rawItems {
					line, ok := it.(map[string]any)
					if !ok {
						return nil, errorsBadRequest(fmt.Sprintf("items[%d] is not an object", i))
					}
					m := model.Supplier{
						SupplierCode:  asStr(line, "supplier_code"),
						Name:          asStr(line, "name"),
						ContactPerson: asStr(line, "contact_person"),
						Phone:         asStr(line, "phone"),
						Address:       asStr(line, "address"),
						AuditStatus:   asStr(line, "audit_status"),
						Status:        1,
					}
					if m.AuditStatus == "" {
						m.AuditStatus = model.AuditPending
					}
					if m.SupplierCode == "" || m.Name == "" {
						rawJSON, _ := json.Marshal(line)
						types := map[string]string{}
						for k, v := range line {
							types[k] = fmt.Sprintf("%T", v)
						}
						zap.L().Warn("[DBG supplier_batch_create coercion empty]",
							zap.Int("index", i),
							zap.String("supplier_code", m.SupplierCode),
							zap.String("name", m.Name),
							zap.String("raw_line_json", string(rawJSON)),
							zap.Any("value_types", types),
						)
						return nil, errorsBadRequest(fmt.Sprintf(
							"items[%d] 缺少必需字段 supplier_code/name（原始值：%s；各字段类型：%v），请补充后再试",
							i, string(rawJSON), types,
						))
					}
					list = append(list, m)
				}
				created, err := deps.Suppliers.CreateBatch(context.Background(), actor.TenantID, list)
				if err != nil {
					return nil, err
				}
				ids := make([]uint, 0, len(created))
				codes := make([]string, 0, len(created))
				for _, m := range created {
					ids = append(ids, m.ID)
					codes = append(codes, m.SupplierCode)
				}
				return map[string]any{
					"count":      len(created),
					"ids":        ids,
					"codes":      codes,
					"sample":     extractListSample(created[0]),
					"all_items":  created,
					"_tx_status": "atomic: all items committed or none",
				}, nil
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
				old, err := deps.Materials.Get(context.Background(), actor.TenantID, id)
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
				if err := deps.Materials.Update(context.Background(), actor.TenantID, id, &m); err != nil {
					return nil, err
				}
				return m, nil
			},
		},

		{
			Name:        "product_create",
			Description: "新建【单条】产品。仅当 1~2 条时使用；批量创建/批量导入请直接调用 product_batch_create（单事务要么全成要么全败，省 token 防卡壳）。",
			Perm:        "bom:edit",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"product_code": map[string]any{"type": "string", "description": "产品编码（必填，唯一）"},
					"name":         map[string]any{"type": "string", "description": "产品名称（必填）"},
					"spec":         map[string]any{"type": "string", "description": "规格型号（可选）"},
					"unit":         map[string]any{"type": "string", "description": "计量单位（必填）"},
					"cost_price":   map[string]any{"type": "string", "description": "成本价 decimal（可选，默认 0）"},
				},
				"required": []string{"product_code", "name", "unit"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				m := &model.Product{
					ProductCode: asStr(args, "product_code"),
					Name:        asStr(args, "name"),
					Spec:        asStr(args, "spec"),
					Unit:        asStr(args, "unit"),
					CostPrice:   asDecimal(args["cost_price"]),
					Status:      1,
				}
				if err := deps.Products.Create(context.Background(), actor.TenantID, m); err != nil {
					return nil, err
				}
				return m, nil
			},
		},
		{
			Name:        "product_batch_create",
			Description: "【批量首选】一次性提交多条产品，单事务保证要么全成要么全回滚。严禁先调 product_list 翻页再一条条 product_create 反复刷。items 上限 500 条。",
			Perm:        "bom:edit",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type":        "array",
						"description": `产品数组，每项形如 {"product_code":"P-001","name":"智能扫地机X1","spec":"黑色 3L","unit":"台","cost_price":"399.5"}。每条必填 product_code、name、unit`,
					},
				},
				"required": []string{"items"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				rawItems := asArr(args, "items")
				if len(rawItems) == 0 {
					return nil, errorsBadRequest("items is required and must be non-empty array")
				}
				if len(rawItems) > 500 {
					return nil, errorsBadRequest(fmt.Sprintf("batch size exceeds 500: got %d", len(rawItems)))
				}
				list := make([]model.Product, 0, len(rawItems))
				for i, it := range rawItems {
					line, ok := it.(map[string]any)
					if !ok {
						return nil, errorsBadRequest(fmt.Sprintf("items[%d] is not an object", i))
					}
					m := model.Product{
						ProductCode: asStr(line, "product_code"),
						Name:        asStr(line, "name"),
						Spec:        asStr(line, "spec"),
						Unit:        asStr(line, "unit"),
						CostPrice:   asDecimal(line["cost_price"]),
						Status:      1,
					}
					if m.ProductCode == "" || m.Name == "" || m.Unit == "" {
						rawJSON, _ := json.Marshal(line)
						types := map[string]string{}
						for k, v := range line {
							types[k] = fmt.Sprintf("%T", v)
						}
						zap.L().Warn("[DBG product_batch_create coercion empty]",
							zap.Int("index", i),
							zap.String("product_code", m.ProductCode),
							zap.String("name", m.Name),
							zap.String("unit", m.Unit),
							zap.String("raw_line_json", string(rawJSON)),
							zap.Any("value_types", types),
						)
						return nil, errorsBadRequest(fmt.Sprintf(
							"items[%d] 缺少必需字段 product_code/name/unit（原始值：%s；各字段类型：%v），请补充后再试",
							i, string(rawJSON), types,
						))
					}
					list = append(list, m)
				}
				created, err := deps.Products.CreateBatch(context.Background(), actor.TenantID, list)
				if err != nil {
					return nil, err
				}
				ids := make([]uint, 0, len(created))
				codes := make([]string, 0, len(created))
				for _, m := range created {
					ids = append(ids, m.ID)
					codes = append(codes, m.ProductCode)
				}
				return map[string]any{
					"count":      len(created),
					"ids":        ids,
					"codes":      codes,
					"sample":     extractListSample(created[0]),
					"all_items":  created,
					"_tx_status": "atomic: all items committed or none",
				}, nil
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
				bom, err := deps.BOMs.Create(context.Background(), actor.TenantID, BOMInput{
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
			Description: "新建采购订单（采购下单）。必填 supplier_id（须 APPROVED）和 details 明细数组；details 每项：material_id、order_qty（数量）、unit_price（单价）。",
			Perm:        "po:create",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"po_number":         map[string]any{"type": "string", "description": "采购单号，留空自动生成"},
					"supplier_id":       map[string]any{"type": "integer", "description": "供应商 ID"},
					"order_date":        map[string]any{"type": "string", "description": "下单日期 YYYY-MM-DD（可选，默认今天）"},
					"expected_delivery": map[string]any{"type": "string", "description": "期望交期 YYYY-MM-DD（可选）"},
					"details":           map[string]any{"type": "array", "description": `明细数组，每项形如 {"material_id":1,"order_qty":"100","unit_price":"5.5"}`},
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
						LocationID: 0,
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
				po, err := deps.POs.Create(context.Background(), actor.TenantID, CreatePOInput{
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
		{
			Name:        "po_batch_create",
			Description: "【批量首选】一次性提交多条采购单，所有订单先整体预校验(字段/明细/供应商APPROVED/material_id存在性等)全部通过后，再逐条内部事务创建。严禁先调 po_list 翻页再一条条 po_create 反复刷。items 上限 500 条。",
			Perm:        "po:create",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type":        "array",
						"description": `采购单数组，每项同 po_create 单条 schema：{"po_number":"PO24...","supplier_id":123,"order_date":"2024-09-01","expected_delivery":"2024-09-20","details":[{"material_id":1,"order_qty":"100","unit_price":"5.5"}]}。每条必填 supplier_id、details（非空）；supplier 必须已 APPROVED；details 内每条必填 material_id、order_qty>0、unit_price>=0`,
					},
				},
				"required": []string{"items"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				rawItems := asArr(args, "items")
				if len(rawItems) == 0 {
					return nil, errorsBadRequest("items is required and must be non-empty array")
				}
				if len(rawItems) > 500 {
					return nil, errorsBadRequest(fmt.Sprintf("batch size exceeds 500: got %d", len(rawItems)))
				}
				inputs := make([]CreatePOInput, 0, len(rawItems))
				for i, it := range rawItems {
					line, ok := it.(map[string]any)
					if !ok {
						return nil, errorsBadRequest(fmt.Sprintf("items[%d] is not an object", i))
					}
					var details []PODetailInput
					for _, d := range asArr(line, "details") {
						row, ok := d.(map[string]any)
						if !ok {
							continue
						}
						details = append(details, PODetailInput{
							MaterialID: asUint(row["material_id"]),
							OrderQty:   asDecimal(row["order_qty"]),
							UnitPrice:  asDecimal(row["unit_price"]),
							LocationID: 0,
						})
					}
					orderDate := time.Now()
					if v := asStr(line, "order_date"); v != "" {
						if t, err := time.Parse("2006-01-02", v); err == nil {
							orderDate = t
						}
					}
					var expected *time.Time
					if v := asStr(line, "expected_delivery"); v != "" {
						if t, err := time.Parse("2006-01-02", v); err == nil {
							expected = &t
						}
					}
					inputs = append(inputs, CreatePOInput{
						PONumber:             asStr(line, "po_number"),
						SupplierID:           asUint(line["supplier_id"]),
						OrderDate:            orderDate,
						ExpectedDeliveryDate: expected,
						CreatedBy:            actor.Username,
						Details:              details,
					})
				}
				created, meta, err := deps.POs.CreateBatch(context.Background(), actor.TenantID, inputs)
				if err != nil {
					res := map[string]any{
						"error":         err.Error(),
						"success_ids":   make([]uint, 0),
						"success_count": meta.SuccessCount,
					}
					if meta.FailedIndex >= 0 {
						res["failed_index"] = meta.FailedIndex
						res["failed_error"] = meta.FailedErr
					}
					for _, p := range created {
						res["success_ids"] = append(res["success_ids"].([]uint), p.ID)
					}
					return res, err
				}
				ids := make([]uint, 0, len(created))
				poNums := make([]string, 0, len(created))
				for _, p := range created {
					ids = append(ids, p.ID)
					poNums = append(poNums, p.PONumber)
				}
				return map[string]any{
					"count":         len(created),
					"ids":           ids,
					"po_numbers":    poNums,
					"sample":        extractListSample(created[0]),
					"all_items":     created,
					"_tx_semantics": "pre-validate ALL rows atomically; then per-order transaction. No rows written if any row fails validation in Pass 1.",
					"success_count": meta.SuccessCount,
				}, nil
			},
		},
		{
			Name:        "customer_create",
			Description: "新建【单条】客户。仅当 1~2 条时使用；批量创建/批量导入请直接调用 customer_batch_create（单事务要么全成要么全败，省 token 防卡壳）。",
			Perm:        "customer:manage",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"customer_code":  map[string]any{"type": "string", "description": "客户编码（必填，唯一）"},
					"name":           map[string]any{"type": "string", "description": "客户名称（必填）"},
					"contact_person": map[string]any{"type": "string", "description": "联系人（可选）"},
					"phone":          map[string]any{"type": "string", "description": "手机号（查物流顺丰接口用，必须真实手机号，否则物流查询将无结果）"},
					"address":        map[string]any{"type": "string", "description": "地址（可选）"},
					"credit_limit":   map[string]any{"type": "string", "description": "信用额度 decimal（可选，默认 0）"},
					"audit_status":   map[string]any{"type": "string", "description": "准入状态 PENDING/APPROVED，默认 PENDING（销售下单前必须 APPROVED）"},
				},
				"required": []string{"customer_code", "name"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				m := &model.Customer{
					CustomerCode:  asStr(args, "customer_code"),
					Name:          asStr(args, "name"),
					ContactPerson: asStr(args, "contact_person"),
					Phone:         asStr(args, "phone"),
					CreditLimit:   asDecimal(args["credit_limit"]),
					AuditStatus:   asStr(args, "audit_status"),
					Status:        1,
				}
				if m.AuditStatus == "" {
					m.AuditStatus = model.AuditPending
				}
				if err := deps.Customers.Create(context.Background(), actor.TenantID, m); err != nil {
					return nil, err
				}
				return m, nil
			},
		},
		{
			Name:        "customer_batch_create",
			Description: "【批量首选】一次性提交多条客户，单事务保证要么全成要么全回滚。严禁先调 customer_list 翻页再一条条 customer_create 反复刷。items 上限 500 条。注意：phone 字段必须是真实手机号，否则后续销售订单物流查询将无结果。",
			Perm:        "customer:manage",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type":        "array",
						"description": `客户数组，每项形如 {"customer_code":"CUST-001","name":"北京XX科技","contact_person":"李总","phone":"13800001111","address":"北京市朝阳区...","credit_limit":"500000","audit_status":"APPROVED"}。每条必填 customer_code、name；phone 强烈建议填真实手机号以支持顺丰物流查询`,
					},
				},
				"required": []string{"items"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				rawItems := asArr(args, "items")
				if len(rawItems) == 0 {
					return nil, errorsBadRequest("items is required and must be non-empty array")
				}
				if len(rawItems) > 500 {
					return nil, errorsBadRequest(fmt.Sprintf("batch size exceeds 500: got %d", len(rawItems)))
				}
				list := make([]model.Customer, 0, len(rawItems))
				for i, it := range rawItems {
					line, ok := it.(map[string]any)
					if !ok {
						return nil, errorsBadRequest(fmt.Sprintf("items[%d] is not an object", i))
					}
					m := model.Customer{
						CustomerCode:  asStr(line, "customer_code"),
						Name:          asStr(line, "name"),
						ContactPerson: asStr(line, "contact_person"),
						Phone:         asStr(line, "phone"),
						CreditLimit:   asDecimal(line["credit_limit"]),
						AuditStatus:   asStr(line, "audit_status"),
						Status:        1,
					}
					if m.AuditStatus == "" {
						m.AuditStatus = model.AuditPending
					}
					if m.CustomerCode == "" || m.Name == "" {
						rawJSON, _ := json.Marshal(line)
						types := map[string]string{}
						for k, v := range line {
							types[k] = fmt.Sprintf("%T", v)
						}
						zap.L().Warn("[DBG customer_batch_create coercion empty]",
							zap.Int("index", i),
							zap.String("customer_code", m.CustomerCode),
							zap.String("name", m.Name),
							zap.String("raw_line_json", string(rawJSON)),
							zap.Any("value_types", types),
						)
						return nil, errorsBadRequest(fmt.Sprintf(
							"items[%d] 缺少必需字段 customer_code/name（原始值：%s；各字段类型：%v），请补充后再试",
							i, string(rawJSON), types,
						))
					}
					list = append(list, m)
				}
				created, err := deps.Customers.CreateBatch(context.Background(), actor.TenantID, list)
				if err != nil {
					return nil, err
				}
				ids := make([]uint, 0, len(created))
				codes := make([]string, 0, len(created))
				for _, m := range created {
					ids = append(ids, m.ID)
					codes = append(codes, m.CustomerCode)
				}
				return map[string]any{
					"count":      len(list),
					"ids":        ids,
					"codes":      codes,
					"sample":     extractListSample(list[0]),
					"all_items":  list,
					"_tx_status": "atomic: all items committed or none",
				}, nil
			},
		},
		{
			Name:        "so_create",
			Description: "新建销售订单（开单）。必填 customer_id（须 APPROVED）和 details 明细数组；details 每项：material_id、qty（数量）、unit_price（单价）。",
			Perm:        "so:create",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"so_number":   map[string]any{"type": "string", "description": "销售单号，留空自动生成"},
					"customer_id": map[string]any{"type": "integer", "description": "客户 ID"},
					"order_date":  map[string]any{"type": "string", "description": "下单日期 YYYY-MM-DD（可选，默认今天）"},
					"details":     map[string]any{"type": "array", "description": `明细数组，每项形如 {"material_id":1,"qty":"100","unit_price":"9.9"}`},
				},
				"required": []string{"customer_id", "details"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				var details []SODetailInput
				for _, it := range asArr(args, "details") {
					line, ok := it.(map[string]any)
					if !ok {
						continue
					}
					details = append(details, SODetailInput{
						MaterialID: asUint(line["material_id"]),
						Qty:        asDecimal(line["qty"]),
						UnitPrice:  asDecimal(line["unit_price"]),
					})
				}
				orderDate := time.Now()
				if v := asStr(args, "order_date"); v != "" {
					if t, err := time.Parse("2006-01-02", v); err == nil {
						orderDate = t
					}
				}
				so, err := deps.SOs.Create(context.Background(), actor.TenantID, CreateSOInput{
					SONumber:   asStr(args, "so_number"),
					CustomerID: asUint(args["customer_id"]),
					OrderDate:  orderDate,
					CreatedBy:  actor.Username,
					Details:    details,
				})
				if err != nil {
					return nil, err
				}
				return so, nil
			},
		},
		{
			Name:        "so_batch_create",
			Description: "【批量首选】一次性提交多条销售订单，所有订单先整体预校验（字段/明细/material_id存在/customer APPROVED 等）全部通过后，再逐条内部事务创建。严禁先调 so_list 翻页再一条条 so_create 反复刷。items 上限 500 条。",
			Perm:        "so:create",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type":        "array",
						"description": `销售单数组，每项同 so_create 单条 schema：{"so_number":"SO24...","customer_id":456,"order_date":"2024-09-01","details":[{"material_id":1,"qty":"50","unit_price":"12"}]}。每条必填 customer_id、details（非空）；customer 必须已 APPROVED；details 每条必填 material_id、qty>0、unit_price>=0`,
					},
				},
				"required": []string{"items"},
			},
			Exec: func(actor *authx.Actor, args map[string]any) (any, error) {
				rawItems := asArr(args, "items")
				if len(rawItems) == 0 {
					return nil, errorsBadRequest("items is required and must be non-empty array")
				}
				if len(rawItems) > 500 {
					return nil, errorsBadRequest(fmt.Sprintf("batch size exceeds 500: got %d", len(rawItems)))
				}
				inputs := make([]CreateSOInput, 0, len(rawItems))
				for i, it := range rawItems {
					line, ok := it.(map[string]any)
					if !ok {
						return nil, errorsBadRequest(fmt.Sprintf("items[%d] is not an object", i))
					}
					var details []SODetailInput
					for _, d := range asArr(line, "details") {
						row, ok := d.(map[string]any)
						if !ok {
							continue
						}
						details = append(details, SODetailInput{
							MaterialID: asUint(row["material_id"]),
							Qty:        asDecimal(row["qty"]),
							UnitPrice:  asDecimal(row["unit_price"]),
						})
					}
					orderDate := time.Now()
					if v := asStr(line, "order_date"); v != "" {
						if t, err := time.Parse("2006-01-02", v); err == nil {
							orderDate = t
						}
					}
					inputs = append(inputs, CreateSOInput{
						SONumber:   asStr(line, "so_number"),
						CustomerID: asUint(line["customer_id"]),
						OrderDate:  orderDate,
						CreatedBy:  actor.Username,
						Details:    details,
					})
				}
				created, meta, err := deps.SOs.CreateBatch(context.Background(), actor.TenantID, inputs)
				if err != nil {
					res := map[string]any{
						"error":         err.Error(),
						"success_ids":   make([]uint, 0),
						"success_count": meta.SuccessCount,
					}
					if meta.FailedIndex >= 0 {
						res["failed_index"] = meta.FailedIndex
						res["failed_error"] = meta.FailedErr
					}
					for _, s := range created {
						res["success_ids"] = append(res["success_ids"].([]uint), s.ID)
					}
					return res, err
				}
				ids := make([]uint, 0, len(created))
				soNums := make([]string, 0, len(created))
				for _, s := range created {
					ids = append(ids, s.ID)
					soNums = append(soNums, s.SONumber)
				}
				return map[string]any{
					"count":         len(created),
					"ids":           ids,
					"so_numbers":    soNums,
					"sample":        extractListSample(created[0]),
					"all_items":     created,
					"_tx_semantics": "pre-validate ALL rows atomically; then per-order transaction. No rows written if any row fails Pass 1 validation.",
					"success_count": meta.SuccessCount,
				}, nil
			},
		},
	}
	return tools
}

// getUploadDir mirrors the default from AssistantHandler so document tools
// can discover uploaded files without holding a handler pointer. Override via
// the SCM_UPLOAD_DIR env var.
func getUploadDir() string {
	if v := os.Getenv("SCM_UPLOAD_DIR"); v != "" {
		return v
	}
	return filepath.Join(".", "uploads", "assistant")
}

// resolveDocPath resolves either an absolute path (from document_list output)
// or a bare name (LLM asked by filename) into a readable file path. It
// refuses any ".." traversal so a hallucinated name can't break out.
func resolveDocPath(path, name string) (string, error) {
	if path != "" {
		clean := filepath.Clean(path)
		if strings.Contains(clean, "..") {
			return "", errors.New("refusing path with .. traversal")
		}
		if _, err := os.Stat(clean); err == nil {
			return clean, nil
		}
	}
	if name == "" {
		return "", errors.New("either path or name is required")
	}
	cleanName := filepath.Base(name)
	if cleanName == "." || cleanName == "/" || strings.Contains(cleanName, "..") {
		return "", errors.New("invalid file name: " + cleanName)
	}
	dir := getUploadDir()
	candidates, err := filepath.Glob(filepath.Join(dir, "*"+strings.TrimSuffix(filepath.Base(cleanName), filepath.Ext(cleanName))+"*"+filepath.Ext(cleanName)))
	if err == nil && len(candidates) > 0 {
		return candidates[0], nil
	}
	// Exact match by suffix: upload files are stored as YYYYMMDD_<uuid>.<ext>
	full := filepath.Join(dir, cleanName)
	if _, err := os.Stat(full); err == nil {
		return full, nil
	}
	if len(candidates) > 0 {
		return candidates[0], nil
	}
	return "", errors.New("file not found in upload dir: " + cleanName)
}

package service

import (
	"strconv"
	"strings"

	"scm/internal/model"
	"scm/internal/pkg/authx"
	"scm/internal/repository"
)

// AuditService writes operation-log records for successful mutations. The
// audit middleware calls Log on every successful POST/PUT/DELETE; the entry
// is derived from the HTTP route (module/action/resource) and the actor.
type AuditService struct {
	repo    *repository.OperationLogRepo
	tenants *repository.TenantRepo
}

func NewAuditService(repo *repository.OperationLogRepo, tenants *repository.TenantRepo) *AuditService {
	return &AuditService{repo: repo, tenants: tenants}
}

// Log records one operation. Best-effort: a failed log write never breaks the
// underlying request.
func (s *AuditService) Log(a *authx.Actor, method, path string) {
	if a == nil {
		return
	}
	module, action, resource, resourceID := describe(method, path)
	if action == "" {
		return // not a logged mutation (e.g. preview)
	}
	summary := action + " " + resource
	if resourceID != "" {
		summary += " #" + resourceID
	}
	_ = s.repo.Create(&model.OperationLog{
		TenantID:   a.TenantID,
		UserID:     a.UserID,
		Username:   a.Username,
		Roles:      strings.Join(a.Roles, ","),
		Module:     module,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Summary:    summary,
		Method:     method,
		Path:       path,
	})
}

// IsPlatform reports whether the tenant id is the platform tenant.
func (s *AuditService) IsPlatform(t uint) bool {
	tn, err := s.tenants.Get(t)
	return err == nil && tn != nil && tn.Code == "platform"
}

// Search lists audit logs. For a business tenant the scope is forced to that
// tenant; for the platform tenant it may span all tenants (via TenantID).
func (s *AuditService) Search(t uint, in PageInput, af repository.AuditFilter) ([]model.OperationLog, int64, error) {
	if !s.IsPlatform(t) {
		af.TenantID = t
	}
	var (
		out   []model.OperationLog
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword}
	if err := s.repo.Search(f, af, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// resourceInfo maps a first path segment to (module, resource).
type resourceInfo struct{ module, resource string }

var resourceMap = map[string]resourceInfo{
	"materials":         {"基础数据", "物料"},
	"suppliers":         {"基础数据", "供应商"},
	"customers":         {"基础数据", "客户"},
	"warehouses":        {"基础数据", "仓库"},
	"locations":         {"基础数据", "库位"},
	"supplier-material": {"基础数据", "供应关系"},
	"po":                {"采购", "采购订单"},
	"bom-order":         {"采购", "采购订单"},
	"so":                {"销售", "销售订单"},
	"inventory":         {"仓储", "出入库"},
	"planning":          {"计划", "计划"},
	"products":          {"研发", "产品"},
	"boms":              {"研发", "BOM"},
	"users":             {"系统", "用户"},
	"tenants":           {"系统", "租户"},
	"rbac":              {"系统", "角色权限"},
}

// describe derives (module, action, resource, resourceID) from a route. Empty
// action means the request is not logged.
func describe(method, path string) (module, action, resource, resourceID string) {
	p := strings.TrimPrefix(path, "/api/v1")
	segs := strings.Split(strings.Trim(p, "/"), "/")
	if len(segs) == 0 || segs[0] == "" {
		return "", "", "", ""
	}
	info, ok := resourceMap[segs[0]]
	if !ok {
		return "", "", "", ""
	}
	module, resource = info.module, info.resource

	// resource-specific refinement.
	switch segs[0] {
	case "inventory":
		if len(segs) > 1 && segs[1] == "stock" {
			resource = "库存"
		}
	case "planning":
		if len(segs) > 1 && segs[1] == "demands" {
			resource = "需求"
		} else if len(segs) > 1 && segs[1] == "mrp" {
			resource = "MRP"
		}
	}

	// resource id = first numeric segment.
	for _, s := range segs {
		if _, err := strconv.Atoi(s); err == nil {
			resourceID = s
			break
		}
	}

	last := segs[len(segs)-1]

	// skip read-ish posts (preview only computes, doesn't mutate).
	if last == "preview" {
		return "", "", "", ""
	}

	switch method {
	case "POST":
		switch last {
		case "confirm":
			action = "拆单下单"
		case "compute":
			action = "运算"
		default:
			action = "创建"
		}
	case "DELETE":
		action = "删除"
	case "PUT":
		switch last {
		case "status":
			action = "状态变更"
		case "approve":
			action = "审批"
		case "cancel":
			action = "取消"
		case "receive":
			action = "收货"
		case "release":
			action = "发布"
		case "audit":
			action = "审核"
		case "permissions":
			action = "权限配置"
		default:
			action = "更新"
		}
	}
	return module, action, resource, resourceID
}

package service

import (
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"scm/internal/model"
)
func SeedRBAC(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.Tenant{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// demo first so it takes id 1 (the SCM_AUTH=off dev fallback tenant).
	demo := model.Tenant{Code: "demo", Name: "演示租户", Plan: "PRO", Status: model.TenantActive}
	platform := model.Tenant{Code: "platform", Name: "平台", Plan: "PLATFORM", Status: model.TenantActive}
	if err := db.Create(&demo).Error; err != nil {
		return err
	}
	if err := db.Create(&platform).Error; err != nil {
		return err
	}

	roles := []model.Role{
		{Code: model.RoleAdmin, Name: "管理员", Remark: "全部权限"},
		{Code: model.RoleProcSpec, Name: "采购专员", Remark: "寻源/下单执行"},
		{Code: model.RoleProcMgr, Name: "采购经理", Remark: "采购审批/供应商准入"},
		{Code: model.RolePlanSpec, Name: "计划专员", Remark: "需求/计划维护"},
		{Code: model.RolePlanSup, Name: "计划主管", Remark: "计划审批/MRP/BOM发布"},
		{Code: model.RoleQC, Name: "质检员/品控", Remark: "收货质检"},
		{Code: model.RoleWhMgr, Name: "仓库管理员", Remark: "仓储作业/库存管理"},
	}
	if err := db.Create(&roles).Error; err != nil {
		return err
	}

	modules := []model.Module{
		{Code: "base", Name: "基础数据", Sort: 1},
		{Code: "purchase", Name: "采购", Sort: 2},
		{Code: "sales", Name: "销售", Sort: 3},
		{Code: "inventory", Name: "仓储", Sort: 4},
		{Code: "planning", Name: "计划", Sort: 5},
		{Code: "system", Name: "系统", Sort: 6},
	}
	if err := db.Create(&modules).Error; err != nil {
		return err
	}
	modID := map[string]uint{}
	for _, m := range modules {
		modID[m.Code] = m.ID
	}

	type permDef struct{ code, name, module string }
	permDefs := []permDef{
		{"material:view", "物料查看", "base"}, {"material:manage", "物料维护", "base"},
		{"supplier:view", "供应商查看", "base"}, {"supplier:manage", "供应商维护", "base"},
		{"supplier:audit", "供应商准入审核", "base"},
		{"customer:view", "客户查看", "base"}, {"customer:manage", "客户维护", "base"},
		{"warehouse:view", "仓库/库位查看", "base"}, {"warehouse:manage", "仓库/库位维护", "base"},
		{"po:view", "采购订单查看", "purchase"}, {"po:create", "采购下单", "purchase"},
		{"po:edit", "采购订单编辑", "purchase"}, {"po:approve", "采购审批", "purchase"},
		{"po:receive", "采购收货", "purchase"}, {"po:delete", "采购订单删除", "purchase"},
		{"so:view", "销售订单查看", "sales"}, {"so:create", "销售下单", "sales"},
		{"so:approve", "销售审批(锁库/信用)", "sales"}, {"so:cancel", "销售取消", "sales"},
		{"so:delete", "销售订单删除", "sales"},
		{"stock:view", "库存查看", "inventory"}, {"inv:move", "出入库作业", "inventory"},
		{"inv:logs:view", "库存流水查看", "inventory"}, {"inv:order:delete", "库存单据删除", "inventory"},
		{"demand:view", "需求查看", "planning"}, {"demand:manage", "需求维护", "planning"},
		{"mrp:view", "MRP 查看", "planning"}, {"mrp:compute", "MRP 运算", "planning"},
		{"mrp:convert", "MRP 转采购", "planning"},
		{"user:manage", "用户管理", "system"}, {"tenant:manage", "租户管理", "system"},
		{"perms:view", "权限目录查看", "system"},
	}
	var perms []model.Permission
	for _, p := range permDefs {
		perms = append(perms, model.Permission{Code: p.code, Name: p.name, ModuleID: modID[p.module]})
	}
	if err := db.Create(&perms).Error; err != nil {
		return err
	}
	permID := map[string]uint{}
	allCodes := make([]string, 0, len(perms))
	for _, p := range perms {
		permID[p.Code] = p.ID
		allCodes = append(allCodes, p.Code)
	}

	matrix := map[string][]string{
		model.RoleAdmin: allCodes,
		model.RoleProcSpec: {
			"material:view", "supplier:view", "customer:view", "warehouse:view",
			"po:view", "po:create", "po:edit",
			"demand:view", "mrp:view", "stock:view", "inv:logs:view",
		},
		model.RoleProcMgr: {
			"material:view", "material:manage", "supplier:view", "supplier:manage", "supplier:audit",
			"customer:view", "warehouse:view",
			"po:view", "po:create", "po:edit", "po:approve", "po:delete",
			"demand:view", "demand:manage", "mrp:view", "stock:view", "inv:logs:view",
		},
		model.RolePlanSpec: {
			"material:view", "supplier:view", "warehouse:view",
			"po:view", "demand:view", "demand:manage", "mrp:view", "mrp:compute",
			"stock:view",
		},
		model.RolePlanSup: {
			"material:view", "material:manage", "supplier:view", "warehouse:view",
			"po:view", "demand:view", "demand:manage", "mrp:view", "mrp:compute", "mrp:convert",
			"stock:view", "inv:logs:view",
		},
		model.RoleQC: {
			"material:view", "warehouse:view",
			"po:view", "po:receive", "stock:view", "inv:move", "inv:logs:view",
		},
		model.RoleWhMgr: {
			"material:view", "warehouse:view", "warehouse:manage",
			"po:view", "po:receive", "stock:view", "inv:move", "inv:logs:view", "inv:order:delete",
		},
	}
	roleID := map[string]uint{}
	for _, r := range roles {
		roleID[r.Code] = r.ID
	}
	for rc, codes := range matrix {
		for _, pc := range codes {
			if err := db.Create(&model.RolePermission{RoleID: roleID[rc], PermissionID: permID[pc]}).Error; err != nil {
				return err
			}
		}
	}

	// initial admins: platform + demo tenant (admin / admin123).
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	adminP := model.User{TenantID: platform.ID, Username: "admin", PasswordHash: string(hash), Name: "平台管理员"}
	adminD := model.User{TenantID: demo.ID, Username: "admin", PasswordHash: string(hash), Name: "演示管理员"}
	if err := db.Create(&adminP).Error; err != nil {
		return err
	}
	if err := db.Create(&adminD).Error; err != nil {
		return err
	}
	for _, u := range []model.User{adminP, adminD} {
		if err := db.Create(&model.UserRole{UserID: u.ID, RoleID: roleID[model.RoleAdmin]}).Error; err != nil {
			return err
		}
	}

	// adopt pre-tenancy dev data into the demo tenant.
	for _, t := range []string{
		"sys_material", "sys_supplier", "base_customer", "sys_warehouse", "sys_location",
		"pur_order", "pur_receipt", "inv_stock", "inv_order", "inv_transaction_log",
		"plan_demand", "plan_mrp_result", "sale_order",
	} {
		if err := db.Exec("UPDATE "+t+" SET tenant_id = ? WHERE tenant_id = 0", demo.ID).Error; err != nil {
			log.Printf("seed: adopt %s: %v", t, err)
		}
	}
	return nil
}

// EnsureRNDCatalog idempotently adds the R&D module + BOM permissions to an
// existing deployment (SeedRBAC skips everything once tenants exist, so later
// catalog additions need their own find-or-create step).
func EnsureRNDCatalog(db *gorm.DB) error {
	var mod model.Module
	if err := db.Where("code = ?", "rnd").First(&mod).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		mod = model.Module{Code: "rnd", Name: "研发", Sort: 0}
		if err := db.Create(&mod).Error; err != nil {
			return err
		}
	}

	permDefs := []struct{ code, name string }{
		{"bom:view", "BOM 查看"},
		{"bom:edit", "BOM 编辑"},
		{"bom:release", "BOM 发布"},
	}
	permID := map[string]uint{}
	for _, p := range permDefs {
		var existing model.Permission
		if err := db.Where("code = ?", p.code).First(&existing).Error; err == nil {
			permID[p.code] = existing.ID
			continue
		}
		np := model.Permission{Code: p.code, Name: p.name, ModuleID: mod.ID}
		if err := db.Create(&np).Error; err != nil {
			return err
		}
		permID[p.code] = np.ID
	}

	// role -> permission matrix additions (RACI-aligned).
	matrix := map[string][]string{
		model.RoleAdmin:    {"bom:view", "bom:edit", "bom:release"},
		model.RoleProcMgr:  {"bom:view"},
		model.RoleProcSpec: {"bom:view"},
		model.RolePlanSpec: {"bom:view", "bom:edit"},
		model.RolePlanSup:  {"bom:view", "bom:edit", "bom:release"},
		model.RoleQC:       {"bom:view"},
		model.RoleWhMgr:    {"bom:view"},
	}
	var roles []model.Role
	if err := db.Find(&roles).Error; err != nil {
		return err
	}
	roleID := map[string]uint{}
	for _, r := range roles {
		roleID[r.Code] = r.ID
	}
	for rc, codes := range matrix {
		for _, pc := range codes {
			var cnt int64
			db.Model(&model.RolePermission{}).
				Where("role_id = ? AND permission_id = ?", roleID[rc], permID[pc]).Count(&cnt)
			if cnt == 0 {
				if err := db.Create(&model.RolePermission{RoleID: roleID[rc], PermissionID: permID[pc]}).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// EnsureAuditCatalog idempotently adds the audit:view permission (system
// module) and grants it to admin + manager/supervisor roles.
func EnsureAuditCatalog(db *gorm.DB) error {
	var mod model.Module
	if err := db.Where("code = ?", "system").First(&mod).Error; err != nil {
		return err
	}
	var perm model.Permission
	if err := db.Where("code = ?", "audit:view").First(&perm).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		perm = model.Permission{Code: "audit:view", Name: "操作日志查看", ModuleID: mod.ID}
		if err := db.Create(&perm).Error; err != nil {
			return err
		}
	}
	grantTo := []string{
		model.RoleAdmin, model.RoleProcMgr, model.RolePlanSup, model.RoleWhMgr,
	}
	var roles []model.Role
	if err := db.Find(&roles).Error; err != nil {
		return err
	}
	roleID := map[string]uint{}
	for _, r := range roles {
		roleID[r.Code] = r.ID
	}
	for _, rc := range grantTo {
		var cnt int64
		db.Model(&model.RolePermission{}).
			Where("role_id = ? AND permission_id = ?", roleID[rc], perm.ID).Count(&cnt)
		if cnt == 0 && roleID[rc] != 0 {
			if err := db.Create(&model.RolePermission{RoleID: roleID[rc], PermissionID: perm.ID}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// MigrateRoles replaces the legacy six-role set with the current seven-role
// design on an EXISTING database: wipes roles/matrix/assignments, re-seeds the
// new roles and matrix, and remaps users from old roles to the closest new
// role. Idempotent: skips when no legacy roles remain.
func MigrateRoles(db *gorm.DB) error {
	legacy := []string{"category_manager", "procurement_assistant", "committee", "qc_wh", "finance"}
	var n int64
	db.Model(&model.Role{}).Where("code IN ?", legacy).Count(&n)
	if n == 0 {
		return nil // nothing to migrate (fresh or already migrated)
	}

	// capture current assignments for remapping.
	var urs []model.UserRole
	if err := db.Find(&urs).Error; err != nil {
		return err
	}
	var oldRoles []model.Role
	if err := db.Find(&oldRoles).Error; err != nil {
		return err
	}
	oldCode := map[uint]string{}
	for _, r := range oldRoles {
		oldCode[r.ID] = r.Code
	}

	// wipe roles, matrix and assignments.
	if err := db.Exec("DELETE FROM sys_user_role").Error; err != nil {
		return err
	}
	if err := db.Exec("DELETE FROM sys_role_permission").Error; err != nil {
		return err
	}
	if err := db.Exec("DELETE FROM sys_role").Error; err != nil {
		return err
	}

	// re-seed the new role catalog + matrix (base permissions; bom perms are
	// added by EnsureRNDCatalog).
	roles := []model.Role{
		{Code: model.RoleAdmin, Name: "管理员", Remark: "全部权限"},
		{Code: model.RoleProcSpec, Name: "采购专员", Remark: "寻源/下单执行"},
		{Code: model.RoleProcMgr, Name: "采购经理", Remark: "采购审批/供应商准入"},
		{Code: model.RolePlanSpec, Name: "计划专员", Remark: "需求/计划维护"},
		{Code: model.RolePlanSup, Name: "计划主管", Remark: "计划审批/MRP/BOM发布"},
		{Code: model.RoleQC, Name: "质检员/品控", Remark: "收货质检"},
		{Code: model.RoleWhMgr, Name: "仓库管理员", Remark: "仓储作业/库存管理"},
	}
	if err := db.Create(&roles).Error; err != nil {
		return err
	}
	roleID := map[string]uint{}
	for _, r := range roles {
		roleID[r.Code] = r.ID
	}
	var perms []model.Permission
	if err := db.Find(&perms).Error; err != nil {
		return err
	}
	permID := map[string]uint{}
	for _, p := range perms {
		permID[p.Code] = p.ID
	}
	matrix := map[string][]string{
		model.RoleAdmin: {},
		model.RoleProcSpec: {
			"material:view", "supplier:view", "customer:view", "warehouse:view",
			"po:view", "po:create", "po:edit", "demand:view", "mrp:view", "stock:view", "inv:logs:view",
		},
		model.RoleProcMgr: {
			"material:view", "material:manage", "supplier:view", "supplier:manage", "supplier:audit",
			"customer:view", "warehouse:view", "po:view", "po:create", "po:edit", "po:approve", "po:delete",
			"demand:view", "demand:manage", "mrp:view", "stock:view", "inv:logs:view",
		},
		model.RolePlanSpec: {
			"material:view", "supplier:view", "warehouse:view",
			"po:view", "demand:view", "demand:manage", "mrp:view", "mrp:compute", "stock:view",
		},
		model.RolePlanSup: {
			"material:view", "material:manage", "supplier:view", "warehouse:view",
			"po:view", "demand:view", "demand:manage", "mrp:view", "mrp:compute", "mrp:convert",
			"stock:view", "inv:logs:view",
		},
		model.RoleQC: {
			"material:view", "warehouse:view",
			"po:view", "po:receive", "stock:view", "inv:move", "inv:logs:view",
		},
		model.RoleWhMgr: {
			"material:view", "warehouse:view", "warehouse:manage",
			"po:view", "po:receive", "stock:view", "inv:move", "inv:logs:view", "inv:order:delete",
		},
	}
	// admin gets every currently-known permission.
	for c := range permID {
		matrix[model.RoleAdmin] = append(matrix[model.RoleAdmin], c)
	}
	for rc, codes := range matrix {
		for _, pc := range codes {
			if pid, ok := permID[pc]; ok && pid != 0 {
				if err := db.Create(&model.RolePermission{RoleID: roleID[rc], PermissionID: pid}).Error; err != nil {
					return err
				}
			}
		}
	}

	// remap users: old role -> closest new role (finance has no successor).
	mapping := map[string]string{
		"admin":                model.RoleAdmin,
		"category_manager":     model.RoleProcMgr,
		"procurement_assistant": model.RoleProcSpec,
		"committee":            model.RoleProcMgr,
		"qc_wh":                model.RoleQC,
	}
	seen := map[[2]uint]bool{}
	for _, ur := range urs {
		oc := oldCode[ur.RoleID]
		nc, ok := mapping[oc]
		if !ok {
			continue
		}
		rid := roleID[nc]
		if rid == 0 {
			continue
		}
		key := [2]uint{ur.UserID, rid}
		if seen[key] {
			continue
		}
		seen[key] = true
		if err := db.Create(&model.UserRole{UserID: ur.UserID, RoleID: rid}).Error; err != nil {
			return err
		}
	}
	return nil
}

package service

import (
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"scm/internal/model"
)

// SeedRBAC bootstraps tenants, the six roles, module/permission catalogs, the
// RACI permission matrix and initial admin users. Idempotent: it only runs
// while the tenant table is empty. Existing pre-tenant business rows are
// adopted by the demo tenant.
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
		{Code: model.RoleAdmin, Name: "管理员", Remark: "全部权限 + 用户管理"},
		{Code: model.RoleCategoryMgr, Name: "品类经理", Remark: "主数据/审批决策"},
		{Code: model.RoleProcAssistant, Name: "采购助理", Remark: "寻源/下单执行"},
		{Code: model.RoleCommittee, Name: "采购决策委员会", Remark: "定标/准入批准"},
		{Code: model.RoleQCWH, Name: "仓库/质检", Remark: "收货/库存作业"},
		{Code: model.RoleFinance, Name: "财务", Remark: "信用/对账监督"},
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
		model.RoleCategoryMgr: {
			"material:view", "material:manage", "supplier:view", "supplier:audit",
			"customer:view", "customer:manage", "warehouse:view",
			"po:view", "po:approve", "so:view", "so:approve",
			"stock:view", "inv:logs:view",
			"demand:view", "demand:manage", "mrp:view", "mrp:compute", "mrp:convert",
		},
		model.RoleProcAssistant: {
			"material:view", "supplier:view", "warehouse:view",
			"po:view", "po:create", "po:edit",
			"demand:view", "demand:manage", "mrp:view",
		},
		model.RoleCommittee: {"po:view", "po:approve", "supplier:view", "supplier:audit", "so:view"},
		model.RoleQCWH: {
			"po:view", "po:receive", "warehouse:view",
			"stock:view", "inv:move", "inv:logs:view",
		},
		model.RoleFinance: {"so:view", "so:approve", "customer:view", "po:view", "stock:view", "inv:logs:view"},
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

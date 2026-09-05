// Command server is the application entrypoint. It wires configuration ->
// database -> repositories -> services -> handlers -> router, then starts the
// HTTP server. Keeping wiring here (and not inside the layers) preserves the
// single-responsibility boundary between each package.
package main

import (
	"go.uber.org/zap"

	"scm/internal/config"
	"scm/internal/database"
	"scm/internal/handler"
	"scm/internal/memory"
	"scm/internal/middleware"
	"scm/pkg/llmclient"
	"scm/internal/repo"
	"scm/internal/router"
	"scm/internal/service"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	cfg := config.Load()

	// 1) Database + schema migration (dialect-agnostic via GORM).
	db, err := database.Open(cfg.DB.Driver, cfg.DB.DSN)
	if err != nil {
		zap.L().Fatal("db open", zap.Error(err))
	}

	// 2) Repositories (data access).
	gdb := repository.NewGormDB(db.DB)
	materialRepo := repository.NewMaterialRepo(gdb)
	supplierRepo := repository.NewSupplierRepo(gdb)
	warehouseRepo := repository.NewWarehouseRepo(gdb)
	locationRepo := repository.NewLocationRepo(gdb)
	poRepo := repository.NewPurchaseOrderRepo(gdb)
	receiptRepo := repository.NewPurchaseReceiptRepo(gdb)
	stockRepo := repository.NewStockRepo(gdb)
	invOrderRepo := repository.NewInventoryOrderRepo(gdb)
	txLogRepo := repository.NewTransactionLogRepo(gdb)
	demandRepo := repository.NewDemandRepo(gdb)
	mrpRepo := repository.NewMrpResultRepo(gdb)
	customerRepo := repository.NewCustomerRepo(gdb)
	soRepo := repository.NewSaleOrderRepo(gdb)
	userRepo := repository.NewUserRepo(gdb)
	tenantRepo := repository.NewTenantRepo(gdb)
	roleRepo := repository.NewRoleRepo(gdb)
	moduleRepo := repository.NewModuleRepo(gdb)
	permRepo := repository.NewPermissionRepo(gdb)
	userRoleRepo := repository.NewUserRoleRepo(gdb)
	productRepo := repository.NewProductRepo(gdb)
	bomRepo := repository.NewBOMRepo(gdb)
	supplierMaterialRepo := repository.NewSupplierMaterialRepo(gdb)
	operationLogRepo := repository.NewOperationLogRepo(gdb)
	dataSourceRepo := repository.NewDataSourceRepo(gdb)
	approvalGroupRepo := repository.NewApprovalGroupRepo(gdb)
	approvalTaskRepo := repository.NewApprovalTaskRepo(gdb)
	apiKeyRepo := repository.NewApiKeyRepo(gdb)
	memRepo := repository.NewAssistantMemoryRepo(gdb)
	memNodeRepo := repository.NewMemoryNodeRepo(gdb)
	memEdgeRepo := repository.NewMemoryEdgeRepo(gdb)
	memProfRepo := repository.NewMemoryProfileRepo(gdb)

	// 3) Services (business logic).
	if err := service.SeedRBAC(db.DB); err != nil {
		zap.L().Fatal("seed rbac", zap.Error(err))
	}
	if err := service.MigrateRoles(db.DB); err != nil {
		zap.L().Fatal("migrate roles", zap.Error(err))
	}
	if err := service.EnsureRNDCatalog(db.DB); err != nil {
		zap.L().Fatal("seed rnd catalog", zap.Error(err))
	}
	if err := service.EnsureAuditCatalog(db.DB); err != nil {
		zap.L().Fatal("seed audit catalog", zap.Error(err))
	}
	if err := service.EnsureApprovalCatalog(db.DB); err != nil {
		zap.L().Fatal("seed approval catalog", zap.Error(err))
	}
	rbacSvc := service.NewRBACService(service.RBACDeps{
		Tenants: tenantRepo, Users: userRepo, Roles: roleRepo,
		Modules: moduleRepo, Perms: permRepo, UserRoles: userRoleRepo, DB: db.DB,
	})
	if err := rbacSvc.RefreshCache(); err != nil {
		zap.L().Fatal("refresh permission cache", zap.Error(err))
	}
	authSvc := service.NewAuthService(userRepo, tenantRepo, userRoleRepo, cfg.Auth.JWTSecret, cfg.Auth.TokenTTL)
	materialSvc := service.NewMaterialService(materialRepo)
	supplierSvc := service.NewSupplierService(supplierRepo)
	warehouseSvc := service.NewWarehouseService(warehouseRepo)
	locationSvc := service.NewLocationService(locationRepo)
	poSvc := service.NewPurchaseOrderService(poRepo, supplierRepo)
	invSvc := service.NewInventoryService(service.InventoryDeps{
		Stock:  stockRepo,
		Orders: invOrderRepo,
		Logs:   txLogRepo,
		DB:     db.DB,
	})
	receivingSvc := service.NewReceivingService(service.ReceivingDeps{
		Receipts: receiptRepo,
		POs:      poRepo,
		Inv:      invSvc,
		DB:       db.DB,
	})
	customerSvc := service.NewCustomerService(customerRepo)
	soSvc := service.NewSalesOrderService(service.SalesOrderDeps{
		Repo:      soRepo,
		Customers: customerRepo,
		Stock:     stockRepo,
		DB:        db.DB,
	})
	stockSvc := service.NewStockService(stockRepo)
	productSvc := service.NewProductService(productRepo)
	bomSvc := service.NewBOMService(service.BOMDeps{
		Repo:      bomRepo,
		Products:  productRepo,
		Materials: materialRepo,
		DB:        db.DB,
	})
	supplierMaterialSvc := service.NewSupplierMaterialService(supplierMaterialRepo, supplierRepo, materialRepo)
	auditSvc := service.NewAuditService(operationLogRepo, tenantRepo)
	storageSvc := service.NewStorageService(db.DB, dataSourceRepo, cfg.DB.Driver, cfg.DB.DSN)
	approvalSvc := service.NewApprovalService(service.ApprovalDeps{
		Groups: approvalGroupRepo,
		Tasks:  approvalTaskRepo,
		Users:  userRepo,
		POSvc:  poSvc,
		SOSvc:  soSvc,
		DB:     db.DB,
	})
	bomOrderSvc := service.NewBOMOrderService(service.BOMOrderDeps{
		BOM:       bomRepo,
		SupMat:    supplierMaterialRepo,
		Suppliers: supplierRepo,
		Materials: materialRepo,
		POSvc:     poSvc,
		DB:        db.DB,
	})
	planningSvc := service.NewPlanningService(service.PlanningDeps{
		Demand:   demandRepo,
		Mrp:      mrpRepo,
		Stock:    stockRepo,
		POSvc:    poSvc,
		Supplier: supplierRepo,
		DB:       db.DB,
	})
	apiKeySvc := service.NewApiKeyService(apiKeyRepo, cfg.Auth.JWTSecret)
	memorySvc := memory.NewService(memRepo, memNodeRepo, memEdgeRepo, memProfRepo,
		llmclient.New(cfg.LLM.URL, cfg.LLM.Model, cfg.LLM.Key),
		memory.Config{WindowSize: 10, ConsolidateInterval: 5})
	assistantSvc := service.NewAssistantService(
		llmclient.New(cfg.LLM.URL, cfg.LLM.Model, cfg.LLM.Key),
		service.AssistantDeps{
			Materials: materialSvc,
			Suppliers: supplierSvc,
			Products:  productSvc,
			BOMs:      bomSvc,
			POs:       poSvc,
			Stock:     stockSvc,
			RBAC:      rbacSvc,
			Audit:     auditSvc,
			Memory:    memorySvc,
		},
	)

	// 4) Handlers (HTTP translation).
	h := &router.Handlers{
		RBAC:      handler.NewRBACHandler(rbacSvc, authSvc),
		Base:      handler.NewBaseDataHandler(materialSvc, supplierSvc, warehouseSvc, locationSvc),
		PO:        handler.NewPurchaseOrderHandler(poSvc),
		Receiving: handler.NewReceivingHandler(receivingSvc),
		Sales:     handler.NewSalesHandler(customerSvc, soSvc),
		Inventory: handler.NewInventoryHandler(invSvc),
		Stock:     handler.NewStockHandler(stockSvc),
		Planning:  handler.NewPlanningHandler(planningSvc),
		RND:       handler.NewRNDHandler(productSvc, bomSvc),
		SupMat:    handler.NewSupplierMaterialHandler(supplierMaterialSvc),
		BOMOrder:  handler.NewBOMOrderHandler(bomOrderSvc),
		AuditLog:  handler.NewOperationLogHandler(auditSvc),
		Storage:   handler.NewStorageHandler(storageSvc, rbacSvc.IsPlatform),
		Approval:  handler.NewApprovalHandler(approvalSvc),
		ApiKey:    handler.NewApiKeyHandler(apiKeySvc, rbacSvc),
		Assistant: handler.NewAssistantHandler(assistantSvc, memorySvc),
	}

	// 5) Router + start.
	authMW := middleware.AuthOrKey(cfg.Auth.Enabled, authSvc.ParseToken, apiKeySvc.Verify)
	permMW := middleware.RequirePerm(cfg.Auth.Enabled, rbacSvc.Check)
	auditMW := middleware.Audit(cfg.Auth.Enabled, auditSvc)
	engine := router.New(cfg.Server.CORSOrigin, h, authMW, permMW, auditMW)
	if !cfg.Auth.Enabled {
		zap.L().Warn("SCM_AUTH=off — authentication disabled (dev only)")
	}
	if cfg.Auth.JWTSecret == "scm-dev-secret-change-me" {
		zap.L().Warn("default JWT secret in use — set SCM_JWT_SECRET in production")
	}
	zap.L().Info("SCM server listening",
		zap.String("addr", cfg.Server.Addr),
		zap.String("driver", cfg.DB.Driver),
		zap.Bool("auth", cfg.Auth.Enabled),
	)
	if err := engine.Run(cfg.Server.Addr); err != nil {
		zap.L().Fatal("server run", zap.Error(err))
	}
}
// Command server is the application entrypoint. It wires configuration ->
// database -> repositories -> services -> handlers -> router, then starts the
// HTTP server. Keeping wiring here (and not inside the layers) preserves the
// single-responsibility boundary between each package.
package main

import (
	"log"

	"scm/internal/config"
	"scm/internal/database"
	"scm/internal/handler"
	"scm/internal/repository"
	"scm/internal/router"
	"scm/internal/service"
)

func main() {
	cfg := config.Load()

	// 1) Database + schema migration (dialect-agnostic via GORM).
	db, err := database.Open(cfg.DB.Driver, cfg.DB.DSN)
	if err != nil {
		log.Fatalf("db open: %v", err)
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

	// 3) Services (business logic).
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
	planningSvc := service.NewPlanningService(service.PlanningDeps{
		Demand:   demandRepo,
		Mrp:      mrpRepo,
		Stock:    stockRepo,
		POSvc:    poSvc,
		Supplier: supplierRepo,
		DB:       db.DB,
	})

	// 4) Handlers (HTTP translation).
	h := &router.Handlers{
		Base:      handler.NewBaseDataHandler(materialSvc, supplierSvc, warehouseSvc, locationSvc),
		PO:        handler.NewPurchaseOrderHandler(poSvc),
		Receiving: handler.NewReceivingHandler(receivingSvc),
		Sales:     handler.NewSalesHandler(customerSvc, soSvc),
		Inventory: handler.NewInventoryHandler(invSvc),
		Stock:     handler.NewStockHandler(stockSvc),
		Planning:  handler.NewPlanningHandler(planningSvc),
	}

	// 5) Router + start.
	engine := router.New(cfg.Server.CORSOrigin, h)
	log.Printf("SCM server listening on %s (driver=%s)", cfg.Server.Addr, cfg.DB.Driver)
	if err := engine.Run(cfg.Server.Addr); err != nil {
		log.Fatalf("server run: %v", err)
	}
}

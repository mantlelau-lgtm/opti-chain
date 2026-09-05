// Package router wires HTTP routes to handlers. It depends on the handler
// layer only, keeping the transport wiring isolated and testable.
package router

import (
	"github.com/gin-gonic/gin"

	"scm/internal/handler"
	"scm/internal/middleware"
)

// Handlers groups every handler the router needs.
type Handlers struct {
	RBAC      *handler.RBACHandler
	Base      *handler.BaseDataHandler
	PO        *handler.PurchaseOrderHandler
	Receiving *handler.ReceivingHandler
	Sales     *handler.SalesHandler
	Inventory *handler.InventoryHandler
	Stock     *handler.StockHandler
	Planning  *handler.PlanningHandler
	RND       *handler.RNDHandler
	SupMat    *handler.SupplierMaterialHandler
	BOMOrder  *handler.BOMOrderHandler
	AuditLog  *handler.OperationLogHandler
	Storage   *handler.StorageHandler
	Approval  *handler.ApprovalHandler
	ApiKey    *handler.ApiKeyHandler
	Assistant *handler.AssistantHandler
}

// New builds a configured gin engine with all routes registered. authMW
// verifies the bearer token and permMW enforces the DB-catalogued permission;
// both protect every /api/v1 route except the public login endpoint.
func New(corsOrigin string, h *Handlers, authMW, permMW, auditMW gin.HandlerFunc) *gin.Engine {
	g := gin.New()
	g.Use(middleware.Recovery())
	g.Use(gin.Logger())
	g.Use(middleware.CORS(corsOrigin))

	api := g.Group("/api/v1")
	{
		api.POST("/auth/login", h.RBAC.Login)
	}
	protected := g.Group("/api/v1")
	protected.Use(authMW, permMW, auditMW)
	{
		protected.GET("/auth/me", h.RBAC.Me)
		protected.PUT("/auth/password", h.RBAC.ChangePassword)
		protected.GET("/rbac/catalog", h.RBAC.Catalog)
		protected.PUT("/rbac/roles/:id/permissions", h.RBAC.RoleSetPermissions)
		protected.GET("/operation-logs", h.AuditLog.List)
		protected.POST("/assistant/chat", h.Assistant.Chat)
		protected.GET("/assistant/memory", h.Assistant.GetHistory)
		protected.DELETE("/assistant/memory", h.Assistant.ClearMemory)
		registerStorage(protected, h.Storage)
		registerApproval(protected, h.Approval)
		registerApiKey(protected, h.ApiKey)
		registerBase(protected, h.Base)
		registerProcurement(protected, h.PO, h.Receiving)
		registerSales(protected, h.Sales)
		registerInventory(protected, h.Inventory, h.Stock)
		registerPlanning(protected, h.Planning)
		registerRND(protected, h.RND)
		registerSupplierMaterial(protected, h.SupMat)
		registerBOMOrder(protected, h.BOMOrder)
		registerRBAC(protected, h.RBAC)
	}
	return g
}

func registerRND(g *gin.RouterGroup, h *handler.RNDHandler) {
	p := g.Group("/products")
	{
		p.GET("", h.ProductList)
		p.GET("/:id", h.ProductGet)
		p.POST("", h.ProductCreate)
		p.PUT("/:id", h.ProductUpdate)
		p.DELETE("/:id", h.ProductDelete)
	}
	b := g.Group("/boms")
	{
		b.GET("", h.BOMList)
		b.GET("/product/:pid", h.BOMListByProduct)
		b.GET("/:id", h.BOMGet)
		b.POST("", h.BOMCreate)
		b.PUT("/:id", h.BOMUpdate)
		b.PUT("/:id/release", h.BOMRelease)
		b.DELETE("/:id", h.BOMDelete)
	}
}

func registerApproval(g *gin.RouterGroup, h *handler.ApprovalHandler) {
	grp := g.Group("/approval-groups")
	{
		grp.GET("", h.GroupList)
		grp.POST("", h.GroupCreate)
		grp.PUT("/:id", h.GroupUpdate)
		grp.DELETE("/:id", h.GroupDelete)
	}
	ap := g.Group("/approvals")
	{
		ap.POST("", h.Submit)
		ap.GET("/pending", h.Pending)
		ap.GET("/processed", h.Processed)
		ap.GET("/submitted", h.Submitted)
		ap.GET("/:id", h.Get)
		ap.POST("/:id/action", h.Act)
	}
}

func registerStorage(g *gin.RouterGroup, h *handler.StorageHandler) {
	s := g.Group("/storage")
	{
		s.GET("/current", h.Current)
		s.GET("/data-sources", h.DataSourceList)
		s.POST("/data-sources", h.DataSourceCreate)
		s.DELETE("/data-sources/:id", h.DataSourceDelete)
		s.POST("/test-connection", h.TestConnection)
		s.POST("/migrate/:id", h.Migrate)
		s.GET("/migrate/status", h.Status)
	}
}

func registerSupplierMaterial(g *gin.RouterGroup, h *handler.SupplierMaterialHandler) {
	sm := g.Group("/supplier-material")
	{
		sm.GET("", h.List)
		sm.POST("", h.Bind)
		sm.PUT("/:id", h.Update)
		sm.DELETE("/:id", h.Unbind)
	}
}

func registerBOMOrder(g *gin.RouterGroup, h *handler.BOMOrderHandler) {
	g.POST("/bom-order/preview", h.Preview)
	g.POST("/bom-order/confirm", h.Confirm)
}

func registerApiKey(g *gin.RouterGroup, h *handler.ApiKeyHandler) {
	k := g.Group("/api-keys")
	{
		k.GET("", h.List)
		k.POST("", h.Create)
		k.PUT("/:id/disable", h.Disable)
		k.PUT("/:id/enable", h.Enable)
		k.DELETE("/:id", h.Delete)
	}
}

func registerRBAC(g *gin.RouterGroup, h *handler.RBACHandler) {
	t := g.Group("/tenants")
	{
		t.GET("", h.TenantList)
		t.POST("", h.TenantCreate)
		t.PUT("/:id", h.TenantUpdate)
	}
	u := g.Group("/users")
	{
		u.GET("", h.UserList)
		u.POST("", h.UserCreate)
		u.PUT("/:id", h.UserUpdate)
		u.DELETE("/:id", h.UserDelete)
		u.GET("/:id/roles", h.UserRoles)
	}
}

func registerBase(g *gin.RouterGroup, h *handler.BaseDataHandler) {
	m := g.Group("/materials")
	{
		m.GET("", h.MaterialList)
		m.GET("/:id", h.MaterialGet)
		m.POST("", h.MaterialCreate)
		m.PUT("/:id", h.MaterialUpdate)
		m.DELETE("/:id", h.MaterialDelete)
	}
	s := g.Group("/suppliers")
	{
		s.GET("", h.SupplierList)
		s.GET("/:id", h.SupplierGet)
		s.POST("", h.SupplierCreate)
		s.PUT("/:id", h.SupplierUpdate)
		s.PUT("/:id/audit", h.SupplierSetAudit)
		s.DELETE("/:id", h.SupplierDelete)
	}
	w := g.Group("/warehouses")
	{
		w.GET("", h.WarehouseList)
		w.GET("/:id", h.WarehouseGet)
		w.POST("", h.WarehouseCreate)
		w.PUT("/:id", h.WarehouseUpdate)
		w.DELETE("/:id", h.WarehouseDelete)
	}
	l := g.Group("/locations")
	{
		l.GET("", h.LocationList)
		l.GET("/:id", h.LocationGet)
		l.POST("", h.LocationCreate)
		l.PUT("/:id", h.LocationUpdate)
		l.DELETE("/:id", h.LocationDelete)
	}
}

func registerProcurement(g *gin.RouterGroup, h *handler.PurchaseOrderHandler, r *handler.ReceivingHandler) {
	p := g.Group("/po")
	{
		p.GET("", h.List)
		p.GET("/:id", h.Get)
		p.POST("", h.Create)
		p.PUT("/:id", h.Update)
		p.PUT("/:id/status", h.SetStatus)
		p.DELETE("/:id", h.Delete)
		p.POST("/:id/receive", r.Receive)
		p.GET("/:id/receipts", r.Receipts)
	}
}

func registerSales(g *gin.RouterGroup, h *handler.SalesHandler) {
	c := g.Group("/customers")
	{
		c.GET("", h.CustomerList)
		c.GET("/:id", h.CustomerGet)
		c.POST("", h.CustomerCreate)
		c.PUT("/:id", h.CustomerUpdate)
		c.DELETE("/:id", h.CustomerDelete)
	}
	o := g.Group("/so")
	{
		o.GET("", h.List)
		o.GET("/:id", h.Get)
		o.POST("", h.Create)
		o.PUT("/:id/approve", h.Approve)
		o.PUT("/:id/cancel", h.Cancel)
		o.DELETE("/:id", h.Delete)
	}
}

func registerInventory(g *gin.RouterGroup, h *handler.InventoryHandler, s *handler.StockHandler) {
	i := g.Group("/inventory")
	{
		i.GET("/stock", s.List)
		i.POST("/move-in", h.MoveIn)
		i.POST("/move-out", h.MoveOut)
		i.GET("/orders", h.ListOrders)
		i.GET("/orders/:id", h.GetOrder)
		i.DELETE("/orders/:id", h.DeleteOrder)
		i.GET("/logs", h.ListLogs)
	}
}

func registerPlanning(g *gin.RouterGroup, h *handler.PlanningHandler) {
	p := g.Group("/planning")
	{
		p.GET("/demands", h.DemandList)
		p.GET("/demands/:id", h.DemandGet)
		p.POST("/demands", h.DemandCreate)
		p.PUT("/demands/:id", h.DemandUpdate)
		p.DELETE("/demands/:id", h.DemandDelete)

		p.GET("/mrp", h.MrpList)
		p.GET("/mrp/:id", h.MrpGet)
		p.DELETE("/mrp/:id", h.MrpDelete)
		p.POST("/mrp/compute", h.ComputeMRP)
		p.POST("/mrp/:id/convert", h.MrpConvert)
	}
}

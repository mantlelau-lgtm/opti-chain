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
	Base      *handler.BaseDataHandler
	PO        *handler.PurchaseOrderHandler
	Receiving *handler.ReceivingHandler
	Sales     *handler.SalesHandler
	Inventory *handler.InventoryHandler
	Stock     *handler.StockHandler
	Planning  *handler.PlanningHandler
}

// New builds a configured gin engine with all routes registered.
func New(corsOrigin string, h *Handlers) *gin.Engine {
	g := gin.New()
	g.Use(middleware.Recovery())
	g.Use(gin.Logger())
	g.Use(middleware.CORS(corsOrigin))

	api := g.Group("/api/v1")
	{
		registerBase(api, h.Base)
		registerProcurement(api, h.PO, h.Receiving)
		registerSales(api, h.Sales)
		registerInventory(api, h.Inventory, h.Stock)
		registerPlanning(api, h.Planning)
	}
	return g
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

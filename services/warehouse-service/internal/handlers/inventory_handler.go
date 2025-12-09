package handlers

import (
	"net/http"
	"sync"

	"observability-system/shared/logger"
	"observability-system/shared/tracing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
)

var (
	inventoryMu sync.RWMutex
	inventory   = map[string]*InventoryItem{
		"PROD-001": {ProductID: "PROD-001", Name: "Laptop", Quantity: 100, Reserved: 0},
		"PROD-002": {ProductID: "PROD-002", Name: "Monitor", Quantity: 50, Reserved: 0},
		"PROD-003": {ProductID: "PROD-003", Name: "Keyboard", Quantity: 200, Reserved: 0},
		"PROD-004": {ProductID: "PROD-004", Name: "Mouse", Quantity: 150, Reserved: 0},
		"PROD-005": {ProductID: "PROD-005", Name: "Headphones", Quantity: 75, Reserved: 0},
	}
)

type InventoryItem struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Reserved  int    `json:"reserved"`
	Available int    `json:"available"`
}

type InventoryHandler struct {
	logger logger.Logger
}

func NewInventoryHandler(log logger.Logger) *InventoryHandler {
	return &InventoryHandler{
		logger: log,
	}
}

func (h *InventoryHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"service": "warehouse-service",
	})
}

func (h *InventoryHandler) CheckStock(c *gin.Context) {
	ctx := c.Request.Context()
	productID := c.Param("product_id")

	tracing.AddSpanAttributes(ctx,
		attribute.String("product.id", productID),
		attribute.String("operation", "check_stock"),
	)

	h.logger.InfoCtx(ctx, "Checking stock",
		logger.String("product_id", productID))

	inventoryMu.RLock()
	item, exists := inventory[productID]
	inventoryMu.RUnlock()

	if !exists {
		h.logger.WarnCtx(ctx, "Product not found",
			logger.String("product_id", productID))

		tracing.AddSpanAttributes(ctx, attribute.Bool("product.found", false))

		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Product not found",
			"product_id": productID,
		})
		return
	}

	available := item.Quantity - item.Reserved

	tracing.AddSpanAttributes(ctx,
		attribute.Bool("product.found", true),
		attribute.Int("stock.quantity", item.Quantity),
		attribute.Int("stock.reserved", item.Reserved),
		attribute.Int("stock.available", available),
	)

	h.logger.InfoCtx(ctx, "Stock check completed",
		logger.String("product_id", productID),
		logger.Int("available", available))

	c.JSON(http.StatusOK, gin.H{
		"product_id": item.ProductID,
		"name":       item.Name,
		"quantity":   item.Quantity,
		"reserved":   item.Reserved,
		"available":  available,
	})
}

func (h *InventoryHandler) ReserveStock(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		ProductID string `json:"product_id" binding:"required"`
		Quantity  int    `json:"quantity" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.ErrorCtx(ctx, "Invalid request body",
			logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	tracing.AddSpanAttributes(ctx,
		attribute.String("product.id", req.ProductID),
		attribute.Int("reservation.quantity", req.Quantity),
		attribute.String("operation", "reserve_stock"),
	)

	h.logger.InfoCtx(ctx, "Reserving stock",
		logger.String("product_id", req.ProductID),
		logger.Int("quantity", req.Quantity))

	inventoryMu.Lock()
	defer inventoryMu.Unlock()

	item, exists := inventory[req.ProductID]
	if !exists {
		tracing.AddSpanAttributes(ctx, attribute.Bool("product.found", false))
		h.logger.WarnCtx(ctx, "Product not found for reservation",
			logger.String("product_id", req.ProductID))

		c.JSON(http.StatusNotFound, gin.H{
			"error":      "Product not found",
			"product_id": req.ProductID,
		})
		return
	}

	available := item.Quantity - item.Reserved

	if available < req.Quantity {
		tracing.AddSpanAttributes(ctx,
			attribute.Bool("reservation.success", false),
			attribute.String("reservation.failure_reason", "insufficient_stock"),
			attribute.Int("stock.available", available),
		)

		h.logger.WarnCtx(ctx, "Insufficient stock for reservation",
			logger.String("product_id", req.ProductID),
			logger.Int("requested", req.Quantity),
			logger.Int("available", available))

		c.JSON(http.StatusConflict, gin.H{
			"error":     "Insufficient stock",
			"available": available,
			"requested": req.Quantity,
		})
		return
	}

	item.Reserved += req.Quantity
	newAvailable := item.Quantity - item.Reserved

	tracing.AddSpanAttributes(ctx,
		attribute.Bool("reservation.success", true),
		attribute.Int("stock.new_reserved", item.Reserved),
		attribute.Int("stock.new_available", newAvailable),
	)

	h.logger.InfoCtx(ctx, "Stock reserved successfully",
		logger.String("product_id", req.ProductID),
		logger.Int("reserved_quantity", req.Quantity),
		logger.Int("new_available", newAvailable))

	c.JSON(http.StatusOK, gin.H{
		"message":           "Stock reserved successfully",
		"product_id":        req.ProductID,
		"reserved_quantity": req.Quantity,
		"new_available":     newAvailable,
	})
}

func (h *InventoryHandler) GetAllInventory(c *gin.Context) {
	ctx := c.Request.Context()

	tracing.AddSpanAttributes(ctx, attribute.String("operation", "get_all_inventory"))

	h.logger.InfoCtx(ctx, "Fetching all inventory")

	inventoryMu.RLock()
	items := make([]gin.H, 0, len(inventory))
	for _, item := range inventory {
		available := item.Quantity - item.Reserved
		items = append(items, gin.H{
			"product_id": item.ProductID,
			"name":       item.Name,
			"quantity":   item.Quantity,
			"reserved":   item.Reserved,
			"available":  available,
		})
	}
	inventoryMu.RUnlock()

	tracing.AddSpanAttributes(ctx, attribute.Int("inventory.count", len(items)))

	c.JSON(http.StatusOK, gin.H{
		"count":     len(items),
		"inventory": items,
	})
}

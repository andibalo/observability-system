package handlers

import (
	"net/http"
	"sync"
	"time"

	"observability-system/shared/logger"
	"observability-system/shared/tracing"
	"order-service/internal/clients"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

var (
	ordersMu sync.RWMutex
	orders   = make(map[string]*Order)
)

type Order struct {
	ID             string    `json:"id"`
	ProductID      string    `json:"product_id"`
	ProductName    string    `json:"product_name"`
	Quantity       int       `json:"quantity"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	StockReserved  bool      `json:"stock_reserved"`
	AvailableStock int       `json:"available_stock,omitempty"`
}

type OrderHandler struct {
	logger          logger.Logger
	warehouseClient *clients.WarehouseClient
}

func NewOrderHandler(log logger.Logger, warehouseClient *clients.WarehouseClient) *OrderHandler {
	return &OrderHandler{
		logger:          log,
		warehouseClient: warehouseClient,
	}
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
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

	orderID := uuid.New().String()

	tracing.AddSpanAttributes(ctx,
		attribute.String("order.id", orderID),
		attribute.String("product.id", req.ProductID),
		attribute.Int("order.quantity", req.Quantity),
		attribute.String("operation", "create_order"),
	)

	h.logger.InfoCtx(ctx, "Creating order",
		logger.String("order_id", orderID),
		logger.String("product_id", req.ProductID),
		logger.Int("quantity", req.Quantity))

	h.logger.InfoCtx(ctx, "Checking stock availability",
		logger.String("order_id", orderID))

	stockInfo, err := h.warehouseClient.CheckStock(ctx, req.ProductID)
	if err != nil {
		tracing.AddSpanAttributes(ctx,
			attribute.Bool("stock_check.success", false),
			attribute.String("error", err.Error()),
		)

		h.logger.ErrorCtx(ctx, "Failed to check stock",
			logger.Err(err),
			logger.String("order_id", orderID))

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":    "Failed to check stock availability",
			"order_id": orderID,
			"details":  err.Error(),
		})
		return
	}

	tracing.AddSpanAttributes(ctx,
		attribute.Bool("stock_check.success", true),
		attribute.Int("stock.available", stockInfo.Available),
	)

	if stockInfo.Available < req.Quantity {
		h.logger.WarnCtx(ctx, "Insufficient stock for order",
			logger.String("order_id", orderID),
			logger.Int("requested", req.Quantity),
			logger.Int("available", stockInfo.Available))

		tracing.AddSpanAttributes(ctx,
			attribute.Bool("order.rejected", true),
			attribute.String("rejection_reason", "insufficient_stock"),
		)

		c.JSON(http.StatusConflict, gin.H{
			"error":     "Insufficient stock",
			"order_id":  orderID,
			"available": stockInfo.Available,
			"requested": req.Quantity,
		})
		return
	}

	h.logger.InfoCtx(ctx, "Reserving stock",
		logger.String("order_id", orderID))

	reservation, err := h.warehouseClient.ReserveStock(ctx, req.ProductID, req.Quantity)
	if err != nil {
		tracing.AddSpanAttributes(ctx,
			attribute.Bool("stock_reservation.success", false),
			attribute.String("error", err.Error()),
		)

		h.logger.ErrorCtx(ctx, "Failed to reserve stock",
			logger.Err(err),
			logger.String("order_id", orderID))

		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":    "Failed to reserve stock",
			"order_id": orderID,
			"details":  err.Error(),
		})
		return
	}

	tracing.AddSpanAttributes(ctx,
		attribute.Bool("stock_reservation.success", true),
		attribute.Int("stock.reserved", reservation.ReservedQuantity),
	)

	order := &Order{
		ID:             orderID,
		ProductID:      req.ProductID,
		ProductName:    stockInfo.Name,
		Quantity:       req.Quantity,
		Status:         "confirmed",
		CreatedAt:      time.Now(),
		StockReserved:  true,
		AvailableStock: reservation.NewAvailable,
	}

	ordersMu.Lock()
	orders[orderID] = order
	ordersMu.Unlock()

	tracing.AddSpanAttributes(ctx,
		attribute.Bool("order.created", true),
		attribute.String("order.status", order.Status),
	)

	h.logger.InfoCtx(ctx, "Order created successfully",
		logger.String("order_id", orderID),
		logger.String("status", order.Status))

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Order created successfully",
		"order":          order,
		"stock_reserved": reservation.ReservedQuantity,
		"request_id":     logger.GetRequestIDFromGin(c),
	})
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	ctx := c.Request.Context()
	orderID := c.Param("order_id")

	tracing.AddSpanAttributes(ctx,
		attribute.String("order.id", orderID),
		attribute.String("operation", "get_order"),
	)

	h.logger.InfoCtx(ctx, "Fetching order",
		logger.String("order_id", orderID))

	ordersMu.RLock()
	order, exists := orders[orderID]
	ordersMu.RUnlock()

	if !exists {
		tracing.AddSpanAttributes(ctx, attribute.Bool("order.found", false))

		c.JSON(http.StatusNotFound, gin.H{
			"error":    "Order not found",
			"order_id": orderID,
		})
		return
	}

	tracing.AddSpanAttributes(ctx,
		attribute.Bool("order.found", true),
		attribute.String("order.status", order.Status),
	)

	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) GetAllOrders(c *gin.Context) {
	ctx := c.Request.Context()

	tracing.AddSpanAttributes(ctx, attribute.String("operation", "get_all_orders"))

	h.logger.InfoCtx(ctx, "Fetching all orders")

	ordersMu.RLock()
	orderList := make([]*Order, 0, len(orders))
	for _, order := range orders {
		orderList = append(orderList, order)
	}
	ordersMu.RUnlock()

	tracing.AddSpanAttributes(ctx, attribute.Int("orders.count", len(orderList)))

	c.JSON(http.StatusOK, gin.H{
		"count":  len(orderList),
		"orders": orderList,
	})
}

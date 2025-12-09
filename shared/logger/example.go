package logger

import (
	"context"

	"github.com/gin-gonic/gin"
)

// Example usage of the logger with Gin and dependency injection

func ExampleBasicLogging() {
	// Create a logger instance (no singleton!)
	logger, err := NewDefaultLogger("order-service", "production")
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Basic logging
	logger.Info("Application started")
	logger.Debug("Debug information", String("key", "value"))
	logger.Warn("Warning message", Int("count", 10))
	logger.Error("Error occurred", Err(nil))
}

func ExampleGinSetup() {
	// Create logger instance
	logger, err := NewDefaultLogger("order-service", "development")
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Create Gin router
	router := gin.New()

	// Inject logger into Gin context (optional but recommended)
	router.Use(InjectLogger(logger))

	// Add logger middleware with dependency injection
	router.Use(GinMiddleware(logger))

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Define routes - pass logger as dependency
	orderHandler := NewOrderHandler(logger)
	router.GET("/api/orders/:id", orderHandler.GetOrder)
	router.POST("/api/orders", orderHandler.CreateOrder)

	router.Run(":8080")
}

// OrderHandler demonstrates dependency injection pattern
type OrderHandler struct {
	logger  Logger
	service *OrderService
}

func NewOrderHandler(logger Logger) *OrderHandler {
	return &OrderHandler{
		logger:  logger,
		service: NewOrderService(logger),
	}
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	ctx := c.Request.Context()
	orderID := c.Param("id")

	// Use injected logger
	h.logger.InfoCtx(ctx, "Fetching order",
		String("order_id", orderID))

	// Or get from Gin context
	logger := GetLoggerFromGin(c)
	logger.Info("Processing request", String("order_id", orderID))

	c.JSON(200, gin.H{
		"order_id": orderID,
		"status":   "completed",
	})
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	ctx := c.Request.Context()

	var order struct {
		CustomerID string `json:"customer_id"`
		Amount     int    `json:"amount"`
	}

	if err := c.ShouldBindJSON(&order); err != nil {
		h.logger.ErrorCtx(ctx, "Invalid request body", Err(err))
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// Call service with context (logger is injected in service)
	if err := h.service.CreateOrder(ctx, order.CustomerID, order.Amount); err != nil {
		h.logger.ErrorCtx(ctx, "Failed to create order", Err(err))
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, gin.H{
		"message":    "Order created",
		"request_id": GetRequestIDFromGin(c),
	})
}

// OrderService demonstrates service layer with injected logger
type OrderService struct {
	logger Logger
	repo   *OrderRepository
}

func NewOrderService(logger Logger) *OrderService {
	return &OrderService{
		logger: logger,
		repo:   NewOrderRepository(logger),
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, customerID string, amount int) error {
	// Service layer logs with injected logger
	s.logger.InfoCtx(ctx, "Service: Creating order",
		String("customer_id", customerID),
		Int("amount", amount))

	// Call repository layer
	return s.repo.Save(ctx, customerID, amount)
}

// OrderRepository demonstrates repository layer with injected logger
type OrderRepository struct {
	logger Logger
}

func NewOrderRepository(logger Logger) *OrderRepository {
	return &OrderRepository{logger: logger}
}

func (r *OrderRepository) Save(ctx context.Context, customerID string, amount int) error {
	// Repository logs will also have the same request_id from context
	r.logger.DebugCtx(ctx, "Repository: Saving order to database",
		String("customer_id", customerID))
	return nil
}

// Example of switching logger implementation
func ExampleSwitchLogger() {
	// Easy to switch to a different logger implementation
	// Just implement the Logger interface

	// Using Zap (default)
	zapLogger, _ := NewZapLogger(Config{
		ServiceName: "order-service",
		Environment: "production",
		Level:       InfoLevel,
	})

	// In the future, you could create:
	// logrusLogger := NewLogrusLogger(config)
	// zerologLogger := NewZerologLogger(config)

	// All use the same interface!
	useLogger(zapLogger)
}

func useLogger(logger Logger) {
	logger.Info("This works with any logger implementation!")
}

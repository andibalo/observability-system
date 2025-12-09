package routes

import (
	"observability-system/shared/logger"
	"observability-system/shared/tracing"
	"order-service/internal/handlers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(
	router *gin.Engine,
	log logger.Logger,
	serviceName string,
	inboxHandler *handlers.InboxHandler,
	orderHandler *handlers.OrderHandler,
) {

	router.Use(tracing.GinMiddleware(serviceName))

	router.Use(logger.InjectLogger(log))
	router.Use(logger.GinMiddleware(log))
	router.Use(gin.Recovery())

	router.GET("/health", inboxHandler.HealthCheck)

	api := router.Group("/api")
	{
		api.POST("/inbox", inboxHandler.CreateInboxMessage)
		api.GET("/inbox", inboxHandler.GetInboxMessages)

		api.POST("/orders", orderHandler.CreateOrder)
		api.GET("/orders", orderHandler.GetAllOrders)
		api.GET("/orders/:order_id", orderHandler.GetOrder)
	}
}

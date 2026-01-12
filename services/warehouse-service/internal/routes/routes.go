package routes

import (
	"observability-system/shared/logger"
	"observability-system/shared/tracing"
	"warehouse-service/internal/handlers"
	"warehouse-service/internal/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRoutes(router *gin.Engine, log logger.Logger, serviceName string, handler *handlers.InventoryHandler) {

	router.Use(tracing.GinMiddleware(serviceName))

	router.Use(logger.InjectLogger(log))
	router.Use(logger.GinMiddleware(log))
	router.Use(gin.Recovery())

	router.Use(metrics.PrometheusMiddleware(serviceName))

	router.GET("/health", handler.HealthCheck)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := router.Group("/api")
	{
		api.GET("/inventory", handler.GetAllInventory)
		api.GET("/inventory/:product_id", handler.CheckStock)
		api.POST("/inventory/reserve", handler.ReserveStock)
	}
}

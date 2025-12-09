package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"observability-system/shared/logger"
	"observability-system/shared/tracing"
	"warehouse-service/internal/config"
	"warehouse-service/internal/handlers"
	"warehouse-service/internal/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	log, err := logger.NewDefaultLogger(cfg.ServiceName, cfg.Environment)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer log.Sync()

	log.Info("Starting warehouse service",
		logger.String("port", cfg.Port),
		logger.String("environment", cfg.Environment),
		logger.String("jaeger_endpoint", cfg.JaegerEndpoint))

	tracingCfg := tracing.Config{
		ServiceName:    cfg.ServiceName,
		ServiceVersion: "1.0.0",
		Environment:    cfg.Environment,
		JaegerEndpoint: cfg.JaegerEndpoint,
	}

	if err := tracing.InitTracer(tracingCfg); err != nil {
		log.Fatal("Failed to initialize tracer",
			logger.Err(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracing.ShutdownTracer(ctx); err != nil {
			log.Error("Error shutting down tracer", logger.Err(err))
		}
	}()

	log.Info("Tracer initialized successfully")

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	inventoryHandler := handlers.NewInventoryHandler(log)

	routes.SetupRoutes(router, log, cfg.ServiceName, inventoryHandler)

	log.Info("Routes configured")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Info("Server starting",
		logger.String("address", addr))

	go func() {
		if err := router.Run(addr); err != nil {
			log.Fatal("Failed to start server",
				logger.Err(err))
		}
	}()

	<-sigChan
	log.Info("Shutdown signal received, initiating graceful shutdown")

	time.Sleep(2 * time.Second)

	log.Info("Service shutdown complete")
}

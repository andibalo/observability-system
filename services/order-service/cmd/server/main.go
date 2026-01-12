package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"observability-system/shared/logger"
	"observability-system/shared/messaging/rabbitmq"
	"observability-system/shared/tracing"
	"order-service/internal/clients"
	"order-service/internal/config"
	"order-service/internal/database"
	"order-service/internal/handlers"
	"order-service/internal/inbox"
	"order-service/internal/metrics"
	"order-service/internal/outbox"
	"order-service/internal/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	log, err := logger.NewDefaultLogger(cfg.ServiceName, cfg.Environment)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}
	defer log.Sync()

	log.Info("Starting order service",
		logger.String("port", cfg.Port),
		logger.String("environment", cfg.Environment),
		logger.String("warehouse_url", cfg.WarehouseServiceURL),
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

	// Initialize Prometheus metrics
	metrics.InitMetrics(cfg.ServiceName)
	log.Info("Metrics initialized successfully")

	db, err := database.NewConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database",
			logger.Err(err))
	}
	defer db.Close()

	log.Info("Connected to database successfully")

	if err := database.InitSchema(db); err != nil {
		log.Fatal("Failed to initialize database schema",
			logger.Err(err))
	}

	log.Info("Database schema initialized")

	var rabbitMQClient *rabbitmq.Client
	if cfg.EnableBroker {
		rabbitMQClient, err := rabbitmq.NewClient(cfg.RabbitMQURL)
		if err != nil {
			log.Fatal("Failed to connect to RabbitMQ",
				logger.Err(err))
		}
		defer rabbitMQClient.Close()

		log.Info("Connected to RabbitMQ successfully")

		if err := rabbitmq.SetupExchangesAndQueues(rabbitMQClient); err != nil {
			log.Fatal("Failed to setup RabbitMQ exchanges and queues",
				logger.Err(err))
		}

		log.Info("RabbitMQ exchanges and queues configured")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inboxStore := inbox.NewInboxStore(db)
	outboxStore := outbox.NewOutboxStore(db)

	warehouseClient := clients.NewWarehouseClient(cfg.WarehouseServiceURL, log)

	inboxHandler := handlers.NewInboxHandler(log, inboxStore)
	orderHandler := handlers.NewOrderHandler(log, warehouseClient)

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	routes.SetupRoutes(router, log, cfg.ServiceName, inboxHandler, orderHandler)

	log.Info("Routes configured")

	log.Info("Initializing message handler registry")
	registry := handlers.NewMessageHandlerRegistry(log)

	orderEvents := handlers.NewOrderEventHandler(log)
	registry.Register("order.created", orderEvents.HandleOrderCreated)
	registry.Register("order.updated", orderEvents.HandleOrderUpdated)
	registry.Register("order.cancelled", orderEvents.HandleOrderCancelled)

	log.Info("Message handlers registered",
		logger.Int("handler_count", len(registry.ListRegisteredHandlers())))

	messageHandler := registry.GetHandler()

	log.Info("Starting inbox workers",
		logger.Int("count", 3),
		logger.Int("max_retries", cfg.MaxRetries))
	inboxWorkers := make([]*inbox.InboxWorker, 3)
	for i := 0; i < 3; i++ {
		worker := inbox.NewInboxWorker(inboxStore, messageHandler, log, 3, 5*time.Second, cfg.MaxRetries)
		inboxWorkers[i] = worker
		go worker.Start(ctx)

		log.Info("Inbox worker started", logger.Int("worker_number", i+1))
	}

	log.Info("Starting outbox workers", logger.Int("count", 3))
	outboxWorkers := make([]*outbox.OutboxWorker, 3)
	for i := 0; i < 3; i++ {
		worker := outbox.NewOutboxWorker(outboxStore, rabbitMQClient, log, 3, 5*time.Second)
		outboxWorkers[i] = worker
		go worker.Start(ctx)

		log.Info("Outbox worker started", logger.Int("worker_number", i+1))
	}

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

	cancel()

	log.Info("Stopping inbox workers")
	for i, worker := range inboxWorkers {
		worker.Stop()
		log.Info("Inbox worker stopped", logger.Int("worker_number", i+1))
	}

	log.Info("Stopping outbox workers")
	for i, worker := range outboxWorkers {
		worker.Stop()
		log.Info("Outbox worker stopped", logger.Int("worker_number", i+1))
	}

	time.Sleep(2 * time.Second)

	if cfg.EnableBroker {
		if err := rabbitMQClient.Close(); err != nil {
			log.Error("Error closing RabbitMQ connection", logger.Err(err))
		} else {
			log.Info("RabbitMQ connection closed")
		}
	}

	log.Info("Service shutdown complete")
}

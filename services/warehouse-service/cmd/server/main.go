package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"observability-system/shared/logger"
	"observability-system/shared/messaging"
	"observability-system/shared/messaging/rabbitmq"
	"observability-system/shared/tracing"
	"warehouse-service/internal/config"
	"warehouse-service/internal/database"
	"warehouse-service/internal/handlers"
	"warehouse-service/internal/inbox"
	"warehouse-service/internal/metrics"
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

	metrics.InitMetrics(cfg.ServiceName)
	log.Info("Metrics initialized successfully")

	var rabbitMQClient *rabbitmq.Client
	if cfg.EnableBroker {
		rabbitMQClient, err = rabbitmq.NewClient(cfg.RabbitMQURL)
		if err != nil {
			log.Fatal("Failed to connect to RabbitMQ", logger.Err(err))
		}
		defer rabbitMQClient.Close()
		log.Info("Connected to RabbitMQ successfully")

		if err := rabbitmq.SetupExchangesAndQueues(rabbitMQClient); err != nil {
			log.Fatal("Failed to setup RabbitMQ exchanges and queues", logger.Err(err))
		}
		log.Info("RabbitMQ exchanges and queues configured")
	}

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	if cfg.EnableBroker {

		inboxStore := inbox.NewInboxStore(db)

		testHandler := func(msg messaging.Message) error {
			bytes, _ := json.Marshal(msg.Payload)
			payloadStr := string(bytes)

			log.Info("Received warehouse test message",
				logger.String("message_id", msg.ID),
				logger.String("payload", payloadStr))
			return nil
		}

		msgHandler := inbox.InboxHandler(inboxStore, testHandler)

		err = rabbitMQClient.Subscribe("warehouse.test", msgHandler)
		if err != nil {
			log.Fatal("Failed to subscribe to warehouse.test", logger.Err(err))
		}
		log.Info("Subscribed to warehouse.test queue")
	}

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

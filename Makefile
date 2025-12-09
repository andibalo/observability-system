.PHONY: help build run-order run-warehouse test docker-up docker-down

help:
	@echo "Available commands:"
	@echo "  make build           - Build all services"
	@echo "  make run-order       - Run order service"
	@echo "  make run-warehouse   - Run warehouse service"
	@echo "  make test            - Run all tests"
	@echo "  make docker-up       - Start services with Docker"
	@echo "  make docker-down     - Stop Docker services"

build:
	cd services/order-service && go build -o bin/order-service ./cmd/server
	cd services/warehouse-service && go build -o bin/warehouse-service ./cmd/server

run-order:
	cd services/order-service && go run cmd/server/main.go

run-warehouse:
	cd services/warehouse-service && go run cmd/server/main.go

test:
	cd services/order-service && go test ./...
	cd services/warehouse-service && go test ./...

docker-up:
	cd infrastructure && docker-compose up --build

docker-down:
	cd infrastructure && docker-compose down

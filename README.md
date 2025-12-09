# Inbox-Outbox Pattern Microservices

A production-ready microservices implementation demonstrating the **Inbox/Outbox pattern** for reliable, event-driven communication using Go, PostgreSQL, RabbitMQ, and Docker.

## 🎯 Features

- ✅ **Distributed Tracing** - OpenTelemetry with Jaeger for end-to-end trace visibility
- ✅ **Inbox/Outbox Pattern** - Guaranteed message delivery with idempotency
- ✅ **Event-Driven Architecture** - Async communication via RabbitMQ
- ✅ **Gin HTTP Framework** - Fast, lightweight REST API
- ✅ **sqlx Database Library** - Type-safe SQL with struct scanning
- ✅ **Dependency Injection** - No singletons, testable code
- ✅ **Logger-Agnostic Interface** - Swap logging libraries easily
- ✅ **Request ID Tracking** - Distributed tracing support
- ✅ **Connection Pooling** - Optimized database performance
- ✅ **Docker Compose** - One-command infrastructure setup
- ✅ **Health Checks** - Service monitoring and readiness

## Services

This project contains two microservices built with Go:
- **Order Service** (Port 8001): Manages customer orders 
- **Warehouse Service** (Port 8002): Manages inventory

## 🚀 Quick Start

### Prerequisites
- **Go 1.21+** - [Download](https://golang.org/dl/)
- **Docker Desktop** - [Download](https://www.docker.com/products/docker-desktop))

**1. Start Infrastructure:**
```powershell
cd infrastructure
docker-compose up -d
```

**2. Run Order Service:**
```powershell
cd services\order-service
.\bin\order-service.exe
```

## Project Structure

```
.
├── services/
│   ├── order-service/       # Order management microservice
│   │   ├── cmd/server/      # Application entrypoint
│   │   ├── internal/        # Private application code
│   │   │   ├── handlers/    # HTTP handlers
│   │   │   ├── services/    # Business logic
│   │   │   ├── models/      # Data models
│   │   │   ├── routes/      # Route definitions
│   │   │   ├── middleware/  # HTTP middleware
│   │   │   └── config/      # Configuration
│   │   └── tests/           # Test files
│   │
│   └── warehouse-service/   # Warehouse management microservice
│       ├── cmd/server/
│       ├── internal/
│       └── tests/
│
├── shared/                  # Shared utilities
│   ├── tracing/             # OpenTelemetry tracing package
│   ├── logger/              # Shared logging
│   ├── utils/
│   ├── types/
│   └── constants/
│
└── infrastructure/          # Deployment configs
    ├── docker-compose.yml   # Jaeger, RabbitMQ, PostgreSQL
    ├── kubernetes/
    └── nginx/
```

## Getting Started

### Prerequisites
- Go 1.21 or higher
- Docker & Docker Compose (optional)

### Run Services Locally

**Order Service:**
```bash
cd services/order-service
go mod download
go run cmd/server/main.go
```

**Warehouse Service:**
```bash
cd services/warehouse-service
go mod download
go run cmd/server/main.go
```

### Run with Docker

```bash
cd infrastructure
docker-compose up --build
```

### Run with Kubernetes

```bash
kubectl apply -f infrastructure/kubernetes/
```

## API Endpoints

### Order Service (http://localhost:8001)
- `GET /health` - Health check
- `POST /api/orders` - Create order (calls warehouse-service to check/reserve stock)
- `GET /api/orders` - Get all orders
- `GET /api/orders/:order_id` - Get order by ID
- `POST /api/inbox` - Create inbox message
- `GET /api/inbox` - Get all inbox messages

### Warehouse Service (http://localhost:8002)
- `GET /health` - Health check
- `GET /api/inventory` - Get all inventory items
- `GET /api/inventory/:product_id` - Get stock for a product
- `POST /api/inventory/reserve` - Reserve stock for an order

## Development

### Running Tests
```bash
# In each service directory
go test ./...
```

### Building
```bash
# In each service directory
go build -o bin/service ./cmd/server
```

## Environment Variables

See `.env.example` files in each service directory.

## Architecture

The services communicate via:
- **REST APIs**: Synchronous communication for direct requests
- **RabbitMQ with Inbox/Outbox Pattern**: Asynchronous communication for event-driven workflows

### Inbox/Outbox Pattern
- **Outbox**: Each service stores events in a local outbox table before publishing to RabbitMQ
- **Inbox**: Each service uses an inbox table to ensure idempotent message processing
- **Benefits**: Guarantees exactly-once delivery, prevents message loss, ensures data consistency

## RabbitMQ Management

Access RabbitMQ Management UI at http://localhost:15672
- Username: `admin`
- Password: `admin`

## Jaeger (Distributed Tracing)

Access Jaeger UI at http://localhost:16686

### Viewing Traces

1. Start infrastructure: `cd infrastructure && docker-compose up -d`
2. Run both services (warehouse first, then order)
3. Make a request to create an order:
   ```powershell
   curl -X POST http://localhost:8001/api/orders -H "Content-Type: application/json" -d '{"product_id": "PROD-001", "quantity": 2}'
   ```
4. Open Jaeger UI at http://localhost:16686
5. Select `order-service` from the Service dropdown
6. Click "Find Traces" to see the distributed trace

The trace will show:
- The incoming HTTP request to order-service
- The outgoing HTTP call to warehouse-service (check stock)
- The outgoing HTTP call to warehouse-service (reserve stock)
- Span attributes with product IDs, quantities, and operation details

### Available Products (Mock Data)

The warehouse-service has these pre-loaded products:
- `PROD-001` - Laptop (100 units)
- `PROD-002` - Monitor (50 units)
- `PROD-003` - Keyboard (200 units)
- `PROD-004` - Mouse (150 units)
- `PROD-005` - Headphones (75 units)

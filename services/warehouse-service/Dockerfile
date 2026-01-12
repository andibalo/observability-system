FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/warehouse-service/go.mod services/warehouse-service/go.sum ./services/warehouse-service/
WORKDIR /app/services/warehouse-service
RUN go mod download

COPY services/warehouse-service/ .
RUN go build -o main ./cmd/server

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/services/warehouse-service/main .

EXPOSE 8002

CMD ["./main"]

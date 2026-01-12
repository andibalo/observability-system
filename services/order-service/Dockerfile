FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy shared module
COPY shared/ ./shared/

# Copy service files
COPY services/order-service/go.mod services/order-service/go.sum ./services/order-service/
WORKDIR /app/services/order-service
RUN go mod download

COPY services/order-service/ .
RUN go build -o main ./cmd/server

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/services/order-service/main .

EXPOSE 8001

CMD ["./main"]

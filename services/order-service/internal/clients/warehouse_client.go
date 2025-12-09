package clients

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"observability-system/shared/httpclient"
	"observability-system/shared/logger"
	"observability-system/shared/tracing"

	"go.opentelemetry.io/otel/attribute"
)

type StockInfo struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	Reserved  int    `json:"reserved"`
	Available int    `json:"available"`
}

type ReservationResult struct {
	Message          string `json:"message"`
	ProductID        string `json:"product_id"`
	ReservedQuantity int    `json:"reserved_quantity"`
	NewAvailable     int    `json:"new_available"`
}

type WarehouseClient struct {
	client *httpclient.Client
	logger logger.Logger
}

func NewWarehouseClient(baseURL string, log logger.Logger) *WarehouseClient {
	return &WarehouseClient{
		client: httpclient.NewWithBaseURL(strings.TrimSuffix(baseURL, "/"), 30*time.Second),
		logger: log,
	}
}

func (c *WarehouseClient) CheckStock(ctx context.Context, productID string) (*StockInfo, error) {
	url := fmt.Sprintf("/api/inventory/%s", productID)

	c.logger.InfoCtx(ctx, "Checking stock from warehouse service",
		logger.String("product_id", productID),
		logger.String("url", url))

	tracing.AddSpanAttributes(ctx,
		attribute.String("warehouse.operation", "check_stock"),
		attribute.String("product.id", productID),
	)

	var stockInfo StockInfo
	resp, err := c.client.R(ctx).
		SetSpanName("HTTP GET /api/inventory/:product_id").
		AddSpanAttribute("product.id", productID).
		SetResult(&stockInfo).
		Get(url)

	if err != nil {
		c.logger.ErrorCtx(ctx, "Failed to call warehouse service",
			logger.Err(err),
			logger.String("product_id", productID))
		return nil, fmt.Errorf("warehouse service call failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		c.logger.WarnCtx(ctx, "Warehouse service returned non-OK status",
			logger.Int("status_code", resp.StatusCode()),
			logger.String("product_id", productID))

		if resp.StatusCode() == http.StatusNotFound {
			return nil, fmt.Errorf("product not found: %s", productID)
		}
		return nil, fmt.Errorf("warehouse service error: status %d", resp.StatusCode())
	}

	c.logger.InfoCtx(ctx, "Stock check completed",
		logger.String("product_id", productID),
		logger.Int("available", stockInfo.Available))

	return &stockInfo, nil
}

func (c *WarehouseClient) ReserveStock(ctx context.Context, productID string, quantity int) (*ReservationResult, error) {
	url := "/api/inventory/reserve"

	c.logger.InfoCtx(ctx, "Reserving stock from warehouse service",
		logger.String("product_id", productID),
		logger.Int("quantity", quantity))

	tracing.AddSpanAttributes(ctx,
		attribute.String("warehouse.operation", "reserve_stock"),
		attribute.String("product.id", productID),
		attribute.Int("reservation.quantity", quantity),
	)

	reqBody := map[string]interface{}{
		"product_id": productID,
		"quantity":   quantity,
	}

	var result ReservationResult
	resp, err := c.client.R(ctx).
		SetSpanName("HTTP POST /api/inventory/reserve").
		AddSpanAttribute("product.id", productID).
		AddSpanAttribute("reservation.quantity", quantity).
		SetBody(reqBody).
		SetResult(&result).
		Post(url)

	if err != nil {
		c.logger.ErrorCtx(ctx, "Failed to call warehouse service for reservation",
			logger.Err(err),
			logger.String("product_id", productID))
		return nil, fmt.Errorf("warehouse service call failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		c.logger.WarnCtx(ctx, "Warehouse service reservation failed",
			logger.Int("status_code", resp.StatusCode()),
			logger.String("product_id", productID))

		if resp.StatusCode() == http.StatusNotFound {
			return nil, fmt.Errorf("product not found: %s", productID)
		}
		if resp.StatusCode() == http.StatusConflict {
			return nil, fmt.Errorf("insufficient stock for product: %s", productID)
		}
		return nil, fmt.Errorf("warehouse service error: status %d", resp.StatusCode())
	}

	c.logger.InfoCtx(ctx, "Stock reservation completed",
		logger.String("product_id", productID),
		logger.Int("reserved", result.ReservedQuantity))

	return &result, nil
}

package tracing

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/gin-gonic/gin"
)

func GinMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}

type TracedHTTPClient struct {
	client *http.Client
}

func NewTracedHTTPClient(timeout time.Duration) *TracedHTTPClient {
	return &TracedHTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *TracedHTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	tracer := otel.Tracer("http-client")
	ctx, span := tracer.Start(ctx, "HTTP "+req.Method+" "+req.URL.Path,
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.host", req.URL.Host),
	)

	InjectTraceContext(ctx, req)

	req = req.WithContext(ctx)

	resp, err := c.client.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
	)

	return resp, nil
}

func (c *TracedHTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}

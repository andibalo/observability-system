package httpclient

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Client struct {
	resty      *resty.Client
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

type Config struct {
	BaseURL          string
	Timeout          time.Duration
	RetryCount       int
	RetryWaitTime    time.Duration
	RetryMaxWaitTime time.Duration
}

func DefaultConfig() Config {
	return Config{
		Timeout:          30 * time.Second,
		RetryCount:       3,
		RetryWaitTime:    100 * time.Millisecond,
		RetryMaxWaitTime: 2 * time.Second,
	}
}

func New(cfg Config) *Client {
	client := resty.New().
		SetTimeout(cfg.Timeout).
		SetRetryCount(cfg.RetryCount).
		SetRetryWaitTime(cfg.RetryWaitTime).
		SetRetryMaxWaitTime(cfg.RetryMaxWaitTime)

	if cfg.BaseURL != "" {
		client.SetBaseURL(cfg.BaseURL)
	}

	return &Client{
		resty:      client,
		tracer:     otel.Tracer("httpclient"),
		propagator: otel.GetTextMapPropagator(),
	}
}

func NewWithBaseURL(baseURL string, timeout time.Duration) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Timeout = timeout
	return New(cfg)
}

func (c *Client) R(ctx context.Context) *TracedRequest {
	return &TracedRequest{
		client:  c,
		request: c.resty.R().SetContext(ctx),
		ctx:     ctx,
	}
}

func (c *Client) GetRestyClient() *resty.Client {
	return c.resty
}

type TracedRequest struct {
	client    *Client
	request   *resty.Request
	ctx       context.Context
	spanName  string
	spanAttrs []attribute.KeyValue
}

func (r *TracedRequest) SetHeader(key, value string) *TracedRequest {
	r.request.SetHeader(key, value)
	return r
}

func (r *TracedRequest) SetHeaders(headers map[string]string) *TracedRequest {
	r.request.SetHeaders(headers)
	return r
}

func (r *TracedRequest) SetBody(body interface{}) *TracedRequest {
	r.request.SetBody(body)
	return r
}

func (r *TracedRequest) SetResult(result interface{}) *TracedRequest {
	r.request.SetResult(result)
	return r
}

func (r *TracedRequest) SetError(err interface{}) *TracedRequest {
	r.request.SetError(err)
	return r
}

func (r *TracedRequest) SetQueryParam(key, value string) *TracedRequest {
	r.request.SetQueryParam(key, value)
	return r
}

func (r *TracedRequest) SetQueryParams(params map[string]string) *TracedRequest {
	r.request.SetQueryParams(params)
	return r
}

func (r *TracedRequest) SetPathParam(key, value string) *TracedRequest {
	r.request.SetPathParam(key, value)
	return r
}

func (r *TracedRequest) SetPathParams(params map[string]string) *TracedRequest {
	r.request.SetPathParams(params)
	return r
}

func (r *TracedRequest) SetSpanName(name string) *TracedRequest {
	r.spanName = name
	return r
}

func (r *TracedRequest) AddSpanAttribute(key string, value interface{}) *TracedRequest {
	switch v := value.(type) {
	case string:
		r.spanAttrs = append(r.spanAttrs, attribute.String(key, v))
	case int:
		r.spanAttrs = append(r.spanAttrs, attribute.Int(key, v))
	case int64:
		r.spanAttrs = append(r.spanAttrs, attribute.Int64(key, v))
	case float64:
		r.spanAttrs = append(r.spanAttrs, attribute.Float64(key, v))
	case bool:
		r.spanAttrs = append(r.spanAttrs, attribute.Bool(key, v))
	}
	return r
}

func (r *TracedRequest) Get(url string) (*resty.Response, error) {
	return r.execute("GET", url)
}

func (r *TracedRequest) Post(url string) (*resty.Response, error) {
	return r.execute("POST", url)
}

func (r *TracedRequest) Put(url string) (*resty.Response, error) {
	return r.execute("PUT", url)
}

func (r *TracedRequest) Delete(url string) (*resty.Response, error) {
	return r.execute("DELETE", url)
}

func (r *TracedRequest) Patch(url string) (*resty.Response, error) {
	return r.execute("PATCH", url)
}

func (r *TracedRequest) execute(method, url string) (*resty.Response, error) {

	spanName := r.spanName
	if spanName == "" {
		spanName = "HTTP " + method
	}

	ctx, span := r.client.tracer.Start(r.ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", method),
		attribute.String("http.url", url),
	)

	if len(r.spanAttrs) > 0 {
		span.SetAttributes(r.spanAttrs...)
	}

	carrier := make(propagation.HeaderCarrier)
	r.client.propagator.Inject(ctx, carrier)
	for key, values := range carrier {
		if len(values) > 0 {
			r.request.SetHeader(key, values[0])
		}
	}

	r.request.SetContext(ctx)

	var resp *resty.Response
	var err error

	switch method {
	case "GET":
		resp, err = r.request.Get(url)
	case "POST":
		resp, err = r.request.Post(url)
	case "PUT":
		resp, err = r.request.Put(url)
	case "DELETE":
		resp, err = r.request.Delete(url)
	case "PATCH":
		resp, err = r.request.Patch(url)
	}

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("http.error", true))
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode()),
		attribute.Int64("http.response_size", int64(len(resp.Body()))),
	)

	return resp, nil
}

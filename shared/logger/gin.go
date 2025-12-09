package logger

import (
	"time"

	"github.com/gin-gonic/gin"
)

const RequestIDHeader = "X-Request-ID"

// GinMiddleware returns a Gin middleware that adds request_id to context and logs HTTP requests
// Requires a Logger instance to be injected
func GinMiddleware(logger Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get or generate request ID
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = GenerateRequestID()
		}

		// Add request ID to response header
		c.Header(RequestIDHeader, requestID)

		// Add request ID to Gin context
		c.Set("request_id", requestID)

		// Add request ID to request context
		ctx := WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)

		// Log request start
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		logger.InfoCtx(ctx, "HTTP request started",
			String("method", c.Request.Method),
			String("path", path),
			String("query", raw),
			String("ip", c.ClientIP()),
			String("user_agent", c.Request.UserAgent()),
		)

		// Process request
		c.Next()

		// Log request completion
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		fields := []Field{
			String("method", c.Request.Method),
			String("path", path),
			Int("status", statusCode),
			Duration("duration", duration),
			Int64("duration_ms", duration.Milliseconds()),
			String("error", c.Errors.ByType(gin.ErrorTypePrivate).String()),
		}

		// Log based on status code
		if statusCode >= 500 {
			logger.ErrorCtx(ctx, "HTTP request completed", fields...)
		} else if statusCode >= 400 {
			logger.WarnCtx(ctx, "HTTP request completed", fields...)
		} else {
			logger.InfoCtx(ctx, "HTTP request completed", fields...)
		}
	}
}

// GetRequestIDFromGin extracts request_id from Gin context
func GetRequestIDFromGin(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return GetRequestID(c.Request.Context())
}

// GetLoggerFromGin retrieves the logger from Gin context
func GetLoggerFromGin(c *gin.Context) Logger {
	if logger, exists := c.Get("logger"); exists {
		if l, ok := logger.(Logger); ok {
			return l.WithContext(c.Request.Context())
		}
	}
	// Fallback: create a new logger (not ideal, but prevents nil panics)
	l, _ := NewDefaultLogger("unknown", "development")
	return l.WithContext(c.Request.Context())
}

// InjectLogger is a middleware that injects the logger into Gin context
func InjectLogger(logger Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("logger", logger)
		c.Next()
	}
}

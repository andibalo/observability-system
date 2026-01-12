package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func PrometheusMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		path := c.FullPath()

		if path == "" {
			path = c.Request.URL.Path
		}
		HTTPRequestsTotal.WithLabelValues(serviceName, method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(serviceName, method, path).Observe(duration)
		HTTPResponseSize.WithLabelValues(serviceName, method, path).Observe(float64(c.Writer.Size()))
	}
}

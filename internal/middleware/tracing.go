package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Tracing starts an HTTP span for every request and links it to the request
// ID so logs and traces can be correlated. A nil tracer is a no-op, which
// keeps the middleware mountable when tracing is disabled.
func Tracing(tracer trace.Tracer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tracer == nil {
			c.Next()
			return
		}

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		ctx, span := tracer.Start(c.Request.Context(), c.Request.Method+" "+route,
			trace.WithAttributes(
				attribute.String("http.request.method", c.Request.Method),
				attribute.String("url.path", c.Request.URL.Path),
				attribute.String("request_id", GetRequestID(c)),
			),
		)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		span.SetAttributes(
			attribute.Int("http.response.status_code", c.Writer.Status()),
			attribute.Int64("http.response.body.size", int64(c.Writer.Size())),
		)
	}
}

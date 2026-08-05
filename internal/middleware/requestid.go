package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/vort-ads/vort-ads-template/pkg/idgen"
)

const RequestIDKey = "request_id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = idgen.MustNew("req")
		}
		c.Set(RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	requestID, _ := c.Get(RequestIDKey)
	if value, ok := requestID.(string); ok {
		return value
	}
	return c.Writer.Header().Get("X-Request-ID")
}

package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func AccessLog(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		event := log.Info()
		if len(c.Errors) > 0 || c.Writer.Status() >= 500 {
			event = log.Error()
		}
		if principal, ok := GetPrincipal(c); ok {
			event = event.Str("user_id", principal.UserID)
		}
		event.
			Str("request_id", GetRequestID(c)).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(started)).
			Msg("http request")
	}
}

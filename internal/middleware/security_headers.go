package middleware

import (
	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds standard response headers that protect against common
// browser-side attacks. The Content-Security-Policy header is intentionally
// not set here — front-end apps should manage their own CSP.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-XSS-Protection", "0")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		if c.Request.TLS != nil {
			header.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}

		c.Next()
	}
}

// CacheControl sets Cache-Control and Pragma headers for responses that must
// not be cached by browsers or intermediate proxies.
func CacheControl(private bool) gin.HandlerFunc {
	value := "no-cache, no-store, must-revalidate"
	if private {
		value = "private, " + value
	}
	return func(c *gin.Context) {
		c.Header("Cache-Control", value)
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}

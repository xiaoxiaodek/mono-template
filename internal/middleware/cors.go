package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSConfig controls cross-origin behaviour. Zero values are replaced with
// safe defaults inside the handler constructor.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

const corsDefaultMaxAge = 12 * time.Hour

var corsDefaultMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
var corsDefaultHeaders = []string{"Authorization", "Content-Type", "X-Request-ID"}

func CORS(cfg CORSConfig) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		allowed[origin] = struct{}{}
	}

	methods := cfg.AllowedMethods
	if len(methods) == 0 {
		methods = corsDefaultMethods
	}
	headers := cfg.AllowedHeaders
	if len(headers) == 0 {
		headers = corsDefaultHeaders
	}
	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = corsDefaultMaxAge
	}
	maxAgeSeconds := int(maxAge.Seconds())

	methodsHeader := strings.Join(methods, ", ")
	headersHeader := strings.Join(headers, ", ")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		originAllowed := origin != ""
		if originAllowed {
			if _, ok := allowed[origin]; !ok {
				originAllowed = false
			}
		}

		if originAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", methodsHeader)
			c.Header("Access-Control-Allow-Headers", headersHeader)
			c.Header("Access-Control-Max-Age", strconv.Itoa(maxAgeSeconds))
			if cfg.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		if c.Request.Method == http.MethodOptions {
			if originAllowed {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}
		c.Next()
	}
}

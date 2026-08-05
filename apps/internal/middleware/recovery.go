package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/vort-ads/vort-ads-template/apps/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/response"
)

func Recovery(log zerolog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Error().Interface("panic", recovered).Str("request_id", GetRequestID(c)).Msg("request panic")
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Error(
			GetRequestID(c),
			apperrors.CodeInternalError,
			"internal server error",
		))
	})
}

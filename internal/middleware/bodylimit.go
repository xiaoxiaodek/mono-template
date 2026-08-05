package middleware

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/vort-ads/vort-ads-template/internal/platform/apperrors"
	"github.com/vort-ads/vort-ads-template/internal/platform/response"
)

// ErrBodyTooLarge is returned when a request body exceeds the configured limit.
// Handlers can use errors.Is to detect this and return HTTP 413.
var ErrBodyTooLarge = errors.New("request body too large")

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, response.Error(
				GetRequestID(c),
				apperrors.CodeValidationError,
				"request body too large",
			))
			return
		}
		// MaxBytesReader catches chunked or Content-Length-less bodies
		// that exceed the limit. The reader translates *http.MaxBytesError
		// into ErrBodyTooLarge so callers can detect it with errors.Is.
		c.Request.Body = &maxBytesReader{
			ReadCloser: http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes),
		}
		c.Next()
	}
}

type maxBytesReader struct {
	io.ReadCloser
}

func (r *maxBytesReader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return n, ErrBodyTooLarge
		}
	}
	return n, err
}

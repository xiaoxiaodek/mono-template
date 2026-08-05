package response

import "github.com/vort-ads/vort-ads-template/internal/platform/apperrors"

type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func OK(requestID string, data any) Envelope {
	return Envelope{
		Code:      "OK",
		Message:   "ok",
		Data:      data,
		RequestID: requestID,
	}
}

func Error(requestID string, code apperrors.Code, message string) Envelope {
	return Envelope{
		Code:      string(code),
		Message:   message,
		RequestID: requestID,
	}
}

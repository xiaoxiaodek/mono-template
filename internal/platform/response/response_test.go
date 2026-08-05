package response

import (
	"reflect"
	"testing"

	"github.com/vort-ads/vort-ads-template/internal/platform/apperrors"
)

func TestOKReturnsStableEnvelopeShape(t *testing.T) {
	data := map[string]string{"id": "u1"}

	env := OK("req-1", data)

	if env.Code != "OK" {
		t.Fatalf("Code = %q, want OK", env.Code)
	}
	if env.Message != "ok" {
		t.Fatalf("Message = %q, want ok", env.Message)
	}
	if env.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want req-1", env.RequestID)
	}
	if !reflect.DeepEqual(env.Data, data) {
		t.Fatalf("Data = %#v, want %#v", env.Data, data)
	}
}

func TestErrorAcceptsTypedAppErrorCode(t *testing.T) {
	env := Error("req-1", apperrors.CodeValidationError, "invalid input")

	if env.Code != string(apperrors.CodeValidationError) {
		t.Fatalf("Code = %q, want %q", env.Code, apperrors.CodeValidationError)
	}
	if env.Message != "invalid input" {
		t.Fatalf("Message = %q, want invalid input", env.Message)
	}
	if env.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want req-1", env.RequestID)
	}
}

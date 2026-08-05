package apperrors

import "testing"

func TestNewCarriesCodeAndPublicMessage(t *testing.T) {
	err := New(CodeUnauthorized, "login required", nil)

	if err.Code != CodeUnauthorized {
		t.Fatalf("Code = %q, want %q", err.Code, CodeUnauthorized)
	}
	if err.PublicMessage() != "login required" {
		t.Fatalf("PublicMessage() = %q, want login required", err.PublicMessage())
	}
}

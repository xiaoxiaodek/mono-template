package idgen

import (
	"regexp"
	"testing"
)

var prefixedIDPattern = regexp.MustCompile(`^req_[0-9a-f]{32}$`)
var rawIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestNewReturnsPrefixedHexID(t *testing.T) {
	id, err := New("req")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if !prefixedIDPattern.MatchString(id) {
		t.Fatalf("New returned %q, want req_ plus 32 lowercase hex characters", id)
	}
}

func TestNewWithEmptyPrefixReturnsRawHexID(t *testing.T) {
	id, err := New("")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if len(id) != 32 {
		t.Fatalf("len(id) = %d, want 32", len(id))
	}
	if !rawIDPattern.MatchString(id) {
		t.Fatalf("New returned %q, want 32 lowercase hex characters", id)
	}
}

func TestMustNewReturnsPrefixedHexID(t *testing.T) {
	id := MustNew("req")

	if !prefixedIDPattern.MatchString(id) {
		t.Fatalf("MustNew returned %q, want req_ plus 32 lowercase hex characters", id)
	}
}

package identity_test

import (
	"errors"
	"testing"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/biz/identity"
)

func TestNewUserNormalizesEmailAndAssignsRole(t *testing.T) {
	user, err := bizidentity.NewUser("usr_1", "  Admin@Example.COM ", "$2a$hash", []string{"admin"})
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if user.Email != "admin@example.com" {
		t.Fatalf("email = %q", user.Email)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "admin" {
		t.Fatalf("roles = %+v", user.Roles)
	}
}

func TestNewUserRejectsInvalidEmail(t *testing.T) {
	_, err := bizidentity.NewUser("usr_1", "bad-email", "$2a$hash", []string{"admin"})
	if !errors.Is(err, bizidentity.ErrInvalidEmail) {
		t.Fatalf("error = %v, want ErrInvalidEmail", err)
	}
}

func TestNewUserRequiresPasswordHash(t *testing.T) {
	_, err := bizidentity.NewUser("usr_1", "admin@example.com", "", nil)
	if !errors.Is(err, bizidentity.ErrPasswordHashRequired) {
		t.Fatalf("error = %v, want ErrPasswordHashRequired", err)
	}
}

func TestNewUserDefaultsToUserRole(t *testing.T) {
	user, err := bizidentity.NewUser("usr_1", "admin@example.com", "$2a$hash", nil)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "user" {
		t.Fatalf("roles = %+v", user.Roles)
	}
}

func TestNewUserAssignsAdminOnlyWhenExplicit(t *testing.T) {
	user, err := bizidentity.NewUser("usr_1", "admin@example.com", "$2a$hash", []string{"admin"})
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "admin" {
		t.Fatalf("roles = %+v", user.Roles)
	}
}

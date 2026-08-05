package memory_test

import (
	"context"
	"errors"
	"testing"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/control-api/internal/biz/identity"
	"github.com/vort-ads/vort-ads-template/apps/control-api/internal/data/identity/memory"
)

func TestUserRepositoryRejectsDuplicateEmail(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()
	first, _ := bizidentity.NewUser("usr_1", "admin@example.com", "hash", nil)
	second, _ := bizidentity.NewUser("usr_2", "admin@example.com", "hash", nil)
	if err := repo.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, second); !errors.Is(err, bizidentity.ErrEmailTaken) {
		t.Fatalf("error = %v", err)
	}
}

func TestUserRepositoryReturnsMissingUser(t *testing.T) {
	repo := memory.NewUserRepository()
	if _, err := repo.FindByID(context.Background(), "missing"); !errors.Is(err, bizidentity.ErrUserNotFound) {
		t.Fatalf("error = %v", err)
	}
	if _, err := repo.FindByEmail(context.Background(), "missing@example.com"); !errors.Is(err, bizidentity.ErrUserNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestUserRepositoryClonesRoles(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()
	roles := []string{"admin"}
	user, _ := bizidentity.NewUser("usr_1", "admin@example.com", "hash", roles)
	if err := repo.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	roles[0] = "changed"
	user.Roles[0] = "changed"
	found, _ := repo.FindByID(ctx, user.ID)
	found.Roles[0] = "mutated"
	again, _ := repo.FindByID(ctx, user.ID)
	if again.Roles[0] != "admin" {
		t.Fatalf("roles = %+v", again.Roles)
	}
}

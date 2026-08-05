package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/biz/identity"
)

func TestPostgresUserRepositoryRoundTrip(t *testing.T) {
	pool := integrationPool(t)
	repo := NewUserRepository(pool)

	suffix := randomSuffix(t)
	user, err := bizidentity.NewUser("usr_"+suffix, "integration_"+suffix+"@example.com", "hash", []string{"admin"})
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID); cleanupErr != nil {
			t.Errorf("cleanup user: %v", cleanupErr)
		}
	})

	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("Create: %v", err)
	}

	byEmail, err := repo.FindByEmail(context.Background(), user.Email)
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	assertUsersEqual(t, byEmail, user)

	byID, err := repo.FindByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	assertUsersEqual(t, byID, user)
}

func TestPostgresUserRepositoryMapsDomainErrors(t *testing.T) {
	pool := integrationPool(t)
	repo := NewUserRepository(pool)

	suffix := randomSuffix(t)
	user, err := bizidentity.NewUser("usr_"+suffix, "duplicate_"+suffix+"@example.com", "hash", []string{"admin"})
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := pool.Exec(context.Background(), "DELETE FROM users WHERE email = $1", user.Email); cleanupErr != nil {
			t.Errorf("cleanup user: %v", cleanupErr)
		}
	})

	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	duplicate := user
	duplicate.ID = "usr_" + randomSuffix(t)
	if err := repo.Create(context.Background(), duplicate); !errors.Is(err, bizidentity.ErrEmailTaken) {
		t.Fatalf("duplicate Create error = %v, want %v", err, bizidentity.ErrEmailTaken)
	}

	if _, err := repo.FindByEmail(context.Background(), "missing_"+suffix+"@example.com"); !errors.Is(err, bizidentity.ErrUserNotFound) {
		t.Fatalf("FindByEmail missing error = %v, want %v", err, bizidentity.ErrUserNotFound)
	}
	if _, err := repo.FindByID(context.Background(), "missing_"+suffix); !errors.Is(err, bizidentity.ErrUserNotFound) {
		t.Fatalf("FindByID missing error = %v, want %v", err, bizidentity.ErrUserNotFound)
	}
}

func TestPostgresRefreshTokenStoreSavesAndRotatesAtomically(t *testing.T) {
	pool := integrationPool(t)
	users := NewUserRepository(pool)
	tokens := NewRefreshTokenStore(pool)

	suffix := randomSuffix(t)
	user, err := bizidentity.NewUser("usr_"+suffix, "tokens_"+suffix+"@example.com", "hash", nil)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID); cleanupErr != nil {
			t.Errorf("cleanup user: %v", cleanupErr)
		}
	})
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	ctx := context.Background()
	oldHash := "old_" + suffix
	newHash := "new_" + suffix
	blockedHash := "blocked_" + suffix
	expiresAt := time.Now().Add(time.Hour).UTC()
	if err := tokens.Save(ctx, user.ID, oldHash, expiresAt); err != nil {
		t.Fatalf("Save old token: %v", err)
	}
	if err := tokens.Save(ctx, user.ID, blockedHash, expiresAt); err != nil {
		t.Fatalf("Save blocking token: %v", err)
	}

	rotated, err := tokens.Rotate(ctx, oldHash, "usr_not_owner", newHash, expiresAt)
	if err != nil {
		t.Fatalf("Rotate with wrong owner: %v", err)
	}
	if rotated {
		t.Fatal("Rotate with wrong owner succeeded")
	}
	assertRefreshTokenOwner(t, pool, oldHash, user.ID)

	if _, err := tokens.Rotate(ctx, oldHash, user.ID, blockedHash, expiresAt); err == nil {
		t.Fatal("Rotate to duplicate hash succeeded")
	}
	assertRefreshTokenOwner(t, pool, oldHash, user.ID)

	rotated, err = tokens.Rotate(ctx, oldHash, user.ID, newHash, expiresAt)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if !rotated {
		t.Fatal("Rotate did not replace owned token")
	}
	assertRefreshTokenMissing(t, pool, oldHash)
	assertRefreshTokenOwner(t, pool, newHash, user.ID)
}

func TestPostgresRegistrationRollsBackUserWhenRefreshTokenInsertFails(t *testing.T) {
	pool := integrationPool(t)
	users := NewUserRepository(pool)
	tokens := NewRefreshTokenStore(pool)
	suffix := randomSuffix(t)
	expiresAt := time.Now().Add(time.Hour).UTC()

	existing, err := bizidentity.NewUser("usr_existing_"+suffix, "existing_"+suffix+"@example.com", "hash", nil)
	if err != nil {
		t.Fatalf("NewUser(existing): %v", err)
	}
	if err := users.Create(context.Background(), existing); err != nil {
		t.Fatalf("Create existing user: %v", err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", existing.ID); cleanupErr != nil {
			t.Errorf("cleanup existing user: %v", cleanupErr)
		}
	})
	duplicateHash := "duplicate_" + suffix
	if err := tokens.Save(context.Background(), existing.ID, duplicateHash, expiresAt); err != nil {
		t.Fatalf("Save duplicate token fixture: %v", err)
	}

	candidate, err := bizidentity.NewUser("usr_candidate_"+suffix, "candidate_"+suffix+"@example.com", "hash", nil)
	if err != nil {
		t.Fatalf("NewUser(candidate): %v", err)
	}
	if err := users.CreateUserWithRefreshToken(context.Background(), candidate, duplicateHash, expiresAt); err == nil {
		t.Fatal("atomic registration with duplicate token hash succeeded")
	}
	if _, err := users.FindByID(context.Background(), candidate.ID); !errors.Is(err, bizidentity.ErrUserNotFound) {
		t.Fatalf("candidate user after failed registration = %v, want %v", err, bizidentity.ErrUserNotFound)
	}
}

func TestPostgresRefreshTokenStoreConcurrentRotateHasSingleWinner(t *testing.T) {
	pool := integrationPool(t)
	users := NewUserRepository(pool)
	tokens := NewRefreshTokenStore(pool)
	suffix := randomSuffix(t)
	user, err := bizidentity.NewUser("usr_concurrent_"+suffix, "concurrent_"+suffix+"@example.com", "hash", nil)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID); cleanupErr != nil {
			t.Errorf("cleanup user: %v", cleanupErr)
		}
	})

	oldHash := "concurrent_old_" + suffix
	expiresAt := time.Now().Add(time.Hour).UTC()
	if err := tokens.Save(context.Background(), user.ID, oldHash, expiresAt); err != nil {
		t.Fatalf("Save old token: %v", err)
	}

	const attempts = 8
	start := make(chan struct{})
	var successes atomic.Int32
	var unexpected atomic.Int32
	var wg sync.WaitGroup
	for attempt := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			rotated, rotateErr := tokens.Rotate(
				context.Background(),
				oldHash,
				user.ID,
				"concurrent_new_"+suffix+"_"+strconv.Itoa(attempt),
				expiresAt,
			)
			if rotateErr != nil {
				unexpected.Add(1)
				return
			}
			if rotated {
				successes.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("successful rotations = %d, want 1", got)
	}
	if got := unexpected.Load(); got != 0 {
		t.Fatalf("unexpected rotation errors = %d, want 0", got)
	}
}

func TestPostgresRefreshTokenStoreRejectsExpiredToken(t *testing.T) {
	pool := integrationPool(t)
	users := NewUserRepository(pool)
	tokens := NewRefreshTokenStore(pool)
	suffix := randomSuffix(t)
	user, err := bizidentity.NewUser("usr_expired_"+suffix, "expired_"+suffix+"@example.com", "hash", nil)
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID); cleanupErr != nil {
			t.Errorf("cleanup user: %v", cleanupErr)
		}
	})

	oldHash := "expired_old_" + suffix
	if err := tokens.Save(context.Background(), user.ID, oldHash, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("Save expired token fixture: %v", err)
	}
	rotated, err := tokens.Rotate(context.Background(), oldHash, user.ID, "expired_new_"+suffix, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Rotate expired token: %v", err)
	}
	if rotated {
		t.Fatal("expired token rotation succeeded")
	}
}

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping integration database: %v", err)
	}
	return pool
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(value[:])
}

func assertUsersEqual(t *testing.T, got bizidentity.User, want bizidentity.User) {
	t.Helper()
	if got.ID != want.ID || got.Email != want.Email || got.PasswordHash != want.PasswordHash {
		t.Fatalf("user = %#v, want identity fields from %#v", got, want)
	}
	if len(got.Roles) != 1 || got.Roles[0] != "admin" {
		t.Fatalf("roles = %v, want [admin]", got.Roles)
	}
	if timestampsDiffer(got.CreatedAt, want.CreatedAt) || timestampsDiffer(got.UpdatedAt, want.UpdatedAt) {
		t.Fatalf("timestamps = (%v, %v), want (%v, %v)", got.CreatedAt, got.UpdatedAt, want.CreatedAt, want.UpdatedAt)
	}
}

func timestampsDiffer(left time.Time, right time.Time) bool {
	delta := left.Sub(right)
	if delta < 0 {
		delta = -delta
	}
	return delta >= time.Millisecond
}

func assertRefreshTokenOwner(t *testing.T, pool *pgxpool.Pool, tokenHash, wantUserID string) {
	t.Helper()
	var userID string
	if err := pool.QueryRow(context.Background(), "SELECT user_id FROM refresh_tokens WHERE token_hash = $1", tokenHash).Scan(&userID); err != nil {
		t.Fatalf("query refresh token %q: %v", tokenHash, err)
	}
	if userID != wantUserID {
		t.Fatalf("refresh token %q owner = %q, want %q", tokenHash, userID, wantUserID)
	}
}

func assertRefreshTokenMissing(t *testing.T, pool *pgxpool.Pool, tokenHash string) {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(context.Background(), "SELECT EXISTS (SELECT 1 FROM refresh_tokens WHERE token_hash = $1)", tokenHash).Scan(&exists); err != nil {
		t.Fatalf("query refresh token %q: %v", tokenHash, err)
	}
	if exists {
		t.Fatalf("refresh token %q still exists", tokenHash)
	}
}

package identity_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/control-api/internal/biz/identity"
	identitydata "github.com/vort-ads/vort-ads-template/apps/control-api/internal/data/identity"
	"github.com/vort-ads/vort-ads-template/apps/control-api/internal/data/identity/memory"
	"github.com/vort-ads/vort-ads-template/apps/internal/platform/security"
)

var generated atomic.Int64

func testID(prefix string) (string, error) {
	return fmt.Sprintf("%s_%d", prefix, generated.Add(1)), nil
}

func newTestUsecase() (*bizidentity.Usecase, *memory.UserRepository, *memory.TokenStore) {
	repo := memory.NewUserRepository()
	tokens := memory.NewTokenStore()
	manager := identitydata.NewTokenManager(security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour))
	return bizidentity.NewUsecase(repo, tokens, security.BcryptPasswordHasher{Cost: 4}, manager, testID, nil), repo, tokens
}

func TestRegisterCreatesUserAndReturnsTokenPair(t *testing.T) {
	usecase, repo, tokens := newTestUsecase()
	out, err := usecase.Register(context.Background(), bizidentity.RegisterInput{Email: " Admin@Example.COM ", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	if out.User.Email != "admin@example.com" || len(out.User.Roles) != 1 || out.User.Roles[0] != "user" {
		t.Fatalf("user = %+v", out.User)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatal("expected token pair")
	}
	stored, err := repo.FindByID(context.Background(), out.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PasswordHash == "" || stored.PasswordHash == "password123" {
		t.Fatal("password was not securely hashed")
	}
	if ok, _ := tokens.Exists(context.Background(), out.RefreshToken); ok {
		t.Fatal("raw refresh token persisted")
	}
	if ok, _ := tokens.Exists(context.Background(), hashToken(out.RefreshToken)); !ok {
		t.Fatal("hashed refresh token missing")
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	usecase, _, _ := newTestUsecase()
	ctx := context.Background()
	if _, err := usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"}); err != nil {
		t.Fatal(err)
	}
	if _, err := usecase.Register(ctx, bizidentity.RegisterInput{Email: " ADMIN@example.com ", Password: "password456"}); !errors.Is(err, bizidentity.ErrEmailTaken) {
		t.Fatalf("error = %v", err)
	}
}

func TestRegisterRemovesUserWhenRefreshTokenSaveFails(t *testing.T) {
	repo := memory.NewUserRepository()
	saveErr := errors.New("save refresh token")
	usecase := bizidentity.NewUsecase(repo, failingSaveTokenStore{saveErr}, security.BcryptPasswordHasher{Cost: 4}, testManager(), testID, nil)
	_, err := usecase.Register(context.Background(), bizidentity.RegisterInput{Email: "rollback@example.com", Password: "password123"})
	if !errors.Is(err, saveErr) {
		t.Fatalf("error = %v", err)
	}
	if _, err := repo.FindByEmail(context.Background(), "rollback@example.com"); !errors.Is(err, bizidentity.ErrUserNotFound) {
		t.Fatalf("user remains: %v", err)
	}
}

func TestRegisterJoinsRefreshSaveAndCompensationDeleteFailures(t *testing.T) {
	saveErr := errors.New("save refresh token")
	deleteErr := errors.New("delete registered user")
	repo := &deleteFailingRepository{UserRepository: memory.NewUserRepository(), err: deleteErr}
	usecase := bizidentity.NewUsecase(repo, failingSaveTokenStore{saveErr}, security.BcryptPasswordHasher{Cost: 4}, testManager(), testID, nil)

	_, err := usecase.Register(context.Background(), bizidentity.RegisterInput{Email: "rollback-errors@example.com", Password: "password123"})
	if !errors.Is(err, saveErr) || !errors.Is(err, deleteErr) {
		t.Fatalf("error = %v, want joined save and delete failures", err)
	}
	if repo.deleteContextErr != nil {
		t.Fatalf("rollback context error = %v", repo.deleteContextErr)
	}
}

func TestRegisterUsesAtomicUnitOfWorkWhenAvailable(t *testing.T) {
	repo := memory.NewUserRepository()
	tokens := &recordingTokenStore{}
	uowErr := errors.New("atomic registration failed")
	uow := &recordingUOW{err: uowErr}
	usecase := bizidentity.NewUsecase(repo, tokens, security.BcryptPasswordHasher{Cost: 4}, testManager(), testID, uow)
	_, err := usecase.Register(context.Background(), bizidentity.RegisterInput{Email: "atomic@example.com", Password: "password123"})
	if !errors.Is(err, uowErr) || uow.calls != 1 || tokens.saveCalls != 0 {
		t.Fatalf("err=%v uow=%d saves=%d", err, uow.calls, tokens.saveCalls)
	}
	if _, err := repo.FindByEmail(context.Background(), "atomic@example.com"); !errors.Is(err, bizidentity.ErrUserNotFound) {
		t.Fatalf("standalone create called: %v", err)
	}
}

func TestRegisterIDGeneratorFailurePersistsNoUser(t *testing.T) {
	repo := memory.NewUserRepository()
	idErr := errors.New("generate id")
	usecase := bizidentity.NewUsecase(repo, memory.NewTokenStore(), security.BcryptPasswordHasher{Cost: 4}, testManager(), func(string) (string, error) { return "", idErr }, nil)
	_, err := usecase.Register(context.Background(), bizidentity.RegisterInput{Email: "id@example.com", Password: "password123"})
	if !errors.Is(err, idErr) {
		t.Fatalf("error = %v", err)
	}
	if _, err := repo.FindByEmail(context.Background(), "id@example.com"); !errors.Is(err, bizidentity.ErrUserNotFound) {
		t.Fatalf("user persisted: %v", err)
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	usecase, _, _ := newTestUsecase()
	ctx := context.Background()
	_, _ = usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"})
	if _, err := usecase.Login(ctx, bizidentity.LoginInput{Email: "admin@example.com", Password: "wrong"}); !errors.Is(err, bizidentity.ErrInvalidCredentials) {
		t.Fatalf("error = %v", err)
	}
}

func TestLoginHidesWhetherUserExists(t *testing.T) {
	usecase, _, _ := newTestUsecase()
	if _, err := usecase.Login(context.Background(), bizidentity.LoginInput{Email: "missing@example.com", Password: "password123"}); !errors.Is(err, bizidentity.ErrInvalidCredentials) {
		t.Fatalf("error = %v", err)
	}
}

func TestLoginComparesPasswordWhenUserDoesNotExist(t *testing.T) {
	hasher := &recordingPasswordHasher{}
	usecase := bizidentity.NewUsecase(memory.NewUserRepository(), memory.NewTokenStore(), hasher, testManager(), testID, nil)
	_, err := usecase.Login(context.Background(), bizidentity.LoginInput{Email: "missing@example.com", Password: "password123"})
	if !errors.Is(err, bizidentity.ErrInvalidCredentials) || hasher.compareCalls != 1 {
		t.Fatalf("err=%v calls=%d", err, hasher.compareCalls)
	}
}

func TestRefreshRotatesRefreshToken(t *testing.T) {
	usecase, _, _ := newTestUsecase()
	ctx := context.Background()
	registered, _ := usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"})
	refreshed, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: registered.RefreshToken})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.RefreshToken == registered.RefreshToken {
		t.Fatal("refresh token not rotated")
	}
	if _, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: registered.RefreshToken}); !errors.Is(err, bizidentity.ErrInvalidCredentials) {
		t.Fatalf("old token error = %v", err)
	}
	if _, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: refreshed.RefreshToken}); err != nil {
		t.Fatalf("new token: %v", err)
	}
}

func TestRefreshHidesMissingTokenSubject(t *testing.T) {
	usecase, _, tokens := newTestUsecase()
	registered, _ := usecase.Register(context.Background(), bizidentity.RegisterInput{Email: "deleted@example.com", Password: "password123"})
	usecase = bizidentity.NewUsecase(memory.NewUserRepository(), tokens, security.BcryptPasswordHasher{Cost: 4}, testManager(), testID, nil)
	if _, err := usecase.Refresh(context.Background(), bizidentity.RefreshInput{RefreshToken: registered.RefreshToken}); !errors.Is(err, bizidentity.ErrInvalidCredentials) {
		t.Fatalf("error = %v", err)
	}
}

func TestConcurrentRefreshHasSingleWinner(t *testing.T) {
	usecase, _, _ := newTestUsecase()
	ctx := context.Background()
	registered, _ := usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"})
	start := make(chan struct{})
	var successes, unexpected atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: registered.RefreshToken})
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, bizidentity.ErrInvalidCredentials) {
				unexpected.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if successes.Load() != 1 || unexpected.Load() != 0 {
		t.Fatalf("successes=%d unexpected=%d", successes.Load(), unexpected.Load())
	}
}

func TestRefreshDoesNotRotateWhenCandidateGenerationFails(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserRepository()
	tokens := memory.NewTokenStore()
	manager := testManager()
	password := security.BcryptPasswordHasher{Cost: 4}
	usecase := bizidentity.NewUsecase(repo, tokens, password, manager, testID, nil)
	registered, _ := usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"})
	signErr := errors.New("sign access token")
	usecase = bizidentity.NewUsecase(repo, tokens, password, failingManager{TokenManager: manager, signErr: signErr}, testID, nil)
	if _, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: registered.RefreshToken}); !errors.Is(err, signErr) {
		t.Fatalf("error = %v", err)
	}
	usecase = bizidentity.NewUsecase(repo, tokens, password, manager, testID, nil)
	if _, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: registered.RefreshToken}); err != nil {
		t.Fatalf("old token invalidated: %v", err)
	}
}

func TestRefreshDoesNotRotateWhenCandidateVerificationFails(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserRepository()
	tokens := memory.NewTokenStore()
	manager := testManager()
	password := security.BcryptPasswordHasher{Cost: 4}
	usecase := bizidentity.NewUsecase(repo, tokens, password, manager, testID, nil)
	registered, _ := usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"})
	verifyErr := errors.New("verify access token")
	usecase = bizidentity.NewUsecase(repo, tokens, password, verifyFailingManager{TokenManager: manager, verifyErr: verifyErr}, testID, nil)
	if _, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: registered.RefreshToken}); !errors.Is(err, verifyErr) {
		t.Fatalf("error = %v", err)
	}
	usecase = bizidentity.NewUsecase(repo, tokens, password, manager, testID, nil)
	if _, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: registered.RefreshToken}); err != nil {
		t.Fatalf("old token invalidated: %v", err)
	}
}

func TestRefreshDoesNotSaveReplacementWhenRotateFails(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserRepository()
	manager := testManager()
	password := security.BcryptPasswordHasher{Cost: 4}
	usecase := bizidentity.NewUsecase(repo, memory.NewTokenStore(), password, manager, testID, nil)
	registered, _ := usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"})
	rotateErr := errors.New("rotate refresh token")
	tokens := &rotationRecordingStore{rotateErr: rotateErr}
	usecase = bizidentity.NewUsecase(repo, tokens, password, manager, testID, nil)
	if _, err := usecase.Refresh(ctx, bizidentity.RefreshInput{RefreshToken: registered.RefreshToken}); !errors.Is(err, rotateErr) {
		t.Fatalf("error = %v", err)
	}
	if tokens.rotateCalls != 1 || tokens.saveCalls != 0 {
		t.Fatalf("rotate=%d save=%d", tokens.rotateCalls, tokens.saveCalls)
	}
}

func TestMeReturnsPublicUser(t *testing.T) {
	usecase, _, _ := newTestUsecase()
	ctx := context.Background()
	registered, _ := usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"})
	out, err := usecase.Me(ctx, registered.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != registered.User.ID || out.Email != registered.User.Email {
		t.Fatalf("user = %+v", out)
	}
}

func TestUsecasePropagatesCanceledContext(t *testing.T) {
	usecase, _, _ := newTestUsecase()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := usecase.Register(ctx, bizidentity.RegisterInput{Email: "admin@example.com", Password: "password123"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func testManager() bizidentity.TokenManager {
	return identitydata.NewTokenManager(security.NewJWTManager("test-secret-with-enough-length", time.Minute, time.Hour))
}

func hashToken(token string) string   { return fmt.Sprintf("%x", sha256Sum(token)) }
func sha256Sum(value string) [32]byte { return sha256.Sum256([]byte(value)) }

type failingSaveTokenStore struct{ err error }

func (s failingSaveTokenStore) Save(context.Context, string, string, time.Time) error { return s.err }
func (failingSaveTokenStore) Rotate(context.Context, string, string, string, time.Time) (bool, error) {
	return false, nil
}

type deleteFailingRepository struct {
	*memory.UserRepository
	err              error
	deleteContextErr error
}

func (r *deleteFailingRepository) Delete(ctx context.Context, _ string) error {
	r.deleteContextErr = ctx.Err()
	return r.err
}

type recordingTokenStore struct{ saveCalls int }

func (s *recordingTokenStore) Save(context.Context, string, string, time.Time) error {
	s.saveCalls++
	return nil
}
func (*recordingTokenStore) Rotate(context.Context, string, string, string, time.Time) (bool, error) {
	return false, nil
}

type recordingUOW struct {
	calls int
	err   error
}

func (u *recordingUOW) CreateUserWithRefreshToken(context.Context, bizidentity.User, string, time.Time) error {
	u.calls++
	return u.err
}

type recordingPasswordHasher struct{ compareCalls int }

func (*recordingPasswordHasher) Hash(string) (string, error)   { return "unused", nil }
func (h *recordingPasswordHasher) Compare(string, string) bool { h.compareCalls++; return false }

type failingManager struct {
	bizidentity.TokenManager
	signErr error
}

func (m failingManager) SignAccessToken(bizidentity.Principal) (string, error) { return "", m.signErr }

type verifyFailingManager struct {
	bizidentity.TokenManager
	verifyErr error
}

func (m verifyFailingManager) VerifyAccessToken(string) (bizidentity.TokenClaims, error) {
	return bizidentity.TokenClaims{}, m.verifyErr
}

type rotationRecordingStore struct {
	rotateErr              error
	rotateCalls, saveCalls int
}

func (s *rotationRecordingStore) Save(context.Context, string, string, time.Time) error {
	s.saveCalls++
	return nil
}
func (s *rotationRecordingStore) Rotate(context.Context, string, string, string, time.Time) (bool, error) {
	s.rotateCalls++
	return false, s.rotateErr
}

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/control-api/internal/biz/identity"
)

const userColumns = `
	u.id,
	u.email,
	u.password_hash,
	u.created_at,
	u.updated_at,
	COALESCE(
		(
			SELECT array_agg(r.name ORDER BY r.name)
			FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = u.id
		),
		ARRAY[]::TEXT[]
	)`

type UserRepository struct {
	pool *pgxpool.Pool
}

type RefreshTokenStore struct {
	pool *pgxpool.Pool
}

var _ bizidentity.UserRepository = (*UserRepository)(nil)
var _ bizidentity.RefreshTokenStore = (*RefreshTokenStore)(nil)
var _ bizidentity.RegistrationUnitOfWork = (*UserRepository)(nil)

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func NewRefreshTokenStore(pool *pgxpool.Pool) *RefreshTokenStore {
	return &RefreshTokenStore{pool: pool}
}

func (s *RefreshTokenStore) Save(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, tokenHash, userID, expiresAt); err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (s *RefreshTokenStore) Rotate(
	ctx context.Context,
	oldHash string,
	userID string,
	newHash string,
	expiresAt time.Time,
) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin refresh token rotation: %w", err)
	}
	defer rollbackSafely(ctx, tx)

	tag, err := tx.Exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE token_hash = $1
		  AND user_id = $2
		  AND expires_at > now()
	`, oldHash, userID)
	if err != nil {
		return false, fmt.Errorf("consume refresh token: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return false, nil
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, newHash, userID, expiresAt); err != nil {
		return false, fmt.Errorf("save rotated refresh token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit refresh token rotation: %w", err)
	}
	return true, nil
}

func (r *UserRepository) Create(ctx context.Context, user bizidentity.User) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create user transaction: %w", err)
	}
	defer rollbackSafely(ctx, tx)

	if err := createUser(ctx, tx, user); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit create user transaction: %w", err)
	}
	return nil
}

func (r *UserRepository) CreateUserWithRefreshToken(
	ctx context.Context,
	user bizidentity.User,
	tokenHash string,
	expiresAt time.Time,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin registration transaction: %w", err)
	}
	defer rollbackSafely(ctx, tx)

	if err := createUser(ctx, tx, user); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, tokenHash, user.ID, expiresAt); err != nil {
		return fmt.Errorf("save registration refresh token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit registration transaction: %w", err)
	}
	return nil
}

func createUser(ctx context.Context, tx pgx.Tx, user bizidentity.User) error {
	_, err := tx.Exec(ctx, `
			INSERT INTO users (id, email, password_hash, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5)
		`, user.ID, user.Email, user.PasswordHash, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return mapUserInsertError(err)
	}

	seenRoles := make(map[string]struct{}, len(user.Roles))
	for _, role := range user.Roles {
		if _, seen := seenRoles[role]; seen {
			continue
		}
		seenRoles[role] = struct{}{}

		tag, assignErr := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id
			FROM roles
			WHERE name = $2
		`, user.ID, role)
		if assignErr != nil {
			return fmt.Errorf("assign role %q: %w", role, assignErr)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("assign role %q: role not found", role)
		}
	}

	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (bizidentity.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users u
		WHERE u.email = $1
	`, email))
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (bizidentity.User, error) {
	return scanUser(r.pool.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users u
		WHERE u.id = $1
	`, id))
}

func scanUser(row pgx.Row) (bizidentity.User, error) {
	var user bizidentity.User
	if err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.Roles,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bizidentity.User{}, bizidentity.ErrUserNotFound
		}
		return bizidentity.User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}

func mapUserInsertError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" && postgresError.ConstraintName == "users_email_key" {
		return bizidentity.ErrEmailTaken
	}
	return fmt.Errorf("insert user: %w", err)
}

func rollbackSafely(ctx context.Context, tx pgx.Tx) {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(rollbackCtx)
}

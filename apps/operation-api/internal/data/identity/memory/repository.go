package memory

import (
	"context"
	"sync"

	bizidentity "github.com/vort-ads/vort-ads-template/apps/operation-api/internal/biz/identity"
)

var _ bizidentity.UserRepository = (*UserRepository)(nil)

type UserRepository struct {
	mu      sync.RWMutex
	byID    map[string]bizidentity.User
	byEmail map[string]string
}

func NewUserRepository() *UserRepository {
	return &UserRepository{byID: make(map[string]bizidentity.User), byEmail: make(map[string]string)}
}
func (r *UserRepository) Create(ctx context.Context, user bizidentity.User) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byEmail[user.Email]; exists {
		return bizidentity.ErrEmailTaken
	}
	user = cloneUser(user)
	r.byID[user.ID] = user
	r.byEmail[user.Email] = user.ID
	return nil
}
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	user, exists := r.byID[id]
	if !exists {
		return nil
	}
	delete(r.byID, id)
	delete(r.byEmail, user.Email)
	return nil
}
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (bizidentity.User, error) {
	if err := ctx.Err(); err != nil {
		return bizidentity.User{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, exists := r.byEmail[email]
	if !exists {
		return bizidentity.User{}, bizidentity.ErrUserNotFound
	}
	return cloneUser(r.byID[id]), nil
}
func (r *UserRepository) FindByID(ctx context.Context, id string) (bizidentity.User, error) {
	if err := ctx.Err(); err != nil {
		return bizidentity.User{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, exists := r.byID[id]
	if !exists {
		return bizidentity.User{}, bizidentity.ErrUserNotFound
	}
	return cloneUser(user), nil
}
func cloneUser(user bizidentity.User) bizidentity.User {
	user.Roles = append([]string(nil), user.Roles...)
	return user
}

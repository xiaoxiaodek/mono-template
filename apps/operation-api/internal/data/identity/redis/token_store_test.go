package redis

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
)

type fakeClient struct {
	mu     sync.Mutex
	values map[string]string
	ttls   map[string]time.Duration
	script string
}

func newFakeClient() *fakeClient {
	return &fakeClient{values: make(map[string]string), ttls: make(map[string]time.Duration)}
}

func (f *fakeClient) Set(ctx context.Context, key string, value any, expiration time.Duration) *redisclient.StatusCmd {
	f.mu.Lock()
	f.values[key] = value.(string)
	f.ttls[key] = expiration
	f.mu.Unlock()
	command := redisclient.NewStatusCmd(ctx)
	command.SetVal("OK")
	return command
}

func (f *fakeClient) Eval(ctx context.Context, script string, keys []string, args ...any) *redisclient.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.script = script
	result := int64(0)
	if len(keys) == 2 && len(args) == 2 {
		userID, _ := args[0].(string)
		ttlMilliseconds, _ := args[1].(int64)
		if f.values[keys[0]] == userID && ttlMilliseconds > 0 {
			delete(f.values, keys[0])
			delete(f.ttls, keys[0])
			f.values[keys[1]] = userID
			f.ttls[keys[1]] = time.Duration(ttlMilliseconds) * time.Millisecond
			result = 1
		}
	}
	command := redisclient.NewCmd(ctx)
	command.SetVal(result)
	return command
}

func TestTokenStoreSaveUsesHashKeyAndBoundedTTL(t *testing.T) {
	client := newFakeClient()
	store := NewTokenStore(client)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	if err := store.Save(context.Background(), "usr_1", "hash_1", now.Add(time.Hour)); err != nil {
		t.Fatalf("save: %v", err)
	}

	if got := client.values["identity:refresh:hash_1"]; got != "usr_1" {
		t.Fatalf("stored user = %q, want usr_1", got)
	}
	if got := client.ttls["identity:refresh:hash_1"]; got != time.Hour {
		t.Fatalf("ttl = %v, want %v", got, time.Hour)
	}
}

func TestTokenStoreRotateAtomicallyReplacesMatchingToken(t *testing.T) {
	client := newFakeClient()
	store := NewTokenStore(client)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	client.values["identity:refresh:old"] = "usr_1"

	rotated, err := store.Rotate(context.Background(), "old", "usr_1", "new", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if !rotated {
		t.Fatal("token was not rotated")
	}
	if _, exists := client.values["identity:refresh:old"]; exists {
		t.Fatal("old token still exists")
	}
	if got := client.values["identity:refresh:new"]; got != "usr_1" {
		t.Fatalf("new token user = %q, want usr_1", got)
	}
	if !strings.Contains(client.script, "redis.call('GET'") ||
		!strings.Contains(client.script, "redis.call('DEL'") ||
		!strings.Contains(client.script, "redis.call('SET'") {
		t.Fatalf("rotation is not a Redis Lua CAS: %s", client.script)
	}
}

func TestTokenStoreRotatePreservesTokenOwnedByAnotherUser(t *testing.T) {
	client := newFakeClient()
	store := NewTokenStore(client)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	client.values["identity:refresh:old"] = "usr_2"

	rotated, err := store.Rotate(context.Background(), "old", "usr_1", "new", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated {
		t.Fatal("rotated token owned by another user")
	}
	if got := client.values["identity:refresh:old"]; got != "usr_2" {
		t.Fatalf("old token user = %q, want usr_2", got)
	}
	if _, exists := client.values["identity:refresh:new"]; exists {
		t.Fatal("new token was created")
	}
}

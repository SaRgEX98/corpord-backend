package sso

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisStateBrowserProviderReplayAndExpiry(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, Password: os.Getenv("TEST_REDIS_PASSWORD")})
	t.Cleanup(func() { client.Close() })
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	store := NewRedisStateStore(client)
	state := fmt.Sprintf("state-%d", time.Now().UnixNano())
	t.Cleanup(func() { client.Del(ctx, "oauth:state:"+stateHash(state)) })
	if err := store.Save(ctx, state, "google", "browser-a", "verifier", time.Minute); err != nil {
		t.Fatal(err)
	}
	if ttl, err := client.PTTL(ctx, "oauth:state:"+stateHash(state)).Result(); err != nil || ttl <= 0 || ttl > time.Minute {
		t.Fatalf("state TTL missing or incorrect: ttl=%v err=%v", ttl, err)
	}
	for _, input := range []struct{ state, provider, browser string }{
		{"", "google", "browser-a"}, {state, "yandex", "browser-a"}, {state, "google", "browser-b"}, {state + "bad", "google", "browser-a"},
	} {
		if _, err := store.Consume(ctx, input.state, input.provider, input.browser); !errors.Is(err, ErrInvalidState) {
			t.Fatalf("invalid callback accepted: %v", err)
		}
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			verifier, err := store.Consume(ctx, state, "google", "browser-a")
			if err == nil {
				if verifier != "verifier" {
					t.Errorf("unexpected verifier %q", verifier)
				}
				successes.Add(1)
			} else if !errors.Is(err, ErrInvalidState) {
				t.Errorf("consume: %v", err)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent callbacks accepted %d times", successes.Load())
	}
	if _, err := store.Consume(ctx, state, "google", "browser-a"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("state replay accepted: %v", err)
	}
	if err := store.Save(ctx, state, "google", "browser-a", "verifier", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := client.PExpireAt(ctx, "oauth:state:"+stateHash(state), time.Now().Add(-time.Second)).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Consume(ctx, state, "google", "browser-a"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("expired state accepted: %v", err)
	}
}

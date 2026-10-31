package sso

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const StateTTL = 5 * time.Minute

var ErrInvalidState = errors.New("invalid or expired OAuth state")

// StateStore atomically consumes a state only for its original browser and provider.
// Implementations must expire records and never return a consumed record twice.
type StateStore interface {
	Save(ctx context.Context, state, provider, browserBinding, codeVerifier string, ttl time.Duration) error
	Consume(ctx context.Context, state, provider, browserBinding string) (codeVerifier string, err error)
}

type redisStateStore struct {
	client redis.UniversalClient
}

func NewRedisStateStore(client redis.UniversalClient) StateStore {
	return &redisStateStore{client: client}
}

type stateRecord struct {
	Provider    string `json:"provider"`
	BrowserHash string `json:"browser_hash"`
	Verifier    string `json:"verifier"`
}

func stateHash(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func (s *redisStateStore) Save(ctx context.Context, state, provider, browserBinding, codeVerifier string, ttl time.Duration) error {
	if state == "" || provider == "" || browserBinding == "" || codeVerifier == "" || ttl <= 0 {
		return ErrInvalidState
	}
	payload, err := json.Marshal(stateRecord{Provider: provider, BrowserHash: stateHash(browserBinding), Verifier: codeVerifier})
	if err != nil {
		return err
	}
	ok, err := s.client.SetNX(ctx, "oauth:state:"+stateHash(state), payload, ttl).Result()
	if err != nil {
		return fmt.Errorf("save OAuth state: %w", err)
	}
	if !ok {
		return errors.New("OAuth state already exists")
	}
	return nil
}

var consumeStateScript = redis.NewScript(`
local record = redis.call('GET', KEYS[1])
if not record then return false end
local data = cjson.decode(record)
if data.provider ~= ARGV[1] or data.browser_hash ~= ARGV[2] then return false end
redis.call('DEL', KEYS[1])
return data.verifier
`)

func (s *redisStateStore) Consume(ctx context.Context, state, provider, browserBinding string) (string, error) {
	if state == "" || provider == "" || browserBinding == "" || len(state) > 128 || len(browserBinding) > 128 {
		return "", ErrInvalidState
	}
	verifier, err := consumeStateScript.Run(ctx, s.client, []string{"oauth:state:" + stateHash(state)}, provider, stateHash(browserBinding)).Text()
	if errors.Is(err, redis.Nil) {
		return "", ErrInvalidState
	}
	if err != nil {
		return "", fmt.Errorf("consume OAuth state: %w", err)
	}
	if verifier == "" {
		return "", ErrInvalidState
	}
	return verifier, nil
}

//go:build integration

package redis

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/uzzz/httpcache"

	"github.com/go-redis/redis/v8"
)

func TestRedis(t *testing.T) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		t.Fatal("REDIS_ADDR is empty")
	}

	store, err := NewStore(WithRedisOptions(&redis.Options{Addr: redisAddr}))
	if err != nil {
		t.Fatal("unexpected error", err)
	}

	data := []byte("data")

	if err := store.Set(context.Background(), uint64(1), data, 1*time.Minute); err != nil {
		t.Error("unexpected error", err)
	}

	fetchedData, err := store.Get(context.Background(), uint64(1))
	if err != nil {
		t.Error("unexpected error", err)
	}
	if !reflect.DeepEqual(data, fetchedData) {
		t.Errorf("expected to return '%s', got '%s'", string(data), string(fetchedData))
	}

	if _, err := store.Get(context.Background(), uint64(2)); err != httpcache.ErrNoEntry {
		t.Errorf("expected httpcache.ErrNoEntry, got %s", err)
	}
}

package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/uzzz/httpcache"

	"github.com/go-redis/redis/v8"
)

// Option is used to set Store settings.
type Option func(o *Options) error

type Options struct {
	client       *redis.Client
	redisOptions *redis.Options
}

// WithClient sets the redis client.
func WithClient(client *redis.Client) Option {
	return func(o *Options) error {
		if client == nil {
			return errors.New("client can't be nil")
		}

		o.client = client

		return nil
	}
}

// WithRedisOptions sets the redis client options.
func WithRedisOptions(redisOptions *redis.Options) Option {
	return func(o *Options) error {
		if redisOptions == nil {
			return errors.New("redisOptions can't be nil")
		}

		o.redisOptions = redisOptions

		return nil
	}
}

var defaultOptions = Options{}

type Store struct {
	client *redis.Client
}

// NewStore initializes redis store.
func NewStore(opts ...Option) (*Store, error) {
	options := defaultOptions

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	if options.client == nil && options.redisOptions == nil {
		return nil, errors.New("either client or redisOptions should be set")
	}

	client := options.client
	if client == nil {
		client = redis.NewClient(options.redisOptions)
	}

	return &Store{
		client: client,
	}, nil
}

// Get data from store
func (s *Store) Get(key uint64) ([]byte, error) {
	cmd := s.client.Get(context.TODO(), keyToString(key))
	result, err := cmd.Bytes()
	if err == redis.Nil {
		return nil, httpcache.ErrNoEntry
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get: %v", err)
	}
	return result, nil
}

func (s *Store) Set(key uint64, data []byte, ttl time.Duration) error {
	if err := s.client.Set(context.TODO(), keyToString(key), data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set: %v", err)
	}
	return nil
}

func keyToString(key uint64) string {
	return strconv.FormatUint(key, 10)
}

var _ httpcache.Store = (*Store)(nil)

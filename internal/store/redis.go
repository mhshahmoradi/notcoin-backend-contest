package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"notcoin_contest/internal/models"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	Client *redis.Client
}

func NewRedisClient(addr, password string, db int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Ping(ctx).Result(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return client, nil
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{Client: client}
}

func (s *RedisStore) Close() error {
	if s.Client != nil {
		return s.Client.Close()
	}
	return nil
}

func (s *RedisStore) StoreCheckoutCode(ctx context.Context, attempt *models.CheckoutAttempt, ttl time.Duration) error {
	key := fmt.Sprintf("checkout_code:%s", attempt.ID)

	attemptJSON, err := json.Marshal(attempt)
	if err != nil {
		return fmt.Errorf("failed to marshal checkout attempt: %w", err)
	}

	err = s.Client.Set(ctx, key, attemptJSON, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set checkout code in redis: %w", err)
	}
	return nil
}

func (s *RedisStore) GetCheckoutAttempt(ctx context.Context, code string) (*models.CheckoutAttempt, error) {
	key := fmt.Sprintf("checkout_code:%s", code)
	val, err := s.Client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get checkout code from redis: %w", err)
	}

	var attempt models.CheckoutAttempt
	if err := json.Unmarshal([]byte(val), &attempt); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkout attempt from redis: %w", err)
	}
	return &attempt, nil
}

func (s *RedisStore) DeleteCheckoutCode(ctx context.Context, code string) error {
	key := fmt.Sprintf("checkout_code:%s", code)
	err := s.Client.Del(ctx, key).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return fmt.Errorf("failed to delete checkout code from redis: %w", err)
	}
	return nil
}

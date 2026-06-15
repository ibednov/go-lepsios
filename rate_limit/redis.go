package rate_limit

import (
	"context"
	"time"

	redisClient "github.com/redis/go-redis/v9"
)

// Service provides Redis-backed rate limiting.
type Service struct {
	client *redisClient.Client
}

// NewService creates a rate limit service.
func NewService(client *redisClient.Client) *Service {
	return &Service{client: client}
}

// LockWithAttempt limits attempts within ttl using INCR + EXPIRE.
func (s *Service) LockWithAttempt(ctx context.Context, key string, attemptCount int, ttl time.Duration) error {
	count, err := s.client.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	if count == 1 {
		if err := s.client.Expire(ctx, key, ttl).Err(); err != nil {
			return err
		}
	}

	if count > int64(attemptCount) {
		return ErrTooManyAttempts
	}

	return nil
}

// GetRemaining returns remaining attempts for key.
func (s *Service) GetRemaining(ctx context.Context, key string, limit int) (int, error) {
	count, err := s.client.Get(ctx, key).Int()
	if err == redisClient.Nil {
		return limit, nil
	}
	if err != nil {
		return 0, err
	}

	remaining := limit - count
	if remaining < 0 {
		return 0, nil
	}
	return remaining, nil
}

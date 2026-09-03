package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ibednov/go-lepsios/telegram/session"
	redisclient "github.com/redis/go-redis/v9"
)

const keyPrefix = "telegram:session:"

type SessionStore struct {
	client *redisclient.Client
	ttl    time.Duration
}

func NewSessionStore(client *redisclient.Client, ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &SessionStore{client: client, ttl: ttl}
}

func (s *SessionStore) key(chatID int64) string {
	return fmt.Sprintf("%s%d", keyPrefix, chatID)
}

func (s *SessionStore) Get(ctx context.Context, chatID int64) (*session.Session, error) {
	raw, err := s.client.Get(ctx, s.key(chatID)).Bytes()
	if err == redisclient.Nil {
		return &session.Session{ChatID: chatID}, nil
	}
	if err != nil {
		return nil, err
	}
	var sess session.Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return nil, err
	}
	sess.ChatID = chatID
	return &sess, nil
}

func (s *SessionStore) Save(ctx context.Context, sess *session.Session) error {
	if sess == nil {
		return fmt.Errorf("session is nil")
	}
	sess.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.key(sess.ChatID), raw, s.ttl).Err()
}

func (s *SessionStore) Delete(ctx context.Context, chatID int64) error {
	return s.client.Del(ctx, s.key(chatID)).Err()
}

package ws

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Bus fans out envelopes across API instances via Redis Pub/Sub.
// channelPrefix example: "wishimi:chat:user:" → channels wishimi:chat:user:{uuid}
type Bus struct {
	rdb    *redis.Client
	hub    *Hub
	prefix string
}

func NewBus(rdb *redis.Client, hub *Hub, channelPrefix string) *Bus {
	return &Bus{rdb: rdb, hub: hub, prefix: channelPrefix}
}

func (b *Bus) UserChannel(userID string) string {
	return b.prefix + userID
}

func (b *Bus) pattern() string {
	return b.prefix + "*"
}

func (b *Bus) PublishUser(ctx context.Context, userID string, envelope []byte) error {
	if b == nil || userID == "" || len(envelope) == 0 {
		return nil
	}
	if b.rdb == nil {
		if b.hub != nil {
			b.hub.Deliver(userID, envelope)
		}
		return nil
	}
	if err := b.rdb.Publish(ctx, b.UserChannel(userID), envelope).Err(); err != nil {
		return fmt.Errorf("ws bus publish: %w", err)
	}
	return nil
}

func (b *Bus) Run(ctx context.Context) {
	if b == nil || b.rdb == nil || b.hub == nil || b.prefix == "" {
		return
	}
	pubsub := b.rdb.PSubscribe(ctx, b.pattern())
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg == nil {
				continue
			}
			userID := strings.TrimPrefix(msg.Channel, b.prefix)
			if userID == "" || userID == msg.Channel {
				continue
			}
			b.hub.Deliver(userID, []byte(msg.Payload))
		}
	}
}

package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRoomFromEnvelope(t *testing.T) {
	t.Parallel()
	raw := MustMarshalEnvelope("chat.message.created", RoomRef{
		Kind:  "friend",
		RefID: "user-b",
	})
	var env Envelope
	require.NoError(t, json.Unmarshal(raw, &env))
	room, ok := RoomFromEnvelope(env)
	require.True(t, ok)
	require.Equal(t, "friend", room.Kind)
	require.Equal(t, "user-b", room.RefID)
}

func TestHubDeliverFiltersBySubscribe(t *testing.T) {
	t.Parallel()
	h := NewHub()
	c1 := testConn(h, "user-a")
	c2 := testConn(h, "user-a")
	c1.Subscribe(RoomRef{Kind: "friend", RefID: "user-b"})

	payload := MustMarshalEnvelope("chat.message.created", map[string]any{
		"kind":    "friend",
		"ref_id":  "user-b",
		"message": map[string]any{"id": "m1"},
	})
	h.Deliver("user-a", payload)

	select {
	case got := <-c1.send:
		require.Equal(t, payload, got)
	case <-time.After(time.Second):
		t.Fatal("expected deliver to subscribed conn")
	}
	select {
	case <-c2.send:
		t.Fatal("unsubscribed conn must not receive")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHubDeliverDropsSlowClient(t *testing.T) {
	t.Parallel()
	h := NewHub()
	c := testConn(h, "user-a")
	c.Subscribe(RoomRef{Kind: "friend", RefID: "f1"})
	for i := 0; i < sendBuffer; i++ {
		require.True(t, c.enqueue([]byte("x")))
	}
	payload := MustMarshalEnvelope("chat.message.created", RoomRef{Kind: "friend", RefID: "f1"})
	h.Deliver("user-a", payload)

	h.mu.RLock()
	_, still := h.users["user-a"][c]
	h.mu.RUnlock()
	require.False(t, still, "slow conn must be unregistered")
}

func TestHubRebind(t *testing.T) {
	t.Parallel()
	h := NewHub()
	c := testConn(h, "")
	h.Rebind(c, "user-a")
	require.Equal(t, "user-a", c.UserID())
	h.mu.RLock()
	_, ok := h.users["user-a"][c]
	_, emptyLeft := h.users[""]
	h.mu.RUnlock()
	require.True(t, ok)
	require.False(t, emptyLeft)
}

func testConn(h *Hub, userID string) *Conn {
	c := &Conn{
		id:     userID + "-test",
		userID: userID,
		hub:    h,
		send:   make(chan []byte, sendBuffer),
		subs:   make(map[RoomRef]struct{}),
	}
	h.mu.Lock()
	if h.users[userID] == nil {
		h.users[userID] = make(map[*Conn]struct{})
	}
	h.users[userID][c] = struct{}{}
	h.mu.Unlock()
	return c
}

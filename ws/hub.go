package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8 << 10
	sendBuffer     = 64
)

type Conn struct {
	id     string
	userID string
	hub    *Hub
	ws     *websocket.Conn
	send   chan []byte

	mu        sync.Mutex
	subs      map[RoomRef]struct{}
	closeOnce sync.Once
}

func (c *Conn) Subscribe(room RoomRef) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.subs == nil {
		c.subs = make(map[RoomRef]struct{})
	}
	c.subs[room] = struct{}{}
}

func (c *Conn) Unsubscribe(room RoomRef) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subs, room)
}

func (c *Conn) subscribed(room RoomRef) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.subs[room]
	return ok
}

func (c *Conn) enqueue(payload []byte) bool {
	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

func (c *Conn) closeSend() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}

type Hub struct {
	mu    sync.RWMutex
	users map[string]map[*Conn]struct{}
}

func NewHub() *Hub {
	return &Hub{users: make(map[string]map[*Conn]struct{})}
}

func (h *Hub) Register(userID string, conn *websocket.Conn) *Conn {
	c := &Conn{
		id:     conn.RemoteAddr().String() + ":" + time.Now().Format("150405.000"),
		userID: userID,
		hub:    h,
		ws:     conn,
		send:   make(chan []byte, sendBuffer),
		subs:   make(map[RoomRef]struct{}),
	}
	h.mu.Lock()
	if h.users[userID] == nil {
		h.users[userID] = make(map[*Conn]struct{})
	}
	h.users[userID][c] = struct{}{}
	h.mu.Unlock()
	go c.writePump()
	return c
}

func (h *Hub) Unregister(c *Conn) {
	if c == nil {
		return
	}
	h.mu.Lock()
	conns := h.users[c.userID]
	if conns != nil {
		if _, ok := conns[c]; ok {
			delete(conns, c)
			if len(conns) == 0 {
				delete(h.users, c.userID)
			}
			c.closeSend()
		}
	}
	h.mu.Unlock()
	if c.ws != nil {
		_ = c.ws.Close()
	}
}

// Rebind moves an existing connection to another userID (post first-frame auth).
func (h *Hub) Rebind(c *Conn, userID string) {
	if c == nil || userID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	old := c.userID
	if conns := h.users[old]; conns != nil {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.users, old)
		}
	}
	c.userID = userID
	if h.users[userID] == nil {
		h.users[userID] = make(map[*Conn]struct{})
	}
	h.users[userID][c] = struct{}{}
}

func (h *Hub) Deliver(userID string, raw []byte) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return
	}
	room, hasRoom := RoomFromEnvelope(env)

	h.mu.RLock()
	conns := h.users[userID]
	targets := make([]*Conn, 0, len(conns))
	for c := range conns {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		if hasRoom && !c.subscribed(room) {
			continue
		}
		if !c.enqueue(raw) {
			h.Unregister(c)
		}
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	all := make([]*Conn, 0)
	for _, conns := range h.users {
		for c := range conns {
			all = append(all, c)
		}
	}
	h.users = make(map[string]map[*Conn]struct{})
	h.mu.Unlock()
	for _, c := range all {
		c.closeSend()
		if c.ws != nil {
			_ = c.ws.Close()
		}
	}
}

func (c *Conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.hub.Unregister(c)
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Conn) ReadLoop(onMessage func(env Envelope) error) {
	defer c.hub.Unregister(c)
	c.ws.SetReadLimit(maxMessageSize)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			c.enqueue(MustMarshalEnvelope(TypeError, ErrorPayload{Code: ErrBadRequest, Message: "invalid envelope"}))
			continue
		}
		if err := onMessage(env); err != nil {
			return
		}
	}
}

func (c *Conn) SendEnvelope(typ string, payload any) {
	c.enqueue(MustMarshalEnvelope(typ, payload))
}

func (c *Conn) UserID() string { return c.userID }

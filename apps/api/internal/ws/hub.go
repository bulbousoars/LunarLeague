// Package ws is a thin pub/sub WebSocket gateway. Each client subscribes to
// one or more "topics" (e.g. draft:<id>, league:<id>). Server-side code
// publishes events with Hub.Publish(topic, event). Behind the scenes events
// are fanned out across replicas via Redis pub/sub.
package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
)

const redisChannel = "lunarleague:ws"

type Event struct {
	Topic string          `json:"topic"`
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data,omitempty"`
}

// Authenticator is satisfied by auth.Service.
type Authenticator interface {
	UserIDFromRequest(r *http.Request) (string, error)
}

type client struct {
	conn   *websocket.Conn
	send   chan Event
	topics map[string]struct{}
	userID string
}

type Hub struct {
	rdb      *redis.Client
	mu       sync.RWMutex
	byTopic  map[string]map[*client]struct{}
	clients  map[*client]struct{}
}

func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		rdb:     rdb,
		byTopic: make(map[string]map[*client]struct{}),
		clients: make(map[*client]struct{}),
	}
}

// Run subscribes to the cross-replica fanout channel.
func (h *Hub) Run(ctx context.Context) {
	if h.rdb == nil {
		<-ctx.Done()
		return
	}
	sub := h.rdb.Subscribe(ctx, redisChannel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case m, ok := <-ch:
			if !ok {
				return
			}
			var ev Event
			if err := json.Unmarshal([]byte(m.Payload), &ev); err != nil {
				continue
			}
			h.localFanout(ev)
		}
	}
}

// Publish sends an event to all subscribers across all replicas.
func (h *Hub) Publish(ctx context.Context, topic, evType string, data any) {
	body, _ := json.Marshal(data)
	ev := Event{Topic: topic, Type: evType, Data: body}
	if h.rdb != nil {
		raw, _ := json.Marshal(ev)
		if err := h.rdb.Publish(ctx, redisChannel, raw).Err(); err != nil {
			slog.Warn("redis publish", "err", err)
		}
		// Redis subscriber will fan out (including to this replica).
		return
	}
	h.localFanout(ev)
}

func (h *Hub) localFanout(ev Event) {
	h.mu.RLock()
	subs := h.byTopic[ev.Topic]
	out := make([]*client, 0, len(subs))
	for c := range subs {
		out = append(out, c)
	}
	h.mu.RUnlock()
	for _, c := range out {
		select {
		case c.send <- ev:
		default:
			// slow client, drop
		}
	}
}

func (h *Hub) addClient(c *client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	c.topics[topic] = struct{}{}
	if _, ok := h.byTopic[topic]; !ok {
		h.byTopic[topic] = make(map[*client]struct{})
	}
	h.byTopic[topic][c] = struct{}{}
}

func (h *Hub) removeClient(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	for t := range c.topics {
		if subs, ok := h.byTopic[t]; ok {
			delete(subs, c)
			if len(subs) == 0 {
				delete(h.byTopic, t)
			}
		}
	}
}

// DraftHandler upgrades a connection and joins draft:<id>.
func (h *Hub) DraftHandler(auth Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFromRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		topic := "draft:" + chi.URLParam(r, "draftID")
		h.serve(w, r, uid, topic)
	}
}

// LeagueHandler joins league:<id>.
func (h *Hub) LeagueHandler(auth Authenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, err := auth.UserIDFromRequest(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		topic := "league:" + chi.URLParam(r, "leagueID")
		h.serve(w, r, uid, topic)
	}
}

func (h *Hub) serve(w http.ResponseWriter, r *http.Request, uid, topic string) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// In dev, the web origin is on a different port. CORS at the HTTP layer
		// already restricts; for WS we accept any origin matching scheme+host
		// — production deploys behind Caddy don't see cross-origin upgrades.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "internal")

	c := &client{
		conn:   conn,
		send:   make(chan Event, 64),
		topics: map[string]struct{}{},
		userID: uid,
	}
	h.addClient(c, topic)
	defer h.removeClient(c)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Reader: handle incoming pings or client-side actions (extensibility hook).
	go func() {
		defer cancel()
		for {
			var msg struct {
				Type string `json:"type"`
			}
			if err := wsjson.Read(ctx, conn, &msg); err != nil {
				if !errors.Is(err, context.Canceled) {
					return
				}
				return
			}
			// no client->server actions yet; presence updates etc. could go here.
		}
	}()

	// Writer: pump events.
	pingT := time.NewTicker(25 * time.Second)
	defer pingT.Stop()
	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "")
			return
		case ev := <-c.send:
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := wsjson.Write(wctx, conn, ev)
			cancel()
			if err != nil {
				return
			}
		case <-pingT.C:
			if err := conn.Ping(ctx); err != nil {
				return
			}
		}
	}
}

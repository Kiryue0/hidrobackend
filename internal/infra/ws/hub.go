// Package ws: WebSocket hub (secondary adapter, LiveBroadcastPort) + bağlantı yönetimi.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/gorilla/websocket"

	"github.com/kiryue0/hidrobackend/internal/app/ports"
)

// Hub kabin bazlı abonelik havuzu; ports.LiveBroadcastPort implementasyonu.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	events     chan ports.LiveEvent
	clients    map[*Client]struct{}
	done       chan struct{} // Run çıkınca kapanır; register/unregister sızıntısını önler
	log        *slog.Logger
}

// NewHub hub oluşturur (Run ile başlatılır).
func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		events:     make(chan ports.LiveEvent, 256),
		clients:    make(map[*Client]struct{}),
		done:       make(chan struct{}),
		log:        log,
	}
}

// Run hub döngüsünü çalıştırır (tek goroutine; clients map'ine yalnızca burada dokunulur).
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			close(h.done) // bekleyen register/unregister gönderimlerini serbest bırak
			for c := range h.clients {
				close(c.send)
			}
			h.clients = nil
			return
		case c := <-h.register:
			h.clients[c] = struct{}{}
		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		case ev := <-h.events:
			h.dispatch(ev)
		}
	}
}

// dispatch olayı ilgili kabine abone istemcilere gönderir.
func (h *Hub) dispatch(ev ports.LiveEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		h.log.Error("ws event marshal hatası", "err", err)
		return
	}
	for c := range h.clients {
		if !c.subscribedTo(ev.CabinID) {
			continue
		}
		select {
		case c.send <- data:
		default:
			// Yavaş istemci: bağlantıyı kapat (writePump tarafından sonlandırılır).
			delete(h.clients, c)
			close(c.send)
		}
	}
}

// AddClient yükseltilmiş bir bağlantıyı verilen kabine abone olarak kaydeder
// ve okuma/yazma pump'larını başlatır. Yetki kontrolü çağırandan önce yapılmış olmalıdır.
func (h *Hub) AddClient(conn *websocket.Conn, cabinID string) {
	c := &Client{
		hub:     h,
		conn:    conn,
		send:    make(chan []byte, sendBuffer),
		cabinID: cabinID,
	}
	select {
	case h.register <- c:
		go c.writePump()
		go c.readPump()
	case <-h.done:
		// Hub kapanıyor: bağlantıyı kapat, pump başlatma.
		_ = conn.Close()
	}
}

// Broadcast ports.LiveBroadcastPort: olayı hub kuyruğuna koyar (bloklamaz).
func (h *Hub) Broadcast(ev ports.LiveEvent) {
	select {
	case h.events <- ev:
	default:
		h.log.Warn("ws event kuyruğu dolu, olay düşürüldü", "cabin", ev.CabinID, "type", ev.Type)
	}
}

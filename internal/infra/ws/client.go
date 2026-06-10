package ws

import (
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	sendBuffer = 32
)

// Client tek bir WebSocket bağlantısı (bir kabine abone).
type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	cabinID string // bu istemcinin abone olduğu (ve sahibi olduğu) kabin
}

func (c *Client) subscribedTo(cabinID string) bool {
	return c.cabinID == cabinID
}

// readPump bağlantıyı canlı tutar (ping/pong) ve kapanışı tespit eder.
// İstemciden gelen mesajlar yok sayılır (tek yönlü yayın).
func (c *Client) readPump() {
	defer func() {
		// Hub kapandıysa unregister'a yazım bloklanmasın (sızıntı önleme).
		select {
		case c.hub.unregister <- c:
		case <-c.hub.done:
		}
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump send kanalındaki mesajları ve periyodik ping'leri yazar.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// hub kanalı kapattı
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

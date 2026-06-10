package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/app/usecases"
	"github.com/kiryue0/hidrobackend/internal/infra/ws"
)

// WSHandler canlı yayın WebSocket bağlantılarını kabul eder.
// Tarayıcı WS başlık gönderemediği için token query param (?token=) veya
// Authorization başlığından okunur. Kabin sahipliği yükseltmeden ÖNCE doğrulanır.
type WSHandler struct {
	hub      *ws.Hub
	tokens   ports.TokenIssuer
	cabins   *usecases.CabinService
	upgrader websocket.Upgrader
}

// NewWSHandler handler üretir. allowedOrigins boş/nil ise tüm origin'lere izin verilir
// (yalnızca geliştirme); doluysa Origin başlığı bu listede olmalıdır.
func NewWSHandler(hub *ws.Hub, tokens ports.TokenIssuer, cabins *usecases.CabinService, allowedOrigins []string) *WSHandler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	checkOrigin := func(r *http.Request) bool {
		if len(allowed) == 0 {
			return true // geliştirme: kısıt yok
		}
		_, ok := allowed[r.Header.Get("Origin")]
		return ok
	}
	return &WSHandler{
		hub:    hub,
		tokens: tokens,
		cabins: cabins,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     checkOrigin,
		},
	}
}

// Handle GET /ws?token=...&cabin_id=CAB-XXXXXX
func (h *WSHandler) Handle(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimSpace(auth[len("Bearer "):])
		}
	}
	userID, err := h.tokens.Parse(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "geçersiz token"})
		return
	}

	cabinID := c.Query("cabin_id")
	// Sahiplik kontrolü (sahip değilse ErrForbidden/ErrNotFound -> uygun kod).
	if _, err := h.cabins.Get(c.Request.Context(), userID, cabinID); err != nil {
		respondError(c, err)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade başarısızsa gorilla zaten yanıt yazdı.
		return
	}
	h.hub.AddClient(conn, cabinID)
}

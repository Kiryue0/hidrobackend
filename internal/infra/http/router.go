package http

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kiryue0/hidrobackend/internal/app/ports"
)

// Pinger /health icin DB erisilebilirligini kontrol eder.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps router'in ihtiyac duydugu bagimliliklar.
type Deps struct {
	Auth    *AuthHandler
	Cabin   *CabinHandler
	Control *ControlHandler
	History *HistoryHandler
	WS      *WSHandler
	Tokens  ports.TokenIssuer
	DB      Pinger
	WebDir  string // statik test arayuzu dizini (bos => kapali)
}

// NewRouter tum rotalari kurar.
func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "down"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "up"})
	})

	auth := r.Group("/auth")
	{
		auth.POST("/register", d.Auth.Register)
		auth.POST("/login", d.Auth.Login)
	}

	// Korumali uclar.
	api := r.Group("/api", AuthMiddleware(d.Tokens))
	// /api/me: JWT middleware'in dogru calistigini gosteren minimal uc.
	api.GET("/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": userIDFrom(c)})
	})

	// Hesap silme: kullanici + tum kabinler cascade silinir.
	if d.Auth != nil {
		api.DELETE("/account", d.Auth.DeleteAccount)
	}

	if d.Cabin != nil {
		api.POST("/cabins", d.Cabin.Create)
		api.GET("/cabins", d.Cabin.List)
		api.POST("/cabins/claim", d.Cabin.Claim)
		api.GET("/cabins/:id", d.Cabin.Get)
	}
	if d.Control != nil {
		api.POST("/cabins/:id/command", d.Control.SendCommand)
		api.PUT("/cabins/:id/config", d.Control.UpdateConfig)
	}
	if d.History != nil {
		api.GET("/cabins/:id/readings", d.History.GetReadings)
	}

	// WebSocket: kendi auth'unu yapar (token query param), bu yuzden /api disinda.
	if d.WS != nil {
		r.GET("/ws", d.WS.Handle)
	}

	// Statik test arayuzu (tek sayfa, vanilla HTML/CSS/JS).
	if d.WebDir != "" {
		index := filepath.Join(d.WebDir, "index.html")
		r.StaticFile("/", index)
		r.StaticFile("/index.html", index)
	}

	return r
}

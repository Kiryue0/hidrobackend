package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeTokens: belirli bir token string'ini geçerli sayar, kalanı reddeder.
type fakeTokens struct {
	valid  string
	userID int64
}

func (f fakeTokens) Issue(id int64) (string, error) { return f.valid, nil }
func (f fakeTokens) Parse(tok string) (int64, error) {
	if tok == f.valid {
		return f.userID, nil
	}
	return 0, errors.New("geçersiz")
}

func setupMW(tokens fakeTokens) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/me", AuthMiddleware(tokens), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": userIDFrom(c)})
	})
	return r
}

func do(r *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMiddleware_ValidToken(t *testing.T) {
	r := setupMW(fakeTokens{valid: "good", userID: 99})
	w := do(r, "Bearer good")
	if w.Code != http.StatusOK {
		t.Fatalf("geçerli token 200 dönmeli, geldi %d", w.Code)
	}
	if body := w.Body.String(); body != `{"user_id":99}` {
		t.Fatalf("userID context'e doğru konmalı, body=%s", body)
	}
}

func TestMiddleware_NoHeader(t *testing.T) {
	r := setupMW(fakeTokens{valid: "good"})
	if w := do(r, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("header yoksa 401, geldi %d", w.Code)
	}
}

func TestMiddleware_MissingBearerPrefix(t *testing.T) {
	r := setupMW(fakeTokens{valid: "good"})
	// "Bearer " prefix'i yoksa token denenmeden reddedilmeli.
	if w := do(r, "good"); w.Code != http.StatusUnauthorized {
		t.Fatalf("Bearer prefix yoksa 401, geldi %d", w.Code)
	}
}

func TestMiddleware_BadToken(t *testing.T) {
	r := setupMW(fakeTokens{valid: "good"})
	if w := do(r, "Bearer kotu-token"); w.Code != http.StatusUnauthorized {
		t.Fatalf("geçersiz token 401, geldi %d", w.Code)
	}
}

package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// checkOrigin closure'ı NewWSHandler içinde kurulur; upgrader.CheckOrigin
// üzerinden test edilir. Boş liste => hepsine izin; dolu liste => Origin eşleşmesi.
func TestCheckOrigin_EmptyListAllowsAll(t *testing.T) {
	h := NewWSHandler(nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Origin", "http://evil.example")
	if !h.upgrader.CheckOrigin(req) {
		t.Fatal("boş allowlist ile herhangi bir origin kabul edilmeli")
	}
	// Origin başlığı hiç olmadan da kabul.
	req2 := httptest.NewRequest(http.MethodGet, "/ws", nil)
	if !h.upgrader.CheckOrigin(req2) {
		t.Fatal("boş allowlist ile origin başlığı olmadan da kabul edilmeli")
	}
}

func TestCheckOrigin_AllowlistMatch(t *testing.T) {
	h := NewWSHandler(nil, nil, nil, []string{"http://localhost:3000", "https://app.example"})

	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:3000", true},
		{"https://app.example", true},
		{"http://evil.example", false},
		{"", false},                       // Origin başlığı yok -> reddet
		{"http://localhost:3000/", false}, // trailing slash farklı -> reddet (tam eşleşme)
		{"HTTP://localhost:3000", false},  // büyük/küçük harf farkı -> reddet
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/ws", nil)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		if got := h.upgrader.CheckOrigin(req); got != tc.want {
			t.Errorf("origin %q: CheckOrigin=%v, beklenen %v", tc.origin, got, tc.want)
		}
	}
}

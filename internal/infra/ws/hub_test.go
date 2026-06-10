package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/kiryue0/hidrobackend/internal/app/ports"
)

func testHub() *Hub {
	return NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// newTestClient hub'a kayıtlı, gerçek conn'suz bir istemci üretir.
// Pump'lar başlatılmaz; yalnızca hub'ın kanal/dispatch mantığı test edilir.
func newTestClient(h *Hub, cabinID string) *Client {
	return &Client{
		hub:     h,
		send:    make(chan []byte, sendBuffer),
		cabinID: cabinID,
	}
}

// TestDispatchIsolation: kabin A'ya abone istemci kabin B olayını ALMAMALI.
func TestDispatchIsolation(t *testing.T) {
	h := testHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	a := newTestClient(h, "CAB-AAAAAA")
	b := newTestClient(h, "CAB-BBBBBB")
	h.register <- a
	h.register <- b

	h.Broadcast(ports.LiveEvent{Type: "reading", CabinID: "CAB-AAAAAA", Data: map[string]int{"t": 1}})

	select {
	case msg := <-a.send:
		var ev ports.LiveEvent
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if ev.CabinID != "CAB-AAAAAA" {
			t.Fatalf("yanlış kabin: %s", ev.CabinID)
		}
	case <-time.After(time.Second):
		t.Fatal("A olayı almadı")
	}

	select {
	case <-b.send:
		t.Fatal("izolasyon ihlali: B kabin A olayını aldı")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestSlowClientClosedAndUnregisterNoDoubleClose:
// dispatch yavaş istemciyi delete+close eder; ardından readPump'ın yaptığı gibi
// unregister gönderilir. Run'daki guard sayesinde double-close PANİĞİ olmamalı.
func TestSlowClientClosedAndUnregisterNoDoubleClose(t *testing.T) {
	h := testHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	// send buffer kapasitesini sıfır okuyucu ile doldur -> dispatch'te yavaş sayılır.
	slow := newTestClient(h, "CAB-SLOW00")
	for i := 0; i < cap(slow.send); i++ {
		slow.send <- []byte("x")
	}
	h.register <- slow

	// Bu broadcast slow.send dolu olduğu için delete+close tetikler.
	h.Broadcast(ports.LiveEvent{Type: "reading", CabinID: "CAB-SLOW00"})

	// dispatch'in işlemesini garanti et: senkronize bir noop event akışı.
	syncHub(t, h)

	// readPump davranışını taklit et: kapanan istemci unregister gönderir.
	// Guard sayesinde tekrar close edilmemeli (panic olmamalı).
	h.unregister <- slow

	syncHub(t, h)
	// Buraya panic olmadan ulaşıldıysa test geçer.
}

// TestConcurrentBroadcastAndConnections: race detector altında yoğun yayın + kayıt/silme.
func TestConcurrentBroadcastAndConnections(t *testing.T) {
	h := testHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	var wg sync.WaitGroup

	// Broadcaster'lar (TelemetryService'i taklit eder).
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				h.Broadcast(ports.LiveEvent{Type: "reading", CabinID: "CAB-AAAAAA"})
			}
		}()
	}

	// Sürekli kayıt olan ve mesajları tüketen istemciler.
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := newTestClient(h, "CAB-AAAAAA")
			h.register <- c
			go func() {
				for range c.send {
				}
			}()
			time.Sleep(5 * time.Millisecond)
			// readPump benzeri unregister.
			h.unregister <- c
		}()
	}

	wg.Wait()
}

// TestShutdownReleasesBlockedRegister: Run çıktıktan sonra register'a yazım
// done kanalı sayesinde bloklanmamalı (goroutine sızıntısı önleme).
func TestShutdownReleasesBlockedRegister(t *testing.T) {
	h := testHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)

	// Run'ı durdur ve done'ın kapanmasını bekle.
	cancel()
	select {
	case <-h.done:
	case <-time.After(time.Second):
		t.Fatal("Run çıkışında done kapanmadı")
	}

	// Run artık register'ı tüketmiyor; AddClient mantığındaki select done'a düşmeli.
	done := make(chan struct{})
	go func() {
		defer close(done)
		c := newTestClient(h, "CAB-AFTER0")
		select {
		case h.register <- c:
			t.Error("Run kapalıyken register başarılı olmamalı")
		case <-h.done:
			// beklenen yol: done kapalı, bloklanmadan çık.
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("kapanıştan sonra register gönderimi bloklandı (sızıntı)")
	}

	// readPump defer'inin taklidi: unregister gönderimi de bloklanmamalı.
	done2 := make(chan struct{})
	go func() {
		defer close(done2)
		c := newTestClient(h, "CAB-AFTER1")
		select {
		case h.unregister <- c:
			t.Error("Run kapalıyken unregister başarılı olmamalı")
		case <-h.done:
		}
	}()
	select {
	case <-done2:
	case <-time.After(time.Second):
		t.Fatal("kapanıştan sonra unregister gönderimi bloklandı (sızıntı)")
	}
}

// TestRunClosesDoneOnce: done kanalı yalnızca bir kez kapanmalı (çift-close paniği yok).
func TestRunClosesDoneOnce(t *testing.T) {
	h := testHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	<-h.done // ilk close
	// İkinci bir okuma da panik olmadan kapalı kanaldan dönmeli.
	select {
	case <-h.done:
	case <-time.After(time.Second):
		t.Fatal("done kapalı kalmalı")
	}
}

// syncHub Run goroutine'inin mevcut kuyruktaki işleri bitirdiğinden emin olur.
func syncHub(t *testing.T, h *Hub) {
	t.Helper()
	probe := newTestClient(h, "CAB-PROBE0")
	h.register <- probe   // Run register'ı işleyene kadar bekler (kanal unbuffered).
	h.unregister <- probe // Run unregister'ı işleyene kadar bekler.
}

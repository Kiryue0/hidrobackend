package usecases

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
	"github.com/kiryue0/hidrobackend/internal/domain/telemetry"
	"github.com/kiryue0/hidrobackend/internal/domain/user"
)

type fakeReadingStore struct {
	inserted []telemetry.Reading
	deleted  int64
}

func (f *fakeReadingStore) Insert(_ context.Context, r telemetry.Reading) error {
	f.inserted = append(f.inserted, r)
	return nil
}
func (f *fakeReadingStore) QueryRaw(_ context.Context, cabinID string, from, to time.Time, limit int32) ([]telemetry.Reading, error) {
	var out []telemetry.Reading
	for _, r := range f.inserted {
		if r.CabinID == cabinID && !r.Ts.Before(from) && !r.Ts.After(to) {
			out = append(out, r)
			if int32(len(out)) >= limit {
				break
			}
		}
	}
	return out, nil
}
func (f *fakeReadingStore) QueryHourly(_ context.Context, _ string, _, _ time.Time, _ int32) ([]telemetry.HourlyReading, error) {
	return nil, nil
}
func (f *fakeReadingStore) Latest(_ context.Context, cabinID string) (telemetry.Reading, error) {
	for i := len(f.inserted) - 1; i >= 0; i-- {
		if f.inserted[i].CabinID == cabinID {
			return f.inserted[i], nil
		}
	}
	return telemetry.Reading{}, apperr.ErrNotFound
}
func (f *fakeReadingStore) DeleteOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	var kept []telemetry.Reading
	var n int64
	for _, r := range f.inserted {
		if r.Ts.Before(cutoff) {
			n++
		} else {
			kept = append(kept, r)
		}
	}
	f.inserted = kept
	f.deleted += n
	return n, nil
}

type fakeAlertStore struct{ inserted []telemetry.Alert }

func (f *fakeAlertStore) Insert(_ context.Context, a telemetry.Alert) error {
	f.inserted = append(f.inserted, a)
	return nil
}

type fakeBroadcaster struct{ events []ports.LiveEvent }

func (f *fakeBroadcaster) Broadcast(ev ports.LiveEvent) { f.events = append(f.events, ev) }

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newTelemetry() (*TelemetryService, *fakeCabinRepo, *fakeReadingStore, *fakeAlertStore, *fakeUserRepo, *fakeBroadcaster) {
	cr := newFakeCabinRepo()
	rs := &fakeReadingStore{}
	as := &fakeAlertStore{}
	ur := newFakeRepo()
	bc := &fakeBroadcaster{}
	return NewTelemetryService(cr, rs, as, ur, bc, quietLog()), cr, rs, as, ur, bc
}

func TestIngestReading(t *testing.T) {
	s, cr, rs, _, _, bc := newTelemetry()
	err := s.IngestReading(context.Background(), telemetry.Reading{
		CabinID: "CAB-3778C4", Ts: time.Unix(1718000000, 0), Temperature: 24.5, Humidity: 60,
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(rs.inserted) != 1 {
		t.Fatal("reading yazılmalı")
	}
	if _, ok := cr.store["CAB-3778C4"]; !ok {
		t.Fatal("bilinmeyen kabin ensure edilmeli")
	}
	if len(bc.events) != 1 || bc.events[0].Type != "reading" {
		t.Fatal("reading event yayılmalı")
	}
}

func TestIngestReading_BadCabinID(t *testing.T) {
	s, _, rs, _, _, _ := newTelemetry()
	err := s.IngestReading(context.Background(), telemetry.Reading{CabinID: "bad"})
	if err == nil {
		t.Fatal("geçersiz kabin_id reddedilmeli")
	}
	if len(rs.inserted) != 0 {
		t.Fatal("geçersizde yazma olmamalı")
	}
}

func TestUpdateActuatorState(t *testing.T) {
	s, cr, _, _, _, bc := newTelemetry()
	err := s.UpdateActuatorState(context.Background(), "CAB-3778C4", ActuatorSnapshot{
		Humidifier: true, Fan1: 200, Fan2: 0, Source: "decision",
	})
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	c := cr.store["CAB-3778C4"]
	acts := c.Actuators()
	if !acts[cabin.ActHumidifier].On {
		t.Fatal("humidifier on olmalı")
	}
	if acts[cabin.ActFan1].Speed != 200 || !acts[cabin.ActFan1].On {
		t.Fatal("fan1 200 ve on olmalı")
	}
	if acts[cabin.ActFan2].On {
		t.Fatal("fan2 (speed 0) off olmalı")
	}
	if acts[cabin.ActFan1].Source != cabin.KaynakDecision {
		t.Fatal("kaynak decision olmalı")
	}
	if len(bc.events) != 1 || bc.events[0].Type != "state" {
		t.Fatal("state event yayılmalı")
	}
}

func TestRecordHeartbeat_ClaimsByUsername(t *testing.T) {
	s, cr, _, _, ur, _ := newTelemetry()
	// kullanıcı oluştur
	u, _ := user.NewUser("melih", "m@x.com", "hash")
	if err := ur.Create(context.Background(), u); err != nil {
		t.Fatalf("kullanıcı oluşturma: %v", err)
	}
	err := s.RecordHeartbeat(context.Background(), "CAB-3778C4", "melih", time.Now())
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	c := cr.store["CAB-3778C4"]
	if !c.Online() {
		t.Fatal("online olmalı")
	}
	if !c.IsOwnedBy(u.ID()) {
		t.Fatal("username eşleşmesiyle claim edilmeli")
	}
}

func TestRecordHeartbeat_UnknownUser_NoClaim(t *testing.T) {
	s, cr, _, _, _, _ := newTelemetry()
	if err := s.RecordHeartbeat(context.Background(), "CAB-3778C4", "yok", time.Now()); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if cr.store["CAB-3778C4"].OwnerUserID() != nil {
		t.Fatal("bilinmeyen kullanıcı claim etmemeli")
	}
}

func TestSetCabinStatus(t *testing.T) {
	s, cr, _, _, _, bc := newTelemetry()
	_ = s.SetCabinStatus(context.Background(), "CAB-3778C4", true)
	if !cr.store["CAB-3778C4"].Online() {
		t.Fatal("online olmalı")
	}
	_ = s.SetCabinStatus(context.Background(), "CAB-3778C4", false)
	if cr.store["CAB-3778C4"].Online() {
		t.Fatal("offline olmalı")
	}
	if len(bc.events) != 2 {
		t.Fatal("iki status event yayılmalı")
	}
}

func TestRecordAlert(t *testing.T) {
	s, _, _, as, _, bc := newTelemetry()
	err := s.RecordAlert(context.Background(), telemetry.Alert{
		CabinID: "CAB-3778C4", Ts: time.Now(), Type: telemetry.AlertKritik, Message: "Sicaklik yuksek",
	})
	if err != nil {
		t.Fatalf("alert: %v", err)
	}
	if len(as.inserted) != 1 {
		t.Fatal("alert yazılmalı")
	}
	if len(bc.events) != 1 || bc.events[0].Type != "alert" {
		t.Fatal("alert event yayılmalı")
	}
}

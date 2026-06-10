package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
	"github.com/kiryue0/hidrobackend/internal/domain/telemetry"
)

func setupHistory(t *testing.T) (*HistoryService, *fakeCabinRepo, *fakeReadingStore) {
	t.Helper()
	repo := newFakeCabinRepo()
	c, _ := cabin.NewCabin(mustCID(t, "CAB-3778C4"), "")
	_ = c.AssignOwner(1)
	repo.store["CAB-3778C4"] = c
	rs := &fakeReadingStore{}
	return NewHistoryService(repo, rs), repo, rs
}

func TestGetSensorHistory_OwnerOnly(t *testing.T) {
	s, _, _ := setupHistory(t)
	_, err := s.GetSensorHistory(context.Background(), 2, "CAB-3778C4", HistoryQuery{})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("sahip değil -> forbidden: %v", err)
	}
}

func TestGetSensorHistory_Raw(t *testing.T) {
	s, _, rs := setupHistory(t)
	now := time.Now()
	rs.inserted = []telemetry.Reading{
		{CabinID: "CAB-3778C4", Ts: now.Add(-1 * time.Hour), Temperature: 24},
		{CabinID: "CAB-3778C4", Ts: now.Add(-30 * time.Minute), Temperature: 25},
		{CabinID: "OTHER", Ts: now, Temperature: 99},
	}
	res, err := s.GetSensorHistory(context.Background(), 1, "CAB-3778C4", HistoryQuery{})
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(res.Raw) != 2 {
		t.Fatalf("yalnızca bu kabinin 2 ölçümü dönmeli, %d", len(res.Raw))
	}
}

func TestGetSensorHistory_FromAfterTo(t *testing.T) {
	s, _, _ := setupHistory(t)
	now := time.Now()
	_, err := s.GetSensorHistory(context.Background(), 1, "CAB-3778C4", HistoryQuery{From: now, To: now.Add(-time.Hour)})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("from>to -> validation: %v", err)
	}
}

func TestRetention_DeletesOld(t *testing.T) {
	rs := &fakeReadingStore{}
	now := time.Now()
	rs.inserted = []telemetry.Reading{
		{CabinID: "CAB-3778C4", Ts: now.AddDate(0, 0, -40)}, // eski
		{CabinID: "CAB-3778C4", Ts: now.AddDate(0, 0, -5)},  // güncel
	}
	svc := NewRetentionService(rs, 30, quietLog())
	svc.purge(context.Background())
	if rs.deleted != 1 {
		t.Fatalf("1 eski ölçüm silinmeli, silinen=%d", rs.deleted)
	}
	if len(rs.inserted) != 1 {
		t.Fatalf("1 güncel ölçüm kalmalı, kalan=%d", len(rs.inserted))
	}
}

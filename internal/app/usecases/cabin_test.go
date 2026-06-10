package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
)

type fakeCabinRepo struct {
	store map[string]*cabin.Cabin
}

func newFakeCabinRepo() *fakeCabinRepo { return &fakeCabinRepo{store: map[string]*cabin.Cabin{}} }

func (f *fakeCabinRepo) Create(_ context.Context, c *cabin.Cabin) error {
	if _, ok := f.store[c.ID().String()]; ok {
		return apperr.ErrConflict
	}
	f.store[c.ID().String()] = c
	return nil
}
func (f *fakeCabinRepo) GetByID(_ context.Context, id cabin.CabinId) (*cabin.Cabin, error) {
	if c, ok := f.store[id.String()]; ok {
		return c, nil
	}
	return nil, apperr.ErrNotFound
}
func (f *fakeCabinRepo) ListByOwner(_ context.Context, ownerID int64) ([]*cabin.Cabin, error) {
	var out []*cabin.Cabin
	for _, c := range f.store {
		if c.IsOwnedBy(ownerID) {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeCabinRepo) Exists(_ context.Context, id cabin.CabinId) (bool, error) {
	_, ok := f.store[id.String()]
	return ok, nil
}
func (f *fakeCabinRepo) Claim(_ context.Context, id cabin.CabinId, ownerID int64) error {
	c, ok := f.store[id.String()]
	if !ok {
		return apperr.ErrNotFound
	}
	return c.AssignOwner(ownerID)
}
func (f *fakeCabinRepo) EnsureCabin(_ context.Context, id cabin.CabinId) error {
	if _, ok := f.store[id.String()]; !ok {
		c, _ := cabin.NewCabin(id, "")
		f.store[id.String()] = c
	}
	return nil
}
func (f *fakeCabinRepo) UpsertActuatorState(_ context.Context, id cabin.CabinId, st cabin.ActuatorState) error {
	c, ok := f.store[id.String()]
	if !ok {
		return apperr.ErrNotFound
	}
	return c.ApplyActuatorState(st)
}
func (f *fakeCabinRepo) MarkOnline(_ context.Context, id cabin.CabinId, lastSeen time.Time) error {
	if c, ok := f.store[id.String()]; ok {
		c.MarkOnline(lastSeen)
	}
	return nil
}
func (f *fakeCabinRepo) MarkOffline(_ context.Context, id cabin.CabinId) error {
	if c, ok := f.store[id.String()]; ok {
		c.MarkOffline()
	}
	return nil
}
func (f *fakeCabinRepo) UpdateConfig(_ context.Context, id cabin.CabinId, t cabin.Thresholds, d cabin.DecisionConfig) error {
	c, ok := f.store[id.String()]
	if !ok {
		return apperr.ErrNotFound
	}
	_ = c.UpdateThresholds(t)
	_ = c.UpdateDecisionConfig(d)
	return nil
}
func (f *fakeCabinRepo) DeleteCabinsByOwner(_ context.Context, ownerID int64) (int64, error) {
	var n int64
	for k, c := range f.store {
		if c.OwnerUserID() != nil && *c.OwnerUserID() == ownerID {
			delete(f.store, k)
			n++
		}
	}
	return n, nil
}

func TestCabinCreate_OK(t *testing.T) {
	s := NewCabinService(newFakeCabinRepo())
	c, err := s.Create(context.Background(), 1, "CAB-3778C4", "Salon")
	if err != nil {
		t.Fatalf("oluşmalı: %v", err)
	}
	if !c.IsOwnedBy(1) {
		t.Fatal("sahip atanmalı")
	}
}

func TestCabinCreate_BadID(t *testing.T) {
	s := NewCabinService(newFakeCabinRepo())
	_, err := s.Create(context.Background(), 1, "bad-id", "")
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("validation bekleniyordu: %v", err)
	}
}

func TestCabinCreate_Duplicate(t *testing.T) {
	s := NewCabinService(newFakeCabinRepo())
	_, _ = s.Create(context.Background(), 1, "CAB-3778C4", "")
	_, err := s.Create(context.Background(), 1, "CAB-3778C4", "")
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("conflict bekleniyordu: %v", err)
	}
}

func TestClaim_NewCabin(t *testing.T) {
	s := NewCabinService(newFakeCabinRepo())
	c, err := s.Claim(context.Background(), 5, "CAB-ABCDEF")
	if err != nil || !c.IsOwnedBy(5) {
		t.Fatalf("yeni kabin claim edilmeli: %v", err)
	}
}

func TestClaim_Idempotent(t *testing.T) {
	s := NewCabinService(newFakeCabinRepo())
	_, _ = s.Claim(context.Background(), 5, "CAB-ABCDEF")
	if _, err := s.Claim(context.Background(), 5, "CAB-ABCDEF"); err != nil {
		t.Fatalf("aynı kullanıcı tekrar claim idempotent olmalı: %v", err)
	}
}

func TestClaim_OwnedByAnother(t *testing.T) {
	s := NewCabinService(newFakeCabinRepo())
	_, _ = s.Claim(context.Background(), 5, "CAB-ABCDEF")
	_, err := s.Claim(context.Background(), 9, "CAB-ABCDEF")
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("başka sahip -> conflict: %v", err)
	}
}

func TestList_OwnerIsolation(t *testing.T) {
	s := NewCabinService(newFakeCabinRepo())
	if _, err := s.Create(context.Background(), 1, "CAB-3778C4", ""); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := s.Create(context.Background(), 2, "CAB-ABCDEF", ""); err != nil {
		t.Fatalf("setup: %v", err)
	}
	list1, err := s.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list1) != 1 || !list1[0].IsOwnedBy(1) {
		t.Fatalf("kullanıcı 1 yalnızca kendi kabinini görmeli, görülen: %d", len(list1))
	}
}

func TestGet_Forbidden(t *testing.T) {
	repo := newFakeCabinRepo()
	s := NewCabinService(repo)
	_, _ = s.Create(context.Background(), 1, "CAB-3778C4", "")
	_, err := s.Get(context.Background(), 2, "CAB-3778C4")
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("sahip olmayan -> forbidden: %v", err)
	}
}

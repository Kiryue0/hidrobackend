package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
	"github.com/kiryue0/hidrobackend/internal/infra/postgres/db"
)

// CabinRepo ports.CabinRepository implementasyonu.
type CabinRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// NewCabinRepo pool ve sqlc Queries ile repo üretir.
func NewCabinRepo(pool *pgxpool.Pool, q *db.Queries) *CabinRepo {
	return &CabinRepo{pool: pool, q: q}
}

// Create cabins + cabin_config satırlarını tek transaction'da ekler.
func (r *CabinRepo) Create(ctx context.Context, c *cabin.Cabin) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit sonrası no-op

	qtx := r.q.WithTx(tx)

	if _, err := qtx.CreateCabin(ctx, db.CreateCabinParams{
		ID:          c.ID().String(),
		OwnerUserID: c.OwnerUserID(),
		Name:        c.Name(),
	}); err != nil {
		if isUniqueViolation(err) {
			return apperr.ErrConflict
		}
		return err
	}

	thr, err := json.Marshal(c.Thresholds())
	if err != nil {
		return err
	}
	dec, err := json.Marshal(c.Decision())
	if err != nil {
		return err
	}
	if err := qtx.InsertCabinConfig(ctx, db.InsertCabinConfigParams{
		CabinID:    c.ID().String(),
		Thresholds: thr,
		Decision:   dec,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetByID kabini config + aktüatör durumlarıyla yükler.
func (r *CabinRepo) GetByID(ctx context.Context, id cabin.CabinId) (*cabin.Cabin, error) {
	row, err := r.q.GetCabin(ctx, id.String())
	if err != nil {
		return nil, mapNotFound(err)
	}

	thresholds := cabin.DefaultThresholds()
	decision := cabin.DefaultDecisionConfig()
	cfg, err := r.q.GetCabinConfig(ctx, id.String())
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		if uerr := json.Unmarshal(cfg.Thresholds, &thresholds); uerr != nil {
			return nil, fmt.Errorf("thresholds çözümleme: %w", uerr)
		}
		if uerr := json.Unmarshal(cfg.Decision, &decision); uerr != nil {
			return nil, fmt.Errorf("decision çözümleme: %w", uerr)
		}
	}

	actRows, err := r.q.ListActuatorStates(ctx, id.String())
	if err != nil {
		return nil, err
	}
	actuators := make(map[cabin.ActuatorType]cabin.ActuatorState, len(actRows))
	for _, a := range actRows {
		at, perr := cabin.ParseActuatorType(a.Actuator)
		if perr != nil {
			continue // bilinmeyen aktüatör tipini atla
		}
		actuators[at] = cabin.ActuatorState{
			Type:   at,
			On:     a.State,
			Speed:  int(a.Speed),
			Source: cabin.KomutKaynagi(a.Source),
		}
	}

	return cabin.Reconstruct(
		id, row.OwnerUserID, row.Name,
		thresholds, decision, actuators,
		row.Online, row.LastSeen,
	), nil
}

// ListByOwner kullanıcının kabinlerini özet olarak döner (config/aktüatör yüklenmez).
func (r *CabinRepo) ListByOwner(ctx context.Context, ownerID int64) ([]*cabin.Cabin, error) {
	rows, err := r.q.ListCabinsByOwner(ctx, &ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]*cabin.Cabin, 0, len(rows))
	for _, row := range rows {
		id, perr := cabin.NewCabinId(row.ID)
		if perr != nil {
			continue
		}
		out = append(out, cabin.Reconstruct(
			id, row.OwnerUserID, row.Name,
			cabin.DefaultThresholds(), cabin.DefaultDecisionConfig(), nil,
			row.Online, row.LastSeen,
		))
	}
	return out, nil
}

// Exists kabin var mı.
func (r *CabinRepo) Exists(ctx context.Context, id cabin.CabinId) (bool, error) {
	return r.q.CabinExists(ctx, id.String())
}

// EnsureCabin kabin yoksa unclaimed olarak (default config ile) oluşturur. Idempotent.
func (r *CabinRepo) EnsureCabin(ctx context.Context, id cabin.CabinId) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := r.q.WithTx(tx)
	if err := qtx.EnsureCabin(ctx, id.String()); err != nil {
		return err
	}
	thr, err := json.Marshal(cabin.DefaultThresholds())
	if err != nil {
		return err
	}
	dec, err := json.Marshal(cabin.DefaultDecisionConfig())
	if err != nil {
		return err
	}
	if err := qtx.EnsureCabinConfig(ctx, db.EnsureCabinConfigParams{
		CabinID:    id.String(),
		Thresholds: thr,
		Decision:   dec,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpsertActuatorState cihazdan gelen otoriter aktüatör durumunu yazar.
func (r *CabinRepo) UpsertActuatorState(ctx context.Context, id cabin.CabinId, st cabin.ActuatorState) error {
	return r.q.UpsertActuatorState(ctx, db.UpsertActuatorStateParams{
		CabinID:  id.String(),
		Actuator: string(st.Type),
		State:    st.On,
		Speed:    int32(st.Speed),
		Source:   string(st.Source),
	})
}

// MarkOnline çevrimiçi + lastSeen günceller.
func (r *CabinRepo) MarkOnline(ctx context.Context, id cabin.CabinId, lastSeen time.Time) error {
	return r.q.MarkCabinOnline(ctx, db.MarkCabinOnlineParams{ID: id.String(), LastSeen: &lastSeen})
}

// MarkOffline çevrimdışı işaretler.
func (r *CabinRepo) MarkOffline(ctx context.Context, id cabin.CabinId) error {
	return r.q.MarkCabinOffline(ctx, id.String())
}

// UpdateConfig kabinin eşik + karar konfigürasyonunu günceller.
func (r *CabinRepo) UpdateConfig(ctx context.Context, id cabin.CabinId, t cabin.Thresholds, d cabin.DecisionConfig) error {
	thr, err := json.Marshal(t)
	if err != nil {
		return err
	}
	dec, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return r.q.UpdateCabinConfig(ctx, db.UpdateCabinConfigParams{
		CabinID:    id.String(),
		Thresholds: thr,
		Decision:   dec,
	})
}

// Claim kabini koşullu olarak atar (sahipsiz veya aynı sahip). Aksi halde ErrConflict.
func (r *CabinRepo) Claim(ctx context.Context, id cabin.CabinId, ownerID int64) error {
	n, err := r.q.ClaimCabin(ctx, db.ClaimCabinParams{
		ID:          id.String(),
		OwnerUserID: &ownerID,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.ErrConflict
	}
	return nil
}

// DeleteCabinsByOwner kullanıcının tüm kabinlerini cascade ile siler.
func (r *CabinRepo) DeleteCabinsByOwner(ctx context.Context, ownerID int64) (int64, error) {
	return r.q.DeleteCabinsByOwner(ctx, &ownerID)
}

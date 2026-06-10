package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
	"github.com/kiryue0/hidrobackend/internal/domain/telemetry"
)

const (
	maxHistoryLimit     = 5000
	defaultHistoryLimit = 1000
)

// HistoryService sensör geçmişi sorgusu (read model) — GetSensorHistoryQuery.
type HistoryService struct {
	cabins   ports.CabinRepository
	readings ports.ReadingStore
}

// NewHistoryService bağımlılıkları enjekte eder.
func NewHistoryService(cabins ports.CabinRepository, readings ports.ReadingStore) *HistoryService {
	return &HistoryService{cabins: cabins, readings: readings}
}

// HistoryQuery sorgu parametreleri.
type HistoryQuery struct {
	From   time.Time
	To     time.Time
	Limit  int32
	Hourly bool // true: saatlik downsample, false: ham
}

// HistoryResult ham veya saatlik sonuç (yalnızca biri dolu).
type HistoryResult struct {
	Raw    []telemetry.Reading
	Hourly []telemetry.HourlyReading
}

// authorize sahiplik kontrolü yapar.
func (s *HistoryService) authorize(ctx context.Context, ownerID int64, cabinID string) (cabin.CabinId, error) {
	id, err := cabin.NewCabinId(cabinID)
	if err != nil {
		return cabin.CabinId{}, fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	c, err := s.cabins.GetByID(ctx, id)
	if err != nil {
		return cabin.CabinId{}, err
	}
	if !c.IsOwnedBy(ownerID) {
		return cabin.CabinId{}, apperr.ErrForbidden
	}
	return id, nil
}

// GetSensorHistory sahiplik kontrolü + read model sorgusu.
func (s *HistoryService) GetSensorHistory(ctx context.Context, ownerID int64, cabinID string, q HistoryQuery) (HistoryResult, error) {
	id, err := s.authorize(ctx, ownerID, cabinID)
	if err != nil {
		return HistoryResult{}, err
	}

	// Varsayılan aralık: son 24 saat.
	if q.To.IsZero() {
		q.To = time.Now()
	}
	if q.From.IsZero() {
		q.From = q.To.Add(-24 * time.Hour)
	}
	if q.From.After(q.To) {
		return HistoryResult{}, fmt.Errorf("%w: from > to", apperr.ErrValidation)
	}
	if q.Limit <= 0 || q.Limit > maxHistoryLimit {
		q.Limit = defaultHistoryLimit
	}

	if q.Hourly {
		rows, err := s.readings.QueryHourly(ctx, id.String(), q.From, q.To, q.Limit)
		if err != nil {
			return HistoryResult{}, err
		}
		return HistoryResult{Hourly: rows}, nil
	}
	rows, err := s.readings.QueryRaw(ctx, id.String(), q.From, q.To, q.Limit)
	if err != nil {
		return HistoryResult{}, err
	}
	return HistoryResult{Raw: rows}, nil
}

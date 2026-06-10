package usecases

import (
	"context"
	"log/slog"
	"time"

	"github.com/kiryue0/hidrobackend/internal/app/ports"
)

// RetentionService ham telemetriyi belirli gün sayısından sonra siler (Bölüm 6 retention).
type RetentionService struct {
	readings ports.ReadingStore
	days     int
	interval time.Duration
	log      *slog.Logger
}

// NewRetentionService days<=0 ise retention devre dışıdır.
func NewRetentionService(readings ports.ReadingStore, days int, log *slog.Logger) *RetentionService {
	return &RetentionService{
		readings: readings,
		days:     days,
		interval: 6 * time.Hour,
		log:      log,
	}
}

// Run periyodik olarak eski ölçümleri temizler (ctx iptal edilene dek). Bloklar; goroutine'de çağrılır.
func (s *RetentionService) Run(ctx context.Context) {
	if s.days <= 0 {
		s.log.Info("retention devre dışı (READING_RETENTION_DAYS<=0)")
		return
	}
	s.purge(ctx) // başlangıçta bir kez
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.purge(ctx)
		}
	}
}

func (s *RetentionService) purge(ctx context.Context) {
	cutoff := time.Now().AddDate(0, 0, -s.days)
	n, err := s.readings.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		s.log.Error("retention temizliği başarısız", "err", err)
		return
	}
	if n > 0 {
		s.log.Info("retention: eski ölçümler silindi", "count", n, "cutoff", cutoff.Format(time.RFC3339))
	}
}

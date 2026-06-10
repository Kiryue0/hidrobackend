package postgres

import (
	"context"
	"time"

	"github.com/kiryue0/hidrobackend/internal/domain/telemetry"
	"github.com/kiryue0/hidrobackend/internal/infra/postgres/db"
)

// deref nullable float64'ü 0 default ile çözer.
func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// ReadingRepo ports.ReadingStore implementasyonu (time-series).
type ReadingRepo struct {
	q *db.Queries
}

// NewReadingRepo repo üretir.
func NewReadingRepo(q *db.Queries) *ReadingRepo {
	return &ReadingRepo{q: q}
}

// Insert tek bir telemetri ölçümünü ekler.
func (r *ReadingRepo) Insert(ctx context.Context, rd telemetry.Reading) error {
	t, h, tds, ph := rd.Temperature, rd.Humidity, rd.TDS, rd.PH
	return r.q.InsertReading(ctx, db.InsertReadingParams{
		CabinID:     rd.CabinID,
		Ts:          rd.Ts,
		Temperature: &t,
		Humidity:    &h,
		Tds:         &tds,
		Ph:          &ph,
		ShtOk:       rd.Health.SHT,
		RtcOk:       rd.Health.RTC,
		TdsOk:       rd.Health.TDS,
		PhOk:        rd.Health.PH,
	})
}

// readingFrom ortak reading alanlarından domain Reading kurar (sqlc her sorgu
// için ayrı satır tipi ürettiğinden alanlar açıkça geçirilir).
func readingFrom(cabinID string, ts time.Time, t, h, tds, ph *float64, sht, rtc, tdsOk, phOk bool) telemetry.Reading {
	return telemetry.Reading{
		CabinID:     cabinID,
		Ts:          ts,
		Temperature: deref(t),
		Humidity:    deref(h),
		TDS:         deref(tds),
		PH:          deref(ph),
		Health:      telemetry.SensorHealth{SHT: sht, RTC: rtc, TDS: tdsOk, PH: phOk},
	}
}

// QueryRaw ham ölçümleri ts aralığında döner.
func (r *ReadingRepo) QueryRaw(ctx context.Context, cabinID string, from, to time.Time, limit int32) ([]telemetry.Reading, error) {
	rows, err := r.q.GetReadings(ctx, db.GetReadingsParams{CabinID: cabinID, Ts: from, Ts_2: to, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]telemetry.Reading, 0, len(rows))
	for _, row := range rows {
		out = append(out, readingFrom(row.CabinID, row.Ts, row.Temperature, row.Humidity, row.Tds, row.Ph, row.ShtOk, row.RtcOk, row.TdsOk, row.PhOk))
	}
	return out, nil
}

// QueryHourly saatlik ortalamaları döner.
func (r *ReadingRepo) QueryHourly(ctx context.Context, cabinID string, from, to time.Time, limit int32) ([]telemetry.HourlyReading, error) {
	rows, err := r.q.GetReadingsHourly(ctx, db.GetReadingsHourlyParams{CabinID: cabinID, Ts: from, Ts_2: to, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]telemetry.HourlyReading, 0, len(rows))
	for _, row := range rows {
		out = append(out, telemetry.HourlyReading{
			Bucket:      row.Bucket,
			Temperature: row.Temperature,
			Humidity:    row.Humidity,
			TDS:         row.Tds,
			PH:          row.Ph,
			Samples:     row.Samples,
		})
	}
	return out, nil
}

// Latest en son ölçümü döner.
func (r *ReadingRepo) Latest(ctx context.Context, cabinID string) (telemetry.Reading, error) {
	row, err := r.q.GetLatestReading(ctx, cabinID)
	if err != nil {
		return telemetry.Reading{}, mapNotFound(err)
	}
	return readingFrom(row.CabinID, row.Ts, row.Temperature, row.Humidity, row.Tds, row.Ph, row.ShtOk, row.RtcOk, row.TdsOk, row.PhOk), nil
}

// DeleteOlderThan retention: eski ölçümleri siler.
func (r *ReadingRepo) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	return r.q.DeleteReadingsOlderThan(ctx, cutoff)
}

// AlertRepo ports.AlertStore implementasyonu.
type AlertRepo struct {
	q *db.Queries
}

// NewAlertRepo repo üretir.
func NewAlertRepo(q *db.Queries) *AlertRepo {
	return &AlertRepo{q: q}
}

// Insert bir uyarıyı ekler.
func (r *AlertRepo) Insert(ctx context.Context, a telemetry.Alert) error {
	return r.q.InsertAlert(ctx, db.InsertAlertParams{
		CabinID: a.CabinID,
		Ts:      a.Ts,
		Type:    string(a.Type),
		Message: a.Message,
	})
}

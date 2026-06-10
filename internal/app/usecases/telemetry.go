package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
	"github.com/kiryue0/hidrobackend/internal/domain/telemetry"
)

// TelemetryService cihazdan (MQTT up/*) gelen mesajların use case'leri.
type TelemetryService struct {
	cabins    ports.CabinRepository
	readings  ports.ReadingStore
	alerts    ports.AlertStore
	users     ports.UserRepository
	broadcast ports.LiveBroadcastPort
	log       *slog.Logger
}

// NewTelemetryService bağımlılıkları enjekte eder.
func NewTelemetryService(
	cabins ports.CabinRepository,
	readings ports.ReadingStore,
	alerts ports.AlertStore,
	users ports.UserRepository,
	broadcast ports.LiveBroadcastPort,
	log *slog.Logger,
) *TelemetryService {
	return &TelemetryService{
		cabins: cabins, readings: readings, alerts: alerts,
		users: users, broadcast: broadcast, log: log,
	}
}

// IngestReading up/sensors: ensure cabin + time-series'e yaz + yay. Cabin aggregate'ine dokunmaz.
func (s *TelemetryService) IngestReading(ctx context.Context, r telemetry.Reading) error {
	id, err := cabin.NewCabinId(r.CabinID)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	if err := s.cabins.EnsureCabin(ctx, id); err != nil {
		return err
	}
	if err := s.readings.Insert(ctx, r); err != nil {
		return err
	}
	s.broadcast.Broadcast(ports.LiveEvent{Type: "reading", CabinID: r.CabinID, Data: r})
	return nil
}

// ActuatorSnapshot up/state payload'undaki aktüatör durumu anlık görüntüsü.
type ActuatorSnapshot struct {
	Humidifier bool
	HavaMotoru bool
	CobLed     bool
	Fan1       int
	Fan2       int
	Source     string
}

// UpdateActuatorState up/state: cihazdan gelen OTORİTER aktüatör durumunu yazar + yayar.
func (s *TelemetryService) UpdateActuatorState(ctx context.Context, cabinID string, snap ActuatorSnapshot) error {
	id, err := cabin.NewCabinId(cabinID)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	if err := s.cabins.EnsureCabin(ctx, id); err != nil {
		return err
	}

	src := cabin.KomutKaynagi(snap.Source)
	type sp struct {
		t     cabin.ActuatorType
		on    bool
		speed int
	}
	items := []sp{
		{cabin.ActHumidifier, snap.Humidifier, 0},
		{cabin.ActHavaMotoru, snap.HavaMotoru, 0},
		{cabin.ActCobLed, snap.CobLed, 0},
		{cabin.ActFan1, snap.Fan1 > 0, snap.Fan1},
		{cabin.ActFan2, snap.Fan2 > 0, snap.Fan2},
	}

	out := make(map[cabin.ActuatorType]cabin.ActuatorState, len(items))
	for _, it := range items {
		st, verr := cabin.NewActuatorState(it.t, it.on, it.speed, src)
		if verr != nil {
			return fmt.Errorf("%w: %s", apperr.ErrValidation, verr.Error())
		}
		if err := s.cabins.UpsertActuatorState(ctx, id, st); err != nil {
			return err
		}
		out[it.t] = st
	}

	s.broadcast.Broadcast(ports.LiveEvent{Type: "state", CabinID: cabinID, Data: out})
	return nil
}

// RecordHeartbeat up/heartbeat: ensure + online/lastSeen + (gerekirse username ile claim).
func (s *TelemetryService) RecordHeartbeat(ctx context.Context, cabinID, username string, ts time.Time) error {
	id, err := cabin.NewCabinId(cabinID)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	if err := s.cabins.EnsureCabin(ctx, id); err != nil {
		return err
	}
	if err := s.cabins.MarkOnline(ctx, id, ts); err != nil {
		return err
	}
	s.tryClaim(ctx, id, username)
	s.broadcast.Broadcast(ports.LiveEvent{Type: "status", CabinID: cabinID, Data: map[string]any{"online": true}})
	return nil
}

// tryClaim username bir kullanıcıyla eşleşirse ve kabin sahipsizse claim eder (best-effort).
func (s *TelemetryService) tryClaim(ctx context.Context, id cabin.CabinId, username string) {
	if username == "" {
		return
	}
	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		if !errors.Is(err, apperr.ErrNotFound) {
			s.log.Warn("claim: kullanıcı arama hatası", "cabin", id.String(), "err", err)
		}
		return // kullanıcı yoksa sessizce geç
	}
	if err := s.cabins.Claim(ctx, id, u.ID()); err != nil {
		if !errors.Is(err, apperr.ErrConflict) {
			s.log.Warn("claim: atama hatası", "cabin", id.String(), "err", err)
		}
		// ErrConflict = başka sahibe ait; normal, yoksay.
	}
}

// SetCabinStatus up/status (LWT retained): online/offline.
func (s *TelemetryService) SetCabinStatus(ctx context.Context, cabinID string, online bool) error {
	id, err := cabin.NewCabinId(cabinID)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	if err := s.cabins.EnsureCabin(ctx, id); err != nil {
		return err
	}
	if online {
		err = s.cabins.MarkOnline(ctx, id, time.Now())
	} else {
		err = s.cabins.MarkOffline(ctx, id)
	}
	if err != nil {
		return err
	}
	s.broadcast.Broadcast(ports.LiveEvent{Type: "status", CabinID: cabinID, Data: map[string]any{"online": online}})
	return nil
}

// RecordAlert up/alert (opsiyonel): sakla + yay.
func (s *TelemetryService) RecordAlert(ctx context.Context, a telemetry.Alert) error {
	id, err := cabin.NewCabinId(a.CabinID)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	if err := s.cabins.EnsureCabin(ctx, id); err != nil {
		return err
	}
	if err := s.alerts.Insert(ctx, a); err != nil {
		return err
	}
	s.broadcast.Broadcast(ports.LiveEvent{Type: "alert", CabinID: a.CabinID, Data: a})
	return nil
}

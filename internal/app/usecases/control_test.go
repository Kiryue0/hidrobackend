package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
)

type fakeCommandPort struct{ last *ports.ActuatorCommand }

func (f *fakeCommandPort) Send(_ context.Context, _ cabin.CabinId, cmd ports.ActuatorCommand) error {
	f.last = &cmd
	return nil
}

type fakeConfigPort struct {
	t *cabin.Thresholds
	d *cabin.DecisionConfig
}

func (f *fakeConfigPort) Send(_ context.Context, _ cabin.CabinId, t cabin.Thresholds, d cabin.DecisionConfig) error {
	f.t, f.d = &t, &d
	return nil
}

type fakeTestPort struct {
	last    *ports.TestReading
	enabled *bool
}

func (f *fakeTestPort) SendTestReading(_ context.Context, _ cabin.CabinId, r ports.TestReading) error {
	f.last = &r
	return nil
}

func (f *fakeTestPort) SetTestMode(_ context.Context, _ cabin.CabinId, enabled bool) error {
	f.enabled = &enabled
	return nil
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func setupControl(t *testing.T) (*ControlService, *fakeCommandPort, *fakeConfigPort, *fakeCabinRepo) {
	t.Helper()
	repo := newFakeCabinRepo()
	c, _ := cabin.NewCabin(mustCID(t, "CAB-3778C4"), "Salon")
	_ = c.AssignOwner(1)
	repo.store["CAB-3778C4"] = c
	cmd := &fakeCommandPort{}
	cfg := &fakeConfigPort{}
	return NewControlService(repo, cmd, cfg, &fakeTestPort{}), cmd, cfg, repo
}

func TestSendTestReading(t *testing.T) {
	repo := newFakeCabinRepo()
	c, _ := cabin.NewCabin(mustCID(t, "CAB-3778C4"), "Salon")
	_ = c.AssignOwner(1)
	repo.store["CAB-3778C4"] = c
	tp := &fakeTestPort{}
	s := NewControlService(repo, &fakeCommandPort{}, &fakeConfigPort{}, tp)

	in := TestInput{T: 31.5, H: 88, Tds: 1100, Ph: 6.1}
	if err := s.SendTestReading(context.Background(), 1, "CAB-3778C4", in); err != nil {
		t.Fatalf("test reading: %v", err)
	}
	if tp.last == nil || tp.last.T != 31.5 || tp.last.H != 88 {
		t.Fatalf("yayınlanan ölçüm yanlış: %+v", tp.last)
	}

	// geçersiz aralık reddedilir
	if err := s.SendTestReading(context.Background(), 1, "CAB-3778C4", TestInput{T: 99, H: 50, Ph: 6, Tds: 100}); !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("aralık dışı sıcaklık ErrValidation dönmeli, geldi: %v", err)
	}
	// sahip olmayan kullanıcı reddedilir
	if err := s.SendTestReading(context.Background(), 2, "CAB-3778C4", in); !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("yabancı kullanıcı ErrForbidden dönmeli, geldi: %v", err)
	}

	// test modu kapatma cihaza iletilir
	if err := s.SetTestMode(context.Background(), 1, "CAB-3778C4", false); err != nil {
		t.Fatalf("test modu: %v", err)
	}
	if tp.enabled == nil || *tp.enabled {
		t.Fatalf("enabled=false iletilmeli, geldi: %v", tp.enabled)
	}
}

func mustCID(t *testing.T, s string) cabin.CabinId {
	t.Helper()
	id, err := cabin.NewCabinId(s)
	if err != nil {
		t.Fatalf("id: %v", err)
	}
	return id
}

func TestSendCommand_Fan(t *testing.T) {
	s, cmd, _, _ := setupControl(t)
	err := s.SendActuatorCommand(context.Background(), 1, "CAB-3778C4", CommandInput{Actuator: "FAN1", Speed: intPtr(128)})
	if err != nil {
		t.Fatalf("komut: %v", err)
	}
	if cmd.last == nil || !cmd.last.IsFan || cmd.last.Speed != 128 {
		t.Fatalf("fan komutu yanlış: %+v", cmd.last)
	}
}

func TestSendCommand_Role(t *testing.T) {
	s, cmd, _, _ := setupControl(t)
	err := s.SendActuatorCommand(context.Background(), 1, "CAB-3778C4", CommandInput{Actuator: "COB_LED", State: boolPtr(true)})
	if err != nil {
		t.Fatalf("komut: %v", err)
	}
	if cmd.last.IsFan || !cmd.last.State {
		t.Fatalf("röle komutu yanlış: %+v", cmd.last)
	}
}

func TestSendCommand_FanMissingSpeed(t *testing.T) {
	s, _, _, _ := setupControl(t)
	err := s.SendActuatorCommand(context.Background(), 1, "CAB-3778C4", CommandInput{Actuator: "FAN1"})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("fan speed eksik -> validation: %v", err)
	}
}

func TestSendCommand_SpeedOutOfRange(t *testing.T) {
	s, _, _, _ := setupControl(t)
	err := s.SendActuatorCommand(context.Background(), 1, "CAB-3778C4", CommandInput{Actuator: "FAN1", Speed: intPtr(300)})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("speed 300 -> validation: %v", err)
	}
}

func TestSendCommand_NotOwner(t *testing.T) {
	s, _, _, _ := setupControl(t)
	err := s.SendActuatorCommand(context.Background(), 999, "CAB-3778C4", CommandInput{Actuator: "FAN1", Speed: intPtr(100)})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("sahip değil -> forbidden: %v", err)
	}
}

func TestSendCommand_BadActuator(t *testing.T) {
	s, _, _, _ := setupControl(t)
	err := s.SendActuatorCommand(context.Background(), 1, "CAB-3778C4", CommandInput{Actuator: "FAN9", Speed: intPtr(100)})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("geçersiz aktüatör -> validation: %v", err)
	}
}

func TestUpdateConfig_ValidThresholds(t *testing.T) {
	s, _, cfg, _ := setupControl(t)
	thr := cabin.DefaultThresholds()
	thr.SicaklikMax = 27
	_, err := s.UpdateCabinConfig(context.Background(), 1, "CAB-3778C4", ConfigInput{Thresholds: &thr})
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.t == nil || cfg.t.SicaklikMax != 27 {
		t.Fatal("yeni eşik cihaza yollanmalı")
	}
	// decision değişmediği için default kalmalı (merge)
	if cfg.d == nil || !cfg.d.OtomatikMod {
		t.Fatal("decision korunmalı")
	}
}

func TestUpdateConfig_InvalidRejected(t *testing.T) {
	s, _, cfg, _ := setupControl(t)
	bad := cabin.DefaultThresholds()
	bad.SicaklikMin = 100 // min > max
	_, err := s.UpdateCabinConfig(context.Background(), 1, "CAB-3778C4", ConfigInput{Thresholds: &bad})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("geçersiz eşik -> validation: %v", err)
	}
	if cfg.t != nil {
		t.Fatal("geçersiz config cihaza YOLLANMAMALI")
	}
}

func TestUpdateConfig_Empty(t *testing.T) {
	s, _, _, _ := setupControl(t)
	_, err := s.UpdateCabinConfig(context.Background(), 1, "CAB-3778C4", ConfigInput{})
	if !errors.Is(err, apperr.ErrValidation) {
		t.Fatalf("boş config -> validation: %v", err)
	}
}

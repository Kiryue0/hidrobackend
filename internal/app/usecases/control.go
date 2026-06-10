package usecases

import (
	"context"
	"fmt"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
)

// ControlService UI'den gelen manuel komut + config güncelleme use case'leri.
type ControlService struct {
	cabins   ports.CabinRepository
	commands ports.ActuatorCommandPort
	config   ports.CabinConfigPort
	test     ports.TestTelemetryPort
}

// NewControlService bağımlılıkları enjekte eder.
func NewControlService(cabins ports.CabinRepository, commands ports.ActuatorCommandPort, config ports.CabinConfigPort, test ports.TestTelemetryPort) *ControlService {
	return &ControlService{cabins: cabins, commands: commands, config: config, test: test}
}

// ownedCabin sahiplik kontrolüyle kabini yükler.
func (s *ControlService) ownedCabin(ctx context.Context, ownerID int64, cabinID string) (*cabin.Cabin, error) {
	id, err := cabin.NewCabinId(cabinID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	c, err := s.cabins.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !c.IsOwnedBy(ownerID) {
		return nil, apperr.ErrForbidden
	}
	return c, nil
}

// CommandInput UI'den gelen aktüatör komut girdisi (state ya da speed verilir).
type CommandInput struct {
	Actuator string
	State    *bool
	Speed    *int
}

// SendActuatorCommand manuel komutu doğrular ve cihaza yollar.
// DB'yi iyimser GÜNCELLEMEZ; gerçek durum cihazın up/state'iyle gelir (Bölüm 10/3).
func (s *ControlService) SendActuatorCommand(ctx context.Context, ownerID int64, cabinID string, in CommandInput) error {
	c, err := s.ownedCabin(ctx, ownerID, cabinID)
	if err != nil {
		return err
	}

	at, err := cabin.ParseActuatorType(in.Actuator)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}

	cmd := ports.ActuatorCommand{Actuator: at, IsFan: at.IsFan()}
	if at.IsFan() {
		if in.Speed == nil {
			return fmt.Errorf("%w: fan için speed gerekli", apperr.ErrValidation)
		}
		if *in.Speed < 0 || *in.Speed > 255 {
			return fmt.Errorf("%w: speed 0..255 olmalı", apperr.ErrValidation)
		}
		cmd.Speed = *in.Speed
	} else {
		if in.State == nil {
			return fmt.Errorf("%w: röle için state gerekli", apperr.ErrValidation)
		}
		cmd.State = *in.State
	}

	return s.commands.Send(ctx, c.ID(), cmd)
}

// ConfigInput UI'den gelen config güncellemesi (kısmi: thresholds ve/veya decision).
type ConfigInput struct {
	Thresholds *cabin.Thresholds
	Decision   *cabin.DecisionConfig
}

// UpdateCabinConfig eşik/karar günceller: VO doğrula -> persist -> cihaza yolla.
func (s *ControlService) UpdateCabinConfig(ctx context.Context, ownerID int64, cabinID string, in ConfigInput) (*cabin.Cabin, error) {
	if in.Thresholds == nil && in.Decision == nil {
		return nil, fmt.Errorf("%w: en az bir alan (thresholds/decision) gerekli", apperr.ErrValidation)
	}
	c, err := s.ownedCabin(ctx, ownerID, cabinID)
	if err != nil {
		return nil, err
	}

	if in.Thresholds != nil {
		if err := c.UpdateThresholds(*in.Thresholds); err != nil {
			return nil, fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
		}
	}
	if in.Decision != nil {
		if err := c.UpdateDecisionConfig(*in.Decision); err != nil {
			return nil, fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
		}
	}

	if err := s.cabins.UpdateConfig(ctx, c.ID(), c.Thresholds(), c.Decision()); err != nil {
		return nil, err
	}
	if err := s.config.Send(ctx, c.ID(), c.Thresholds(), c.Decision()); err != nil {
		return nil, err
	}
	return c, nil
}

// TestInput UI test modunda girilen sahte ölçüm.
type TestInput struct {
	T, H, Tds, Ph float64
}

// SendTestReading sahte ölçümü doğrular ve normal telemetri hattına enjekte eder.
// Veri DB'ye yazılır ve WS ile yayılır; cihaz down/test'ten görebilir.
func (s *ControlService) SendTestReading(ctx context.Context, ownerID int64, cabinID string, in TestInput) error {
	c, err := s.ownedCabin(ctx, ownerID, cabinID)
	if err != nil {
		return err
	}
	if in.T < -20 || in.T > 60 {
		return fmt.Errorf("%w: sıcaklık -20..60 °C aralığında olmalı", apperr.ErrValidation)
	}
	if in.H < 0 || in.H > 100 {
		return fmt.Errorf("%w: nem 0..100 aralığında olmalı", apperr.ErrValidation)
	}
	if in.Ph < 0 || in.Ph > 14 {
		return fmt.Errorf("%w: pH 0..14 aralığında olmalı", apperr.ErrValidation)
	}
	if in.Tds < 0 || in.Tds > 5000 {
		return fmt.Errorf("%w: TDS 0..5000 aralığında olmalı", apperr.ErrValidation)
	}
	return s.test.SendTestReading(ctx, c.ID(), ports.TestReading{T: in.T, H: in.H, Tds: in.Tds, Ph: in.Ph})
}

// SetTestMode test modunu açıp kapatır; kapanışta cihaza normal moda dön bildirimi gider.
func (s *ControlService) SetTestMode(ctx context.Context, ownerID int64, cabinID string, enabled bool) error {
	c, err := s.ownedCabin(ctx, ownerID, cabinID)
	if err != nil {
		return err
	}
	return s.test.SetTestMode(ctx, c.ID(), enabled)
}

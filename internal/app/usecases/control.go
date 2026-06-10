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
}

// NewControlService bağımlılıkları enjekte eder.
func NewControlService(cabins ports.CabinRepository, commands ports.ActuatorCommandPort, config ports.CabinConfigPort) *ControlService {
	return &ControlService{cabins: cabins, commands: commands, config: config}
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

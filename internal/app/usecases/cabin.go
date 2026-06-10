package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
)

// CabinService kabin CRUD + claim use case'lerini sağlar.
type CabinService struct {
	cabins ports.CabinRepository
}

// NewCabinService bağımlılıkları enjekte eder.
func NewCabinService(cabins ports.CabinRepository) *CabinService {
	return &CabinService{cabins: cabins}
}

// Create kullanıcıya ait yeni bir kabin oluşturur (manuel).
func (s *CabinService) Create(ctx context.Context, ownerID int64, cabinID, name string) (*cabin.Cabin, error) {
	id, err := cabin.NewCabinId(cabinID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	exists, err := s.cabins.Exists(ctx, id)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.ErrConflict
	}
	c, err := cabin.NewCabin(id, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}
	if err := c.AssignOwner(ownerID); err != nil {
		return nil, err
	}
	if err := s.cabins.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Claim kabini kullanıcıya bağlar. Kabin yoksa oluşturulur (sahipli), varsa
// ve sahipsizse atanır; başka kullanıcıya aitse ErrConflict döner.
func (s *CabinService) Claim(ctx context.Context, ownerID int64, cabinID string) (*cabin.Cabin, error) {
	id, err := cabin.NewCabinId(cabinID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}

	existing, err := s.cabins.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			// Kabin yok: sahipli oluştur.
			c, nerr := cabin.NewCabin(id, "")
			if nerr != nil {
				return nil, fmt.Errorf("%w: %s", apperr.ErrValidation, nerr.Error())
			}
			_ = c.AssignOwner(ownerID)
			if cerr := s.cabins.Create(ctx, c); cerr != nil {
				return nil, cerr
			}
			return c, nil
		}
		return nil, err
	}

	// Kabin var: sahiplik durumunu kontrol et.
	if existing.IsOwnedBy(ownerID) {
		return existing, nil // idempotent
	}
	if existing.OwnerUserID() != nil {
		return nil, apperr.ErrConflict // başka kullanıcıya ait
	}
	// Sahipsiz: koşullu ata (yarış durumunu repo halleder).
	if err := s.cabins.Claim(ctx, id, ownerID); err != nil {
		return nil, err
	}
	return s.cabins.GetByID(ctx, id)
}

// Get kabini sahiplik kontrolüyle döner (config + aktüatör durumu dahil).
func (s *CabinService) Get(ctx context.Context, ownerID int64, cabinID string) (*cabin.Cabin, error) {
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

// List kullanıcının kabinlerini döner.
func (s *CabinService) List(ctx context.Context, ownerID int64) ([]*cabin.Cabin, error) {
	return s.cabins.ListByOwner(ctx, ownerID)
}

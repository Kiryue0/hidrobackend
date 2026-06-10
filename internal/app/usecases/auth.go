// Package usecases: application inbound port'ları (use case implementasyonları).
package usecases

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/user"
)

// AuthService kayıt ve giriş use case'lerini sağlar.
type AuthService struct {
	users  ports.UserRepository
	cabins ports.CabinRepository
	hasher ports.PasswordHasher
	tokens ports.TokenIssuer
}

// NewAuthService bağımlılıkları enjekte eder.
func NewAuthService(users ports.UserRepository, cabins ports.CabinRepository, hasher ports.PasswordHasher, tokens ports.TokenIssuer) *AuthService {
	return &AuthService{users: users, cabins: cabins, hasher: hasher, tokens: tokens}
}

// RegisterInput kayıt girdisi.
type RegisterInput struct {
	Username string
	Email    string
	Password string
}

// Register yeni kullanıcı oluşturur. Kullanıcı adı/e-posta benzersiz olmalıdır.
func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*user.User, error) {
	if len(in.Password) < 8 {
		return nil, fmt.Errorf("%w: parola en az 8 karakter olmalı", apperr.ErrValidation)
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, fmt.Errorf("parola hash: %w", err)
	}

	u, err := user.NewUser(in.Username, in.Email, hash)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", apperr.ErrValidation, err.Error())
	}

	if err := s.users.Create(ctx, u); err != nil {
		return nil, err // repo ErrConflict döndürebilir
	}
	return u, nil
}

// LoginInput giriş girdisi: kullanıcı adı veya e-posta + parola.
type LoginInput struct {
	Identifier string // username veya email
	Password   string
}

// Login kimlik bilgilerini doğrular ve JWT access token döner.
func (s *AuthService) Login(ctx context.Context, in LoginInput) (string, error) {
	id := strings.TrimSpace(in.Identifier)

	var (
		u   *user.User
		err error
	)
	if strings.Contains(id, "@") {
		u, err = s.users.GetByEmail(ctx, strings.ToLower(id))
	} else {
		u, err = s.users.GetByUsername(ctx, id)
	}
	if err != nil {
		if errors.Is(err, apperr.ErrNotFound) {
			// Kullanıcı yokluğunu sızdırma: aynı hatayı dön.
			return "", apperr.ErrInvalidCredentials
		}
		return "", err
	}

	if !s.hasher.Compare(u.PasswordHash(), in.Password) {
		return "", apperr.ErrInvalidCredentials
	}

	token, err := s.tokens.Issue(u.ID())
	if err != nil {
		return "", fmt.Errorf("token üretimi: %w", err)
	}
	return token, nil
}

// DeleteAccount kullanıcının tüm kabinlerini cascade ile siler, ardından kullanıcıyı siler.
func (s *AuthService) DeleteAccount(ctx context.Context, userID int64) error {
	// Önce kabinleri sil (cascade: cabin_config, actuator_state, readings, alerts)
	if _, err := s.cabins.DeleteCabinsByOwner(ctx, userID); err != nil {
		return fmt.Errorf("kabin silme: %w", err)
	}

	if _, err := s.users.Delete(ctx, userID); err != nil {
		return fmt.Errorf("kullanıcı silme: %w", err)
	}
	return nil
}

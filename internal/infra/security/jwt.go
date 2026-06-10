package security

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTService ports.TokenIssuer implementasyonu (HS256).
type JWTService struct {
	secret []byte
	ttl    time.Duration
}

// NewJWTService secret ve token ömrüyle servis üretir.
func NewJWTService(secret string, ttl time.Duration) *JWTService {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &JWTService{secret: []byte(secret), ttl: ttl}
}

// Issue verilen kullanıcı için imzalı access token üretir.
func (s *JWTService) Issue(userID int64) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.secret)
}

// Parse token'ı doğrular (imza + süre) ve userID döner.
func (s *JWTService) Parse(token string) (int64, error) {
	parsed, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("beklenmeyen imza metodu")
			}
			return s.secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		return 0, err
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid {
		return 0, errors.New("geçersiz token")
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, errors.New("geçersiz token subject")
	}
	return id, nil
}

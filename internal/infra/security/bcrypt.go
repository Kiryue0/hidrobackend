// Package security: bcrypt parola hasher + JWT token servisi (outbound adapter'lar).
package security

import "golang.org/x/crypto/bcrypt"

// BcryptHasher ports.PasswordHasher implementasyonu.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher verilen cost ile hasher üretir (0 -> bcrypt.DefaultCost).
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

// Hash parolayı bcrypt ile hash'ler.
func (h *BcryptHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Compare hash ile parolayı karşılaştırır.
func (h *BcryptHasher) Compare(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

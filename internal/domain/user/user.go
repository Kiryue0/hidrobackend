// Package user: Identity bounded context — User aggregate root.
// Saf Go; dış bağımlılık yoktur.
package user

import (
	"errors"
	"regexp"
	"strings"
)

var (
	emailPattern    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)
)

// User aggregate root'tur: kimlik, kimlik bilgileri ve sahip olunan kabinler.
type User struct {
	id           int64
	username     string
	email        string
	passwordHash string
}

// NewUser yeni bir kullanıcı oluşturur (kayıt). passwordHash önceden hesaplanmış olmalıdır.
func NewUser(username, email, passwordHash string) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))

	if !usernamePattern.MatchString(username) {
		return nil, errors.New("geçersiz kullanıcı adı (3-32, harf/rakam/._-)")
	}
	if !emailPattern.MatchString(email) {
		return nil, errors.New("geçersiz e-posta")
	}
	if passwordHash == "" {
		return nil, errors.New("parola hash'i zorunlu")
	}
	return &User{username: username, email: email, passwordHash: passwordHash}, nil
}

// Reconstruct repo'nun DB'den User'ı yeniden kurması için (doğrulama yapılmaz).
func Reconstruct(id int64, username, email, passwordHash string) *User {
	return &User{id: id, username: username, email: email, passwordHash: passwordHash}
}

func (u *User) ID() int64            { return u.id }
func (u *User) Username() string     { return u.username }
func (u *User) Email() string        { return u.email }
func (u *User) PasswordHash() string { return u.passwordHash }

// SetID repo insert sonrası üretilen kimliği atar.
func (u *User) SetID(id int64) { u.id = id }

// ValidateUsername dışarıdan (claim eşleşmesi vb.) kullanım için username doğrular.
func ValidateUsername(s string) bool {
	return usernamePattern.MatchString(strings.TrimSpace(s))
}

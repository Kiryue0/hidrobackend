// Package apperr: application katmanı sentinel hataları.
// Infra (HTTP) bunları uygun durum kodlarına eşler.
package apperr

import "errors"

var (
	// ErrNotFound: kayıt bulunamadı.
	ErrNotFound = errors.New("kayıt bulunamadı")
	// ErrConflict: benzersizlik/durum çakışması (örn. kullanıcı adı alınmış).
	ErrConflict = errors.New("çakışma")
	// ErrInvalidCredentials: kullanıcı adı/parola hatalı.
	ErrInvalidCredentials = errors.New("kimlik bilgileri hatalı")
	// ErrValidation: girdi doğrulama hatası (domain/VO veya use case).
	ErrValidation = errors.New("doğrulama hatası")
	// ErrForbidden: yetki yok (kabin sahibi değil vb.).
	ErrForbidden = errors.New("yetkisiz")
)

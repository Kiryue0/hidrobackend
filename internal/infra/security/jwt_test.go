package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWT_IssueParse_RoundTrip(t *testing.T) {
	svc := NewJWTService("super-secret", time.Hour)
	tok, err := svc.Issue(42)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	id, err := svc.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if id != 42 {
		t.Fatalf("userID beklenen 42, gelen %d", id)
	}
}

func TestJWT_RejectsExpired(t *testing.T) {
	// NewJWTService negatif/0 TTL'i 24h'e çevirdiği için geçmiş exp'li
	// token'ı elle üretiyoruz.
	secret := "super-secret"
	svc := NewJWTService(secret, time.Hour)
	claims := jwt.RegisteredClaims{
		Subject:   "7",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := svc.Parse(tok); err == nil {
		t.Fatal("süresi geçmiş token reddedilmeliydi")
	}
}

func TestJWT_RejectsWrongSecret(t *testing.T) {
	issuer := NewJWTService("secret-A", time.Hour)
	tok, _ := issuer.Issue(1)

	verifier := NewJWTService("secret-B", time.Hour)
	if _, err := verifier.Parse(tok); err == nil {
		t.Fatal("yanlış secret ile imzalanan token reddedilmeliydi")
	}
}

// TestJWT_RejectsAlgNone: alg=none confusion saldırısı reddedilmeli.
func TestJWT_RejectsAlgNone(t *testing.T) {
	svc := NewJWTService("super-secret", time.Hour)

	claims := jwt.RegisteredClaims{
		Subject:   "5",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("none-sign: %v", err)
	}
	if _, err := svc.Parse(tok); err == nil {
		t.Fatal("alg=none token reddedilmeliydi (confusion saldırısı)")
	}
}

// TestJWT_RejectsNonHMACAlg: RS256 başlığı taşıyan ama (saldırganın secret
// sandığı veriyle) HMAC ile imzalanmış token reddedilmeli — klasik
// RS256->HS256 algoritma confusion denemesi. WithValidMethods + keyfunc'taki
// *SigningMethodHMAC tip kontrolü bunu engellemeli.
func TestJWT_RejectsNonHMACAlg(t *testing.T) {
	secret := "super-secret"
	svc := NewJWTService(secret, time.Hour)

	b64 := func(s string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(s))
	}
	header := b64(`{"alg":"RS256","typ":"JWT"}`)
	payload := b64(`{"sub":"9","exp":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`)
	signing := header + "." + payload

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	tok := signing + "." + sig

	if _, err := svc.Parse(tok); err == nil {
		t.Fatal("RS256 başlıklı token reddedilmeliydi (alg confusion)")
	}
}

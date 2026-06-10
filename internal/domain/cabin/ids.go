// Package cabin: Cabin bounded context — aggregate, value object'ler ve invariant'lar.
// Saf Go; dış bağımlılık yoktur.
package cabin

import (
	"errors"
	"regexp"
)

// cabinIDPattern: "CAB-" + MAC son 6 hex (büyük harf). Örn: CAB-3778C4.
var cabinIDPattern = regexp.MustCompile(`^CAB-[0-9A-F]{6}$`)

// CabinId kabin kimliğini doğrulanmış biçimde temsil eder.
type CabinId struct {
	value string
}

// NewCabinId verilen string'i doğrular ve CabinId üretir.
func NewCabinId(s string) (CabinId, error) {
	if !cabinIDPattern.MatchString(s) {
		return CabinId{}, errors.New("geçersiz kabin_id: 'CAB-XXXXXX' (6 büyük hex) bekleniyor")
	}
	return CabinId{value: s}, nil
}

// String kimlik değerini döner.
func (c CabinId) String() string { return c.value }

// IsZero kimliğin boş olup olmadığını söyler.
func (c CabinId) IsZero() bool { return c.value == "" }

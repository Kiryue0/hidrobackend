package http

import (
	"testing"
)

// parseUnix kenar durumları: boş/sıfır/negatif/geçersiz -> zero time;
// geçerli pozitif saniye -> doğru time.Time.
func TestParseUnix(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantZero bool
		wantSec  int64
	}{
		{"bos", "", true, 0},
		{"sifir", "0", true, 0},
		{"negatif", "-5", true, 0},
		{"gecersiz", "abc", true, 0},
		{"ondalik", "12.5", true, 0},
		{"gecerli", "1700000000", false, 1700000000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseUnix(c.in)
			if c.wantZero {
				if !got.IsZero() {
					t.Fatalf("%q: zero bekleniyordu, %v", c.in, got)
				}
				return
			}
			if got.IsZero() {
				t.Fatalf("%q: zero olmamalı", c.in)
			}
			if got.Unix() != c.wantSec {
				t.Fatalf("%q: unix=%d, beklenen %d", c.in, got.Unix(), c.wantSec)
			}
		})
	}
}

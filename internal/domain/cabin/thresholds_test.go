package cabin

import "testing"

func TestNewThresholds_Default_OK(t *testing.T) {
	if _, err := NewThresholds(DefaultThresholds()); err != nil {
		t.Fatalf("default thresholds geçerli olmalı: %v", err)
	}
}

func TestNewThresholds_Invalid(t *testing.T) {
	base := DefaultThresholds()

	cases := map[string]func(Thresholds) Thresholds{
		"sıcaklık min>=max":   func(x Thresholds) Thresholds { x.SicaklikMin = 30; x.SicaklikMax = 28; return x },
		"sıcaklık max>kritik": func(x Thresholds) Thresholds { x.SicaklikKritikMax = 27; return x },
		"sıcaklık fiziksel":   func(x Thresholds) Thresholds { x.SicaklikKritikMax = 80; return x },
		"nem >100":            func(x Thresholds) Thresholds { x.NemKritikMax = 120; return x },
		"nem min>=max":        func(x Thresholds) Thresholds { x.NemMin = 80; x.NemMax = 70; return x },
		"tds negatif":         func(x Thresholds) Thresholds { x.TdsMin = -1; return x },
		"tds min>=max":        func(x Thresholds) Thresholds { x.TdsMin = 2000; x.TdsMax = 1000; return x },
		"ph aralık":           func(x Thresholds) Thresholds { x.PhMax = 15; return x },
		"ph min>=max":         func(x Thresholds) Thresholds { x.PhMin = 7; x.PhMax = 6; return x },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewThresholds(mut(base)); err == nil {
				t.Fatalf("hata bekleniyordu (%s)", name)
			}
		})
	}
}

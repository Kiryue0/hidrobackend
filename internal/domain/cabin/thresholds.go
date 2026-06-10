package cabin

import "fmt"

// Mutlak fiziksel sınırlar (backend güvenlik katmanı; cihazın da kendi default'u var).
const (
	absTempMin = -20.0
	absTempMax = 60.0
	absHumMin  = 0.0
	absHumMax  = 100.0
	absPhMin   = 0.0
	absPhMax   = 14.0
)

// Thresholds firmware EsikConfig aynasıdır (immutable value object).
// JSON etiketleri Bölüm 2.3 down/config kontratıyla birebir eşleşir.
type Thresholds struct {
	SicaklikMin       float64 `json:"sicaklikMin"`
	SicaklikMax       float64 `json:"sicaklikMax"`
	SicaklikKritikMax float64 `json:"sicaklikKritikMax"`
	NemMin            float64 `json:"nemMin"`
	NemMax            float64 `json:"nemMax"`
	NemKritikMax      float64 `json:"nemKritikMax"`
	TdsMin            float64 `json:"tdsMin"`
	TdsMax            float64 `json:"tdsMax"`
	PhMin             float64 `json:"phMin"`
	PhMax             float64 `json:"phMax"`
}

// NewThresholds eşik değerlerini doğrular ve geçerliyse value object döner.
func NewThresholds(t Thresholds) (Thresholds, error) {
	if err := t.validate(); err != nil {
		return Thresholds{}, err
	}
	return t, nil
}

func (t Thresholds) validate() error {
	// Sıcaklık
	if t.SicaklikMin < absTempMin || t.SicaklikKritikMax > absTempMax {
		return fmt.Errorf("sıcaklık fiziksel sınır dışı (%.1f..%.1f)", absTempMin, absTempMax)
	}
	if !(t.SicaklikMin < t.SicaklikMax && t.SicaklikMax <= t.SicaklikKritikMax) {
		return fmt.Errorf("sıcaklık tutarsız: min < max <= kritikMax olmalı")
	}
	// Nem
	if t.NemMin < absHumMin || t.NemKritikMax > absHumMax {
		return fmt.Errorf("nem fiziksel sınır dışı (%.0f..%.0f)", absHumMin, absHumMax)
	}
	if !(t.NemMin < t.NemMax && t.NemMax <= t.NemKritikMax) {
		return fmt.Errorf("nem tutarsız: min < max <= kritikMax olmalı")
	}
	// TDS
	if t.TdsMin < 0 {
		return fmt.Errorf("tdsMin negatif olamaz")
	}
	if !(t.TdsMin < t.TdsMax) {
		return fmt.Errorf("tds tutarsız: min < max olmalı")
	}
	// pH
	if t.PhMin < absPhMin || t.PhMax > absPhMax {
		return fmt.Errorf("pH fiziksel sınır dışı (%.0f..%.0f)", absPhMin, absPhMax)
	}
	if !(t.PhMin < t.PhMax) {
		return fmt.Errorf("pH tutarsız: min < max olmalı")
	}
	return nil
}

// DefaultThresholds makul başlangıç değerleri (Bölüm 2.3 örneğiyle uyumlu).
func DefaultThresholds() Thresholds {
	return Thresholds{
		SicaklikMin: 18, SicaklikMax: 28, SicaklikKritikMax: 32,
		NemMin: 55, NemMax: 75, NemKritikMax: 85,
		TdsMin: 800, TdsMax: 1800,
		PhMin: 5.5, PhMax: 6.5,
	}
}

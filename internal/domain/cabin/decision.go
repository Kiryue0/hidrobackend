package cabin

import "fmt"

// HavaMotoruMod: su havalandırma pompası çalışma modu.
const (
	HavaMotoruManuel  = 0
	HavaMotoruSurekli = 1
	HavaMotoruDongu   = 2
)

// DecisionConfig firmware KararConfig aynasıdır (immutable value object).
// Karar motoru CİHAZDA çalışır; bu yalnızca parametre taşır.
type DecisionConfig struct {
	OtomatikMod   bool    `json:"otomatikMod"`
	FanOrtaEsik   float64 `json:"fanOrtaEsik"`
	FanTamEsik    float64 `json:"fanTamEsik"`
	LedKisEsik    float64 `json:"ledKisEsik"`
	LedKapatEsik  float64 `json:"ledKapatEsik"`
	FanBazHiz     int     `json:"fanBazHiz"`
	FanOrtaHiz    int     `json:"fanOrtaHiz"`
	FanTamHiz     int     `json:"fanTamHiz"`
	LedKisikDuty  int     `json:"ledKisikDuty"`
	HisterezisC   float64 `json:"histerezisC"`
	HisterezisNem float64 `json:"histerezisNem"`
	HavaMotoruMod int     `json:"havaMotoruMod"`
	HavaAcikSn    int     `json:"havaAcikSn"`
	HavaKapaliSn  int     `json:"havaKapaliSn"`
}

// NewDecisionConfig karar parametrelerini doğrular.
func NewDecisionConfig(d DecisionConfig) (DecisionConfig, error) {
	if err := d.validate(); err != nil {
		return DecisionConfig{}, err
	}
	return d, nil
}

func validPWM(v int) bool { return v >= 0 && v <= 255 }

func (d DecisionConfig) validate() error {
	for name, v := range map[string]int{
		"fanBazHiz": d.FanBazHiz, "fanOrtaHiz": d.FanOrtaHiz,
		"fanTamHiz": d.FanTamHiz, "ledKisikDuty": d.LedKisikDuty,
	} {
		if !validPWM(v) {
			return fmt.Errorf("%s 0..255 aralığında olmalı", name)
		}
	}
	// Sıcaklık eşik sıralaması (fan kademeli, LED kıs/kapat)
	if !(d.FanOrtaEsik <= d.FanTamEsik) {
		return fmt.Errorf("fanOrtaEsik <= fanTamEsik olmalı")
	}
	if !(d.LedKisEsik <= d.LedKapatEsik) {
		return fmt.Errorf("ledKisEsik <= ledKapatEsik olmalı")
	}
	// Fan hızları kademeli artmalı
	if !(d.FanBazHiz <= d.FanOrtaHiz && d.FanOrtaHiz <= d.FanTamHiz) {
		return fmt.Errorf("fan hızları kademeli olmalı: baz <= orta <= tam")
	}
	if d.HisterezisC < 0 || d.HisterezisNem < 0 {
		return fmt.Errorf("histerezis negatif olamaz")
	}
	switch d.HavaMotoruMod {
	case HavaMotoruManuel, HavaMotoruSurekli, HavaMotoruDongu:
	default:
		return fmt.Errorf("havaMotoruMod 0|1|2 olmalı")
	}
	if d.HavaAcikSn < 0 || d.HavaKapaliSn < 0 {
		return fmt.Errorf("hava süreleri negatif olamaz")
	}
	return nil
}

// DefaultDecisionConfig makul başlangıç değerleri (Bölüm 2.3 örneğiyle uyumlu).
func DefaultDecisionConfig() DecisionConfig {
	return DecisionConfig{
		OtomatikMod: true,
		FanOrtaEsik: 28, FanTamEsik: 30, LedKisEsik: 32, LedKapatEsik: 34,
		FanBazHiz: 80, FanOrtaHiz: 128, FanTamHiz: 255, LedKisikDuty: 128,
		HisterezisC: 0.5, HisterezisNem: 2.0,
		HavaMotoruMod: HavaMotoruSurekli, HavaAcikSn: 900, HavaKapaliSn: 900,
	}
}

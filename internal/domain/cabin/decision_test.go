package cabin

import "testing"

func TestNewDecisionConfig_Default_OK(t *testing.T) {
	if _, err := NewDecisionConfig(DefaultDecisionConfig()); err != nil {
		t.Fatalf("default decision geçerli olmalı: %v", err)
	}
}

func TestNewDecisionConfig_Invalid(t *testing.T) {
	base := DefaultDecisionConfig()
	cases := map[string]func(DecisionConfig) DecisionConfig{
		"pwm aralık":        func(x DecisionConfig) DecisionConfig { x.FanTamHiz = 300; return x },
		"fan kademe":        func(x DecisionConfig) DecisionConfig { x.FanBazHiz = 200; x.FanOrtaHiz = 100; return x },
		"fan eşik sıra":     func(x DecisionConfig) DecisionConfig { x.FanOrtaEsik = 35; x.FanTamEsik = 30; return x },
		"led eşik sıra":     func(x DecisionConfig) DecisionConfig { x.LedKisEsik = 40; x.LedKapatEsik = 34; return x },
		"histerezis neg":    func(x DecisionConfig) DecisionConfig { x.HisterezisC = -1; return x },
		"hava mod geçersiz": func(x DecisionConfig) DecisionConfig { x.HavaMotoruMod = 5; return x },
		"hava süre neg":     func(x DecisionConfig) DecisionConfig { x.HavaAcikSn = -10; return x },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewDecisionConfig(mut(base)); err == nil {
				t.Fatalf("hata bekleniyordu (%s)", name)
			}
		})
	}
}

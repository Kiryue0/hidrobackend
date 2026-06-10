package cabin

import "testing"

func TestParseActuatorType(t *testing.T) {
	for _, s := range []string{"HUMIDIFIER", "HAVA_MOTORU", "COB_LED", "FAN1", "FAN2"} {
		if _, err := ParseActuatorType(s); err != nil {
			t.Errorf("%q geçerli olmalı", s)
		}
	}
	if _, err := ParseActuatorType("FAN3"); err == nil {
		t.Error("FAN3 geçersiz olmalı")
	}
}

func TestNewActuatorState_Fan(t *testing.T) {
	if _, err := NewActuatorState(ActFan1, true, 300, KaynakBackend); err == nil {
		t.Fatal("hız>255 reddedilmeli")
	}
	s, err := NewActuatorState(ActFan2, true, 0, KaynakBackend)
	if err != nil {
		t.Fatalf("geçerli olmalı: %v", err)
	}
	if s.On {
		t.Fatal("fan speed=0 ise On=false türetilmeli")
	}
}

func TestNewActuatorState_Role(t *testing.T) {
	s, err := NewActuatorState(ActCobLed, true, 99, KaynakButton)
	if err != nil {
		t.Fatalf("geçerli olmalı: %v", err)
	}
	if s.Speed != 0 {
		t.Fatal("röle aktüatörde speed sıfırlanmalı")
	}
}

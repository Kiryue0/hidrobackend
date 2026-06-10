package cabin

import "fmt"

// ActuatorType firmware AktuatorTipi aynasıdır.
type ActuatorType string

const (
	ActHumidifier ActuatorType = "HUMIDIFIER"
	ActHavaMotoru ActuatorType = "HAVA_MOTORU" // su pompası (besin suyuna oksijen)
	ActCobLed     ActuatorType = "COB_LED"
	ActFan1       ActuatorType = "FAN1"
	ActFan2       ActuatorType = "FAN2"
)

// roleActuators on/off; fanActuators ise hız (0..255) taşır.
var roleActuators = map[ActuatorType]bool{
	ActHumidifier: true, ActHavaMotoru: true, ActCobLed: true,
}
var fanActuators = map[ActuatorType]bool{
	ActFan1: true, ActFan2: true,
}

// ParseActuatorType string'i doğrulayıp ActuatorType döner.
func ParseActuatorType(s string) (ActuatorType, error) {
	a := ActuatorType(s)
	if roleActuators[a] || fanActuators[a] {
		return a, nil
	}
	return "", fmt.Errorf("geçersiz aktüatör tipi: %q", s)
}

// IsFan aktüatörün hız taşıyan bir fan olup olmadığını söyler.
func (a ActuatorType) IsFan() bool { return fanActuators[a] }

// KomutKaynagi: aktüatör durumunun nereden geldiği (firmware KomutKaynagi).
type KomutKaynagi string

const (
	KaynakButton   KomutKaynagi = "button"
	KaynakSerial   KomutKaynagi = "serial"
	KaynakDecision KomutKaynagi = "decision"
	KaynakBackend  KomutKaynagi = "backend"
)

// ActuatorState bir aktüatörün güncel durumu (value object).
// Röleler için On; fanlar için Speed (0..255) anlamlıdır.
// JSON etiketleri WebSocket yayın formatı içindir.
type ActuatorState struct {
	Type   ActuatorType `json:"type"`
	On     bool         `json:"on"`
	Speed  int          `json:"speed"`
	Source KomutKaynagi `json:"source"`
}

// NewActuatorState durumu doğrular. Fanlar için Speed>0 -> On=true türetilir.
func NewActuatorState(t ActuatorType, on bool, speed int, source KomutKaynagi) (ActuatorState, error) {
	if !roleActuators[t] && !fanActuators[t] {
		return ActuatorState{}, fmt.Errorf("geçersiz aktüatör tipi: %q", t)
	}
	if t.IsFan() {
		if speed < 0 || speed > 255 {
			return ActuatorState{}, fmt.Errorf("fan hızı 0..255 olmalı")
		}
		on = speed > 0
	} else {
		speed = 0
	}
	return ActuatorState{Type: t, On: on, Speed: speed, Source: source}, nil
}

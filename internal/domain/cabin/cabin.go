package cabin

import (
	"errors"
	"time"
)

// SensorType kabinde bulunması zorunlu sensör tipleri.
type SensorType string

const (
	SensorTemperature SensorType = "TEMPERATURE"
	SensorHumidity    SensorType = "HUMIDITY"
	SensorTDS         SensorType = "TDS"
	SensorPH          SensorType = "PH"
)

// requiredSensors: geçerli bir kabinde tanımlı olması zorunlu sensör seti (invariant).
var requiredSensors = []SensorType{SensorTemperature, SensorHumidity, SensorTDS, SensorPH}

// Cabin aggregate root'tur: kimlik, sahiplik, sensör/aktüatör tanımları,
// güncel aktüatör durumu ve config (thresholds + decision).
type Cabin struct {
	id          CabinId
	ownerUserID *int64 // claim'e kadar nil
	name        string
	sensors     []SensorType
	actuators   map[ActuatorType]ActuatorState
	thresholds  Thresholds
	decision    DecisionConfig
	online      bool
	lastSeen    *time.Time
}

// NewCabin yeni bir kabin oluşturur (standart sensör seti + default config ile).
// Invariant: zorunlu sensör seti tanımlı olmalıdır.
func NewCabin(id CabinId, name string) (*Cabin, error) {
	if id.IsZero() {
		return nil, errors.New("kabin_id zorunlu")
	}
	c := &Cabin{
		id:         id,
		name:       name,
		sensors:    append([]SensorType(nil), requiredSensors...),
		actuators:  make(map[ActuatorType]ActuatorState),
		thresholds: DefaultThresholds(),
		decision:   DefaultDecisionConfig(),
	}
	if err := c.checkInvariants(); err != nil {
		return nil, err
	}
	return c, nil
}

// checkInvariants zorunlu sensör setinin tanımlı olduğunu doğrular.
func (c *Cabin) checkInvariants() error {
	have := make(map[SensorType]bool, len(c.sensors))
	for _, s := range c.sensors {
		have[s] = true
	}
	for _, req := range requiredSensors {
		if !have[req] {
			return errors.New("geçersiz kabin: zorunlu sensör seti (ısı, nem, pH, TDS) eksik")
		}
	}
	return nil
}

// AssignOwner kabini bir kullanıcıya bağlar (claim). Zaten sahibi varsa hata döner.
func (c *Cabin) AssignOwner(userID int64) error {
	if c.ownerUserID != nil {
		if *c.ownerUserID == userID {
			return nil // idempotent
		}
		return errors.New("kabin zaten başka bir kullanıcıya ait")
	}
	c.ownerUserID = &userID
	return nil
}

// Rename kabin adını değiştirir.
func (c *Cabin) Rename(name string) { c.name = name }

// UpdateThresholds doğrulanmış eşik değerlerini uygular.
func (c *Cabin) UpdateThresholds(t Thresholds) error {
	v, err := NewThresholds(t)
	if err != nil {
		return err
	}
	c.thresholds = v
	return nil
}

// UpdateDecisionConfig doğrulanmış karar parametrelerini uygular.
func (c *Cabin) UpdateDecisionConfig(d DecisionConfig) error {
	v, err := NewDecisionConfig(d)
	if err != nil {
		return err
	}
	c.decision = v
	return nil
}

// ApplyActuatorState aktüatör durumunu CİHAZDAN gelen up/state ile günceller.
// Otoriter kaynak budur; komut gönderirken değil.
func (c *Cabin) ApplyActuatorState(s ActuatorState) error {
	if !roleActuators[s.Type] && !fanActuators[s.Type] {
		return errors.New("geçersiz aktüatör tipi")
	}
	c.actuators[s.Type] = s
	return nil
}

// MarkOnline cihazı çevrimiçi işaretler ve lastSeen'i günceller.
func (c *Cabin) MarkOnline(at time.Time) {
	c.online = true
	t := at
	c.lastSeen = &t
}

// MarkOffline cihazı çevrimdışı işaretler (LWT).
func (c *Cabin) MarkOffline() { c.online = false }

// Touch lastSeen'i günceller (heartbeat).
func (c *Cabin) Touch(at time.Time) {
	t := at
	c.lastSeen = &t
}

// --- Erişimciler (read-only) ---

func (c *Cabin) ID() CabinId              { return c.id }
func (c *Cabin) Name() string             { return c.name }
func (c *Cabin) Sensors() []SensorType    { return append([]SensorType(nil), c.sensors...) }
func (c *Cabin) Thresholds() Thresholds   { return c.thresholds }
func (c *Cabin) Decision() DecisionConfig { return c.decision }
func (c *Cabin) Online() bool             { return c.online }

// OwnerUserID iç pointer'ı sızdırmamak için kopya döner (nil = claim'siz).
func (c *Cabin) OwnerUserID() *int64 {
	if c.ownerUserID == nil {
		return nil
	}
	v := *c.ownerUserID
	return &v
}

// LastSeen iç pointer'ı sızdırmamak için kopya döner.
func (c *Cabin) LastSeen() *time.Time {
	if c.lastSeen == nil {
		return nil
	}
	v := *c.lastSeen
	return &v
}
func (c *Cabin) Actuators() map[ActuatorType]ActuatorState {
	out := make(map[ActuatorType]ActuatorState, len(c.actuators))
	for k, v := range c.actuators {
		out[k] = v
	}
	return out
}

// IsOwnedBy verilen kullanıcının sahip olup olmadığını söyler (yetki kontrolü için).
func (c *Cabin) IsOwnedBy(userID int64) bool {
	return c.ownerUserID != nil && *c.ownerUserID == userID
}

// Reconstruct persistans katmanının (repo) DB'den aggregate'i yeniden kurması için.
// Doğrulama yapılmaz; saklı veri zaten geçerli kabul edilir.
func Reconstruct(
	id CabinId, ownerUserID *int64, name string,
	thresholds Thresholds, decision DecisionConfig,
	actuators map[ActuatorType]ActuatorState,
	online bool, lastSeen *time.Time,
) *Cabin {
	// Çağıranın map'ini doğrudan tutma; aliasing'i önlemek için kopyala.
	acts := make(map[ActuatorType]ActuatorState, len(actuators))
	for k, v := range actuators {
		acts[k] = v
	}
	return &Cabin{
		id:          id,
		ownerUserID: ownerUserID,
		name:        name,
		sensors:     append([]SensorType(nil), requiredSensors...),
		actuators:   acts,
		thresholds:  thresholds,
		decision:    decision,
		online:      online,
		lastSeen:    lastSeen,
	}
}

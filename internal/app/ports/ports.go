// Package ports: application katmanının inbound/outbound arayüzleri (hexagonal).
package ports

import (
	"context"
	"time"

	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
	"github.com/kiryue0/hidrobackend/internal/domain/telemetry"
	"github.com/kiryue0/hidrobackend/internal/domain/user"
)

// --- Outbound: persistans ---

// UserRepository kullanıcı kalıcılığı. apperr.ErrNotFound / ErrConflict döner.
type UserRepository interface {
	Create(ctx context.Context, u *user.User) error // başarılıysa u.SetID çağrılır
	GetByID(ctx context.Context, id int64) (*user.User, error)
	GetByUsername(ctx context.Context, username string) (*user.User, error)
	GetByEmail(ctx context.Context, email string) (*user.User, error)
	// Delete kullanıcıyı ve cascade ile tüm verisini siler.
	// Dönen int64 silinen kullanıcı sayısıdır (0 ise etkilenen satır yok).
	Delete(ctx context.Context, id int64) (int64, error)
}

// CabinRepository kabin aggregate kalıcılığı (küçük aggregate).
// Not: aktüatör durumu ve config için ayrı yöntemler ileriki adımlarda eklenir;
// blanket Save kullanılmaz çünkü aktüatör durumunun otoriter kaynağı cihazdır.
type CabinRepository interface {
	// Create cabins + cabin_config (default) satırlarını tek tx'te ekler.
	Create(ctx context.Context, c *cabin.Cabin) error
	// GetByID kabini config + aktüatör durumlarıyla birlikte yükler.
	GetByID(ctx context.Context, id cabin.CabinId) (*cabin.Cabin, error)
	// ListByOwner bir kullanıcının kabinlerini döner (config/aktüatör yüklenmez; özet).
	ListByOwner(ctx context.Context, ownerID int64) ([]*cabin.Cabin, error)
	// Exists kabin var mı.
	Exists(ctx context.Context, id cabin.CabinId) (bool, error)
	// Claim kabini koşullu olarak kullanıcıya atar (sahipsiz veya zaten aynı sahip ise).
	// Atama yapılamazsa (başka sahip) apperr.ErrConflict döner.
	Claim(ctx context.Context, id cabin.CabinId, ownerID int64) error

	// DeleteCabinsByOwner bir kullanıcının tüm kabinlerini cascade ile siler.
	// Kullanıcı silinmeden önce çağrılmalıdır (FK ON DELETE SET NULL yerine).
	DeleteCabinsByOwner(ctx context.Context, ownerID int64) (int64, error)

	// --- MQTT (cihaz) tarafı yazımları ---

	// EnsureCabin kabin yoksa unclaimed olarak oluşturur (idempotent, default config).
	EnsureCabin(ctx context.Context, id cabin.CabinId) error
	// UpsertActuatorState cihazdan gelen otoriter aktüatör durumunu yazar.
	UpsertActuatorState(ctx context.Context, id cabin.CabinId, st cabin.ActuatorState) error
	// MarkOnline çevrimiçi + lastSeen (heartbeat / status online).
	MarkOnline(ctx context.Context, id cabin.CabinId, lastSeen time.Time) error
	// MarkOffline çevrimdışı (LWT status offline).
	MarkOffline(ctx context.Context, id cabin.CabinId) error

	// UpdateConfig kabinin eşik + karar konfigürasyonunu kalıcılaştırır (UI'den).
	UpdateConfig(ctx context.Context, id cabin.CabinId, t cabin.Thresholds, d cabin.DecisionConfig) error
}

// ReadingStore time-series telemetri yazımı + sorgu (aggregate'ten ayrı; Bölüm 3.4).
type ReadingStore interface {
	Insert(ctx context.Context, r telemetry.Reading) error
	// QueryRaw ham ölçümleri ts aralığında (yeniden eskiye) döner.
	QueryRaw(ctx context.Context, cabinID string, from, to time.Time, limit int32) ([]telemetry.Reading, error)
	// QueryHourly saatlik ortalamalara downsample edilmiş ölçümleri döner.
	QueryHourly(ctx context.Context, cabinID string, from, to time.Time, limit int32) ([]telemetry.HourlyReading, error)
	// Latest kabinin en son ölçümünü döner (yoksa apperr.ErrNotFound).
	Latest(ctx context.Context, cabinID string) (telemetry.Reading, error)
	// DeleteOlderThan retention: cutoff'tan eski ölçümleri siler, silinen sayıyı döner.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// AlertStore cihaz uyarılarının kalıcılığı (opsiyonel).
type AlertStore interface {
	Insert(ctx context.Context, a telemetry.Alert) error
}

// --- Outbound: canlı yayın (WebSocket) ---

// LiveEvent UI'ye yayılan tek bir olay.
type LiveEvent struct {
	Type    string `json:"type"` // "reading" | "state" | "status" | "alert"
	CabinID string `json:"cabin_id"`
	Data    any    `json:"data"`
}

// LiveBroadcastPort olayları ilgili kabine abone UI istemcilerine yayar.
type LiveBroadcastPort interface {
	Broadcast(ev LiveEvent)
}

// --- Outbound: cihaza komut/config (MQTT down/*) ---

// ActuatorCommand UI'den gelen manuel aktüatör komutu (down/command).
// Röleler için State; fanlar için Speed anlamlıdır (IsFan ayrımı).
type ActuatorCommand struct {
	Actuator cabin.ActuatorType
	IsFan    bool
	State    bool
	Speed    int
}

// ActuatorCommandPort manuel komutu cihaza yollar (cabin/{id}/down/command).
type ActuatorCommandPort interface {
	Send(ctx context.Context, cabinID cabin.CabinId, cmd ActuatorCommand) error
}

// CabinConfigPort eşik + karar konfigürasyonunu cihaza yollar (cabin/{id}/down/config).
type CabinConfigPort interface {
	Send(ctx context.Context, cabinID cabin.CabinId, t cabin.Thresholds, d cabin.DecisionConfig) error
}

// TestReading UI test modunda girilen sahte sensör değerleri.
type TestReading struct {
	T   float64
	H   float64
	Tds float64
	Ph  float64
}

// TestTelemetryPort sahte ölçümü normal telemetri hattına (cabin/{id}/up/sensors;
// backend'in kendi aboneliği işler: DB + WS) ve cihaz ekranında gösterilebilmesi
// için cabin/{id}/down/test'e yayar. SetTestMode(false) cihaza normal moda
// dönmesini bildirir.
type TestTelemetryPort interface {
	SendTestReading(ctx context.Context, cabinID cabin.CabinId, r TestReading) error
	SetTestMode(ctx context.Context, cabinID cabin.CabinId, enabled bool) error
}

// --- Outbound: güvenlik servisleri ---

// PasswordHasher parola hash'leme/doğrulama (bcrypt).
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) bool
}

// TokenIssuer JWT access token üretir/çözer.
type TokenIssuer interface {
	Issue(userID int64) (string, error)
	// Parse token'ı doğrular ve userID döner.
	Parse(token string) (int64, error)
}

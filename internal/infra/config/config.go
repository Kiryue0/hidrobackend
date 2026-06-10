// Package config yükleme: ortam değişkenleri + .env (viper).
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config tüm uygulama ayarlarını tutar.
type Config struct {
	HTTPPort string `mapstructure:"HTTP_PORT"`

	DatabaseURL string `mapstructure:"DATABASE_URL"`

	JWTSecret string        `mapstructure:"JWT_SECRET"`
	JWTTTL    time.Duration `mapstructure:"JWT_TTL"`

	MQTTBroker   string `mapstructure:"MQTT_BROKER"`
	MQTTClientID string `mapstructure:"MQTT_CLIENT_ID"`
	MQTTUsername string `mapstructure:"MQTT_USERNAME"`
	MQTTPassword string `mapstructure:"MQTT_PASSWORD"`

	// ReadingRetentionDays: ham telemetri saklama süresi (gün). 0 = sınırsız.
	ReadingRetentionDays int `mapstructure:"READING_RETENTION_DAYS"`

	// AllowedOrigins: WebSocket için izin verilen origin'ler (virgülle ayrılmış).
	// Boş veya "*" => tüm origin'ler (yalnızca geliştirme için).
	AllowedOrigins string `mapstructure:"ALLOWED_ORIGINS"`

	// MigrationsDir: .up.sql dosyalarının bulunduğu dizin. Boş ise "migrations".
	MigrationsDir string `mapstructure:"MIGRATIONS_DIR"`

	// WebDir: statik test arayüzünün (index.html) sunulacağı dizin. Boş => kapalı.
	WebDir string `mapstructure:"WEB_DIR"`

	LogLevel string `mapstructure:"LOG_LEVEL"`
}

// AllowedOriginList izin verilen origin'leri liste olarak döner (boşsa nil = hepsi).
func (c *Config) AllowedOriginList() []string {
	s := strings.TrimSpace(c.AllowedOrigins)
	if s == "" || s == "*" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Load ortam değişkenlerini ve (varsa) .env dosyasını okur.
func Load() (*Config, error) {
	v := viper.New()

	v.SetDefault("HTTP_PORT", "8080")
	v.SetDefault("DATABASE_URL", "postgres://hidro:hidro@localhost:5432/hidro?sslmode=disable")
	v.SetDefault("JWT_SECRET", "dev-secret-change-me")
	v.SetDefault("JWT_TTL", "24h")
	v.SetDefault("MQTT_BROKER", "tcp://localhost:1883")
	v.SetDefault("MQTT_CLIENT_ID", "hidro-backend")
	v.SetDefault("MQTT_USERNAME", "backend")
	v.SetDefault("MQTT_PASSWORD", "backendpass")
	v.SetDefault("READING_RETENTION_DAYS", 30)
	v.SetDefault("ALLOWED_ORIGINS", "")
	v.SetDefault("MIGRATIONS_DIR", "migrations")
	v.SetDefault("WEB_DIR", "web")
	v.SetDefault("LOG_LEVEL", "info")

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	// .env yoksa sorun değil; sadece ortam değişkenleri kullanılır.
	_ = v.ReadInConfig()

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config unmarshal: %w", err)
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET zorunlu")
	}

	// Railway / prod ortam değişkenleri: viper AutomaticEnv tutarsız olabiliyor,
	// bu yüzden kritik değişkenleri doğrudan os.Getenv ile okuyup varsayılanı eziyoruz.
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("PORT"); v != "" {
		cfg.HTTPPort = v
	}
	if v := os.Getenv("MQTT_BROKER"); v != "" {
		cfg.MQTTBroker = v
	}
	if v := os.Getenv("MQTT_USERNAME"); v != "" {
		cfg.MQTTUsername = v
	}
	if v := os.Getenv("MQTT_PASSWORD"); v != "" {
		cfg.MQTTPassword = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}

	return &cfg, nil
}

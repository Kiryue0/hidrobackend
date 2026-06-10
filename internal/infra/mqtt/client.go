package mqtt

import (
	"crypto/tls"
	"net/url"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// needsTLS broker URL'si tls:// veya ssl://, mqtts:// şemasıyla mı başlıyor kontrol eder.
func needsTLS(brokerURL string) bool {
	u, err := url.Parse(brokerURL)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "tls", "ssl", "mqtts":
		return true
	default:
		return false
	}
}

// connectTimeout ilk bağlantı denemesi için maksimum bekleme süresi.
// Bu süre içinde broker'a ulaşılamazsa uygulama yine de başlar;
// AutoReconnect + ConnectRetry arka planda yeniden dener.
const connectTimeout = 30 * time.Second

// baseClientOptions ortak paho yapılandırmasını üretir.
// tls:// şeması algılanırsa sistem kök sertifikalarıyla TLS aktifleştirir.
func baseClientOptions(cfg Config) *paho.ClientOptions {
	opts := paho.NewClientOptions().
		AddBroker(cfg.Broker).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username).SetPassword(cfg.Password)
	}

	if needsTLS(cfg.Broker) {
		opts.SetTLSConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})
	}

	return opts
}

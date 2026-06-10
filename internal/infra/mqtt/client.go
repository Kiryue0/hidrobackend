package mqtt

import (
	"context"
	"crypto/tls"
	"log/slog"
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

// connectTimeout tek bir bağlantı denemesi için maksimum bekleme süresi.
const connectTimeout = 15 * time.Second

// connectRetryInterval başarısız bağlantı denemeleri arası bekleme.
const connectRetryInterval = 5 * time.Second

// connectWithRetry bağlantı kurulana dek dener ve HER denemenin gerçek
// hatasını loglar. Paho'nun ConnectRetry'ı CONNACK reddini (örn. "not
// Authorized") token'a yansıtmadan sessizce yeniden dediği için
// kullanılmıyor; teşhis edilebilirlik bu döngüyle sağlanıyor.
func connectWithRetry(ctx context.Context, c paho.Client, log *slog.Logger, name string) {
	for attempt := 1; ; attempt++ {
		token := c.Connect()
		token.Wait()
		if err := token.Error(); err != nil {
			log.Error("mqtt baglanti hatasi", "client", name, "deneme", attempt, "err", err)
		} else {
			log.Info("mqtt baglandi", "client", name)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(connectRetryInterval):
		}
	}
}

// baseClientOptions ortak paho yapılandırmasını üretir.
// tls:// şeması algılanırsa sistem kök sertifikalarıyla TLS aktifleştirir.
func baseClientOptions(cfg Config) *paho.ClientOptions {
	opts := paho.NewClientOptions().
		AddBroker(cfg.Broker).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectTimeout(connectTimeout)

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

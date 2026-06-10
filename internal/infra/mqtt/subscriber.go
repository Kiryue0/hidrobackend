// Package mqtt: paho tabanlı primary (driving) adapter — cihazdan gelen up/* mesajları.
package mqtt

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/kiryue0/hidrobackend/internal/app/usecases"
	"github.com/kiryue0/hidrobackend/internal/domain/telemetry"
)

const (
	upTopicFilter = "cabin/+/up/#"
	subQoS        = 1
	workerCount   = 4
	queueSize     = 256
	handleTimeout = 5 * time.Second
)

// Config MQTT bağlantı ayarları.
type Config struct {
	Broker   string
	ClientID string
	Username string
	Password string
}

type job struct {
	topic   string
	payload []byte
}

// Subscriber up/* mesajlarını dinler ve TelemetryService'e yönlendirir.
type Subscriber struct {
	client    paho.Client
	broker    string
	username  string
	telemetry *usecases.TelemetryService
	log       *slog.Logger

	jobs chan job
	wg   sync.WaitGroup

	ctx    context.Context
	cancel context.CancelFunc
}

// NewSubscriber paho client'ı yapılandırır (henüz bağlanmaz).
func NewSubscriber(cfg Config, ts *usecases.TelemetryService, log *slog.Logger) *Subscriber {
	s := &Subscriber{
		broker:    cfg.Broker,
		username:  cfg.Username,
		telemetry: ts,
		log:       log,
		jobs:      make(chan job, queueSize),
	}

	opts := baseClientOptions(cfg).
		SetClientID(cfg.ClientID).
		SetOnConnectHandler(s.onConnect).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			log.Warn("mqtt baglanti koptu", "err", err)
		}).
		SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
			log.Info("mqtt yeniden baglaniliyor...")
		})

	s.client = paho.NewClient(opts)
	return s
}

// Start worker'ları başlatır ve broker'a bağlanır. Abonelik onConnect'te yapılır.
func (s *Subscriber) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go s.worker()
	}

	// Etkin yapılandırmayı logla: Railway'de env değişkeninin gerçekten
	// uygulanıp uygulanmadığı buradan doğrulanır (parola asla loglanmaz).
	s.log.Info("mqtt baglaniliyor", "client", "subscriber", "broker", s.broker, "username", s.username)
	go connectWithRetry(s.ctx, s.client, s.log, "subscriber")
	return nil
}

// onConnect (ilk bağlanma + her reconnect) up/* filtresine abone olur.
func (s *Subscriber) onConnect(c paho.Client) {
	token := c.Subscribe(upTopicFilter, subQoS, s.onMessage)
	if !token.WaitTimeout(10 * time.Second) {
		s.log.Error("mqtt abonelik zaman asimi", "filter", upTopicFilter)
		return
	}
	if err := token.Error(); err != nil {
		s.log.Error("mqtt abonelik hatasi", "filter", upTopicFilter, "err", err)
		return
	}
	s.log.Info("mqtt abone olundu", "filter", upTopicFilter)
}

// onMessage paho callback'i: mesajı kuyruğa koyar (bloklamaz; doluysa düşürür).
func (s *Subscriber) onMessage(_ paho.Client, m paho.Message) {
	// payload kopyalanır; paho buffer'ı yeniden kullanabilir.
	p := make([]byte, len(m.Payload()))
	copy(p, m.Payload())
	select {
	case s.jobs <- job{topic: m.Topic(), payload: p}:
	default:
		s.log.Warn("mqtt kuyruğu dolu, mesaj düşürüldü", "topic", m.Topic())
	}
}

func (s *Subscriber) worker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case j, ok := <-s.jobs:
			if !ok {
				return
			}
			s.handle(j)
		}
	}
}

// handle topic'i çözer ve uygun use case'i çağırır.
func (s *Subscriber) handle(j job) {
	cabinID, msgType, ok := parseUpTopic(j.topic)
	if !ok {
		s.log.Warn("mqtt geçersiz topic", "topic", j.topic)
		return
	}

	ctx, cancel := context.WithTimeout(s.ctx, handleTimeout)
	defer cancel()

	var err error
	switch msgType {
	case "sensors":
		err = s.handleSensors(ctx, cabinID, j.payload)
	case "state":
		err = s.handleState(ctx, cabinID, j.payload)
	case "heartbeat":
		err = s.handleHeartbeat(ctx, cabinID, j.payload)
	case "status":
		err = s.handleStatus(ctx, cabinID, j.payload)
	case "alert":
		err = s.handleAlert(ctx, cabinID, j.payload)
	default:
		s.log.Debug("mqtt bilinmeyen mesaj tipi", "type", msgType, "topic", j.topic)
		return
	}
	if err != nil {
		s.log.Error("mqtt mesaj işleme hatası", "type", msgType, "cabin", cabinID, "err", err)
	}
}

// parseUpTopic "cabin/{id}/up/{type}" -> (id, type, true).
func parseUpTopic(topic string) (cabinID, msgType string, ok bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 || parts[0] != "cabin" || parts[2] != "up" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

// tsOrNow ts=0 (RTC ayarsız) ise alış zamanını kullanır (Bölüm 11 notu).
func tsOrNow(ts int64) time.Time {
	if ts <= 0 {
		return time.Now()
	}
	return time.Unix(ts, 0)
}

// --- payload tipleri (Bölüm 2.3 kontratı) ---

type sensorsPayload struct {
	Ts int64   `json:"ts"`
	T  float64 `json:"t"`
	H  float64 `json:"h"`
	Td float64 `json:"tds"`
	Ph float64 `json:"ph"`
	Ok struct {
		Sht bool `json:"sht"`
		Rtc bool `json:"rtc"`
		Tds bool `json:"tds"`
		Ph  bool `json:"ph"`
	} `json:"ok"`
}

type statePayload struct {
	Ts         int64  `json:"ts"`
	Humidifier bool   `json:"humidifier"`
	HavaMotoru bool   `json:"havaMotoru"`
	CobLed     bool   `json:"cobLed"`
	Fan1       int    `json:"fan1"`
	Fan2       int    `json:"fan2"`
	Source     string `json:"source"`
}

type heartbeatPayload struct {
	Ts     int64  `json:"ts"`
	Fw     string `json:"fw"`
	Ip     string `json:"ip"`
	Uptime int64  `json:"uptime"`
	User   string `json:"user"`
}

type alertPayload struct {
	Ts   int64  `json:"ts"`
	Type string `json:"type"`
	Msg  string `json:"msg"`
}

func (s *Subscriber) handleSensors(ctx context.Context, cabinID string, payload []byte) error {
	var p sensorsPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	return s.telemetry.IngestReading(ctx, telemetry.Reading{
		CabinID:     cabinID,
		Ts:          tsOrNow(p.Ts),
		Temperature: p.T,
		Humidity:    p.H,
		TDS:         p.Td,
		PH:          p.Ph,
		Health:      telemetry.SensorHealth{SHT: p.Ok.Sht, RTC: p.Ok.Rtc, TDS: p.Ok.Tds, PH: p.Ok.Ph},
	})
}

func (s *Subscriber) handleState(ctx context.Context, cabinID string, payload []byte) error {
	var p statePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	return s.telemetry.UpdateActuatorState(ctx, cabinID, usecases.ActuatorSnapshot{
		Humidifier: p.Humidifier,
		HavaMotoru: p.HavaMotoru,
		CobLed:     p.CobLed,
		Fan1:       p.Fan1,
		Fan2:       p.Fan2,
		Source:     p.Source,
	})
}

func (s *Subscriber) handleHeartbeat(ctx context.Context, cabinID string, payload []byte) error {
	var p heartbeatPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	return s.telemetry.RecordHeartbeat(ctx, cabinID, p.User, tsOrNow(p.Ts))
}

func (s *Subscriber) handleStatus(ctx context.Context, cabinID string, payload []byte) error {
	// up/status retained düz metin: "online" / "offline".
	online := strings.TrimSpace(strings.ToLower(string(payload))) == "online"
	return s.telemetry.SetCabinStatus(ctx, cabinID, online)
}

func (s *Subscriber) handleAlert(ctx context.Context, cabinID string, payload []byte) error {
	var p alertPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	return s.telemetry.RecordAlert(ctx, telemetry.Alert{
		CabinID: cabinID,
		Ts:      tsOrNow(p.Ts),
		Type:    telemetry.AlertType(p.Type),
		Message: p.Msg,
	})
}

// Stop broker bağlantısını kapatır ve worker'ların bitmesini bekler.
func (s *Subscriber) Stop() {
	if s.client.IsConnected() {
		s.client.Disconnect(250)
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

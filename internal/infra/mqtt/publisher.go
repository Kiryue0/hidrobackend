package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
)

const publishQoS = 1

// Publisher down/command + down/config yayını yapar.
// ports.ActuatorCommandPort ve ports.CabinConfigPort implementasyonu.
type Publisher struct {
	client paho.Client
	broker string
	log    *slog.Logger
	cancel context.CancelFunc
}

// NewPublisher kendi paho client'ını yapılandırır (ClientID'ye "-pub" eklenir).
func NewPublisher(cfg Config, log *slog.Logger) *Publisher {
	opts := baseClientOptions(cfg).
		SetClientID(cfg.ClientID + "-pub")
	return &Publisher{client: paho.NewClient(opts), broker: cfg.Broker, log: log}
}

// Start broker'a arka planda bağlanır; başarısız denemelerin gerçek hatasını loglar.
func (p *Publisher) Start(ctx context.Context) error {
	ctx, p.cancel = context.WithCancel(ctx)
	p.log.Info("mqtt baglaniliyor", "client", "publisher", "broker", p.broker)
	go connectWithRetry(ctx, p.client, p.log, "publisher")
	return nil
}

// Stop bağlantıyı (ve süren bağlanma denemelerini) durdurur.
func (p *Publisher) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	if p.client.IsConnected() {
		p.client.Disconnect(250)
	}
}

func (p *Publisher) publish(ctx context.Context, topic string, payload []byte) error {
	token := p.client.Publish(topic, publishQoS, false, payload)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-tokenDone(token):
		return token.Error()
	}
}

// tokenDone paho token'ı için bir tamamlanma kanalı üretir (ctx ile select edilebilsin).
func tokenDone(t paho.Token) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		t.Wait()
		close(ch)
	}()
	return ch
}

// Send ports.ActuatorCommandPort: down/command yayını.
func (p *Publisher) Send(ctx context.Context, cabinID cabin.CabinId, cmd ports.ActuatorCommand) error {
	var payload []byte
	var err error
	if cmd.IsFan {
		payload, err = json.Marshal(struct {
			Actuator string `json:"actuator"`
			Speed    int    `json:"speed"`
		}{string(cmd.Actuator), cmd.Speed})
	} else {
		payload, err = json.Marshal(struct {
			Actuator string `json:"actuator"`
			State    bool   `json:"state"`
		}{string(cmd.Actuator), cmd.State})
	}
	if err != nil {
		return err
	}
	topic := fmt.Sprintf("cabin/%s/down/command", cabinID.String())
	return p.publish(ctx, topic, payload)
}

// testPort TestTelemetryPort implementasyonu (aynı client).
type testPort struct{ p *Publisher }

// TestPort aynı yayıncıyı TestTelemetryPort olarak döner.
func (p *Publisher) TestPort() ports.TestTelemetryPort { return testPort{p: p} }

// SendTestReading sahte ölçümü up/sensors kontratıyla yayınlar (backend'in
// kendi aboneliği normal hattan işler: DB + WS + grafik) ve cihazın
// gösterebilmesi için down/test'e iletir.
func (t testPort) SendTestReading(ctx context.Context, cabinID cabin.CabinId, r ports.TestReading) error {
	// ts=0: alış zamanı kullanılır (tsOrNow). Kontrat: Bölüm 2.3 up/sensors.
	sp := sensorsPayload{T: r.T, H: r.H, Td: r.Tds, Ph: r.Ph}
	sp.Ok.Sht, sp.Ok.Tds, sp.Ok.Ph = true, true, true
	sensors, err := json.Marshal(sp)
	if err != nil {
		return err
	}
	if err := t.p.publish(ctx, fmt.Sprintf("cabin/%s/up/sensors", cabinID.String()), sensors); err != nil {
		return err
	}
	down, err := json.Marshal(struct {
		Enabled bool    `json:"enabled"`
		T       float64 `json:"t"`
		H       float64 `json:"h"`
	}{true, r.T, r.H})
	if err != nil {
		return err
	}
	return t.p.publish(ctx, fmt.Sprintf("cabin/%s/down/test", cabinID.String()), down)
}

// SetTestMode cihaza test modunun açılıp kapandığını bildirir (down/test).
func (t testPort) SetTestMode(ctx context.Context, cabinID cabin.CabinId, enabled bool) error {
	payload, err := json.Marshal(struct {
		Enabled bool `json:"enabled"`
	}{enabled})
	if err != nil {
		return err
	}
	return t.p.publish(ctx, fmt.Sprintf("cabin/%s/down/test", cabinID.String()), payload)
}

// configPublisher CabinConfigPort'u ayrı bir tip üzerinden sunar (aynı client).
// Not: down/config payload'u {"thresholds":...,"decision":...} (Bölüm 2.3).
type configPort struct{ p *Publisher }

// ConfigPort aynı yayıncıyı CabinConfigPort olarak döner.
func (p *Publisher) ConfigPort() ports.CabinConfigPort { return configPort{p: p} }

func (c configPort) Send(ctx context.Context, cabinID cabin.CabinId, t cabin.Thresholds, d cabin.DecisionConfig) error {
	payload, err := json.Marshal(struct {
		Thresholds cabin.Thresholds     `json:"thresholds"`
		Decision   cabin.DecisionConfig `json:"decision"`
	}{t, d})
	if err != nil {
		return err
	}
	topic := fmt.Sprintf("cabin/%s/down/config", cabinID.String())
	return c.p.publish(ctx, topic, payload)
}

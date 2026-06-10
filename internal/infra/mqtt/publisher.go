package mqtt

import (
	"context"
	"encoding/json"
	"fmt"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/kiryue0/hidrobackend/internal/app/ports"
	"github.com/kiryue0/hidrobackend/internal/domain/cabin"
)

const publishQoS = 1

// Publisher down/command + down/config yayını yapar.
// ports.ActuatorCommandPort ve ports.CabinConfigPort implementasyonu.
type Publisher struct {
	client paho.Client
}

// NewPublisher kendi paho client'ını yapılandırır (ClientID'ye "-pub" eklenir).
func NewPublisher(cfg Config) *Publisher {
	opts := baseClientOptions(cfg).
		SetClientID(cfg.ClientID + "-pub")
	return &Publisher{client: paho.NewClient(opts)}
}

// Start broker'a bağlanır. connectTimeout içinde bağlanamazsa arka plana bırakır.
func (p *Publisher) Start() error {
	token := p.client.Connect()
	if token.WaitTimeout(connectTimeout) {
		return token.Error()
	}
	// Timeout: broker şu an erişilemez. AutoReconnect arka planda yeniden dener.
	return nil
}

// Stop bağlantıyı kapatır.
func (p *Publisher) Stop() {
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

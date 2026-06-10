// Sahte ESP32 cihazı (gerçek firmware MQTT modülü ADIM F yazılana dek test/sunum aracı).
// Kabini verilen backend kullanıcısına claim eder, sürekli telemetri yayar VE
// backend'den gelen down/command + down/config mesajlarını dinleyip aktüatör durumunu
// güncelleyerek gerçek up/state'i geri yayar (otoriter kaynak = cihaz döngüsü).
//
// Kullanım:
//   go run ./cmd/sim <backend_kullanici_adi> [kabin_id] [parola]
// Örnek:
//   go run ./cmd/sim melih
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Kullanım: go run ./cmd/sim <backend_kullanici_adi> [kabin_id] [parola]")
		os.Exit(1)
	}
	user := os.Args[1]
	cabin := "CAB-3778C4"
	if len(os.Args) > 2 {
		cabin = os.Args[2]
	}
	pass := "devpass"
	if len(os.Args) > 3 {
		pass = os.Args[3]
	}
	broker := os.Getenv("MQTT_BROKER")
	if broker == "" {
		broker = "tcp://localhost:1883"
	}
	base := "cabin/" + cabin

	var mu sync.Mutex
	st := struct {
		Humidifier bool
		HavaMotoru bool
		CobLed     bool
		Fan1       int
		Fan2       int
	}{Humidifier: true, HavaMotoru: false, CobLed: true, Fan1: 120, Fan2: 80}

	var cli paho.Client

	pubState := func(source string) {
		mu.Lock()
		payload := map[string]any{
			"ts": 0, "humidifier": st.Humidifier, "havaMotoru": st.HavaMotoru,
			"cobLed": st.CobLed, "fan1": st.Fan1, "fan2": st.Fan2, "source": source,
		}
		mu.Unlock()
		b, _ := json.Marshal(payload)
		cli.Publish(base+"/up/state", 1, false, b)
	}

	opts := paho.NewClientOptions().
		AddBroker(broker).
		SetClientID("esp32-sim-" + cabin).
		SetUsername(cabin).
		SetPassword(pass).
		SetAutoReconnect(true).
		SetWill(base+"/up/status", "offline", 1, true). // LWT
		SetOnConnectHandler(func(c paho.Client) {
			// Manuel komutları dinle ve uygula (cihaz davranışı)
			c.Subscribe(base+"/down/command", 1, func(_ paho.Client, m paho.Message) {
				var cmd struct {
					Actuator string `json:"actuator"`
					State    *bool  `json:"state"`
					Speed    *int   `json:"speed"`
				}
				if json.Unmarshal(m.Payload(), &cmd) != nil {
					return
				}
				mu.Lock()
				switch cmd.Actuator {
				case "HUMIDIFIER":
					if cmd.State != nil {
						st.Humidifier = *cmd.State
					}
				case "HAVA_MOTORU":
					if cmd.State != nil {
						st.HavaMotoru = *cmd.State
					}
				case "COB_LED":
					if cmd.State != nil {
						st.CobLed = *cmd.State
					}
				case "FAN1":
					if cmd.Speed != nil {
						st.Fan1 = *cmd.Speed
					}
				case "FAN2":
					if cmd.Speed != nil {
						st.Fan2 = *cmd.Speed
					}
				}
				mu.Unlock()
				log.Printf("[sim] komut alındı: %s", string(m.Payload()))
				pubState("backend") // cihaz uygular ve gerçek durumu geri yayınlar
			})
			c.Subscribe(base+"/down/config", 1, func(_ paho.Client, m paho.Message) {
				log.Printf("[sim] config alındı: %s", string(m.Payload()))
			})
			log.Printf("[sim] bağlandı + abone: %s/down/#", base)
		})

	cli = paho.NewClient(opts)
	if t := cli.Connect(); t.Wait() && t.Error() != nil {
		log.Fatalf("[sim] broker bağlantısı başarısız: %v", t.Error())
	}

	// Çevrimiçi + claim + ilk aktüatör durumu
	cli.Publish(base+"/up/status", 1, true, "online")
	hb, _ := json.Marshal(map[string]any{"ts": 0, "fw": "sim-1.0", "ip": "10.0.0.9", "uptime": 0, "user": user})
	cli.Publish(base+"/up/heartbeat", 1, false, hb)
	pubState("decision")
	log.Printf("[sim] ESP32 simülatörü: kabin=%s -> kullanıcı=%q (Ctrl+C ile durdur)", cabin, user)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-stop:
			log.Println("[sim] çevrimdışı bırakılıyor...")
			cli.Publish(base+"/up/status", 1, true, "offline").Wait()
			cli.Disconnect(250)
			return
		case <-ticker.C:
			i++
			t := 22 + rand.Float64()*9    // ~22-31 °C
			h := 55 + rand.Float64()*25   // ~55-80 %
			tds := 800 + rand.Intn(500)   // ~800-1300 ppm
			ph := 5.5 + rand.Float64()    // ~5.5-6.5
			s, _ := json.Marshal(map[string]any{
				"ts": 0, "t": round1(t), "h": round1(h), "tds": tds, "ph": round2(ph),
				"ok": map[string]bool{"sht": true, "rtc": false, "tds": true, "ph": true},
			})
			cli.Publish(base+"/up/sensors", 1, false, s)
			if i%6 == 0 {
				hb, _ := json.Marshal(map[string]any{"ts": 0, "fw": "sim-1.0", "ip": "10.0.0.9", "uptime": i * 5, "user": user})
				cli.Publish(base+"/up/heartbeat", 1, false, hb)
			}
			if i%13 == 0 {
				al, _ := json.Marshal(map[string]any{"ts": 0, "type": "UYARI", "msg": "Nem ust esige yaklasti"})
				cli.Publish(base+"/up/alert", 1, false, al)
			}
		}
	}
}

func round1(v float64) float64 { return float64(int(v*10)) / 10 }
func round2(v float64) float64 { return float64(int(v*100)) / 100 }

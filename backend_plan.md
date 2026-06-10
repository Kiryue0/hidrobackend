# Hidroponik Akıllı Kabin — BACKEND Master Doküman

> Bu doküman yeni bir Claude Code oturumuna doğrudan verilmek üzere hazırlanmıştır.
> Backend'i (Go) sıfırdan kuracak oturum bunu tek referans alır.
> Firmware (ESP32) tarafı TAMAMLANDI; bu doküman onun üstüne backend'i tarif eder.

---

## 0. KARARLAR (kilitli) ve VARSAYILANLAR (değiştirilebilir)

**Kilitli kararlar:**
- Mimari: **Hexagonal (Ports & Adapters) + DDD** (tam sürüm)
- Dil: **Go**
- Veritabanı: **PostgreSQL** (time-series için ayrı tablo; ölçek gerekirse TimescaleDB)
- Canlı kanal: **WebSocket** (sunucu → tarayıcı broadcast)
- Mesajlaşma: **MQTT (Mosquitto broker)** — backend ↔ ESP32
- Karar motoru **CİHAZDA** çalışır (firmware). Backend yalnızca eşikleri günceller + manuel komut gönderir. İnternet kopsa kabin kendi kararını verir.

**Varsayılanlar (ilk oturumda onaylat/değiştir):**
- HTTP framework: **Gin**
- DB erişimi: **pgx + sqlc** (ORM değil; type-safe SQL). Alternatif: GORM (daha kolay ama daha az şeffaf).
- MQTT client: **eclipse/paho.mqtt.golang**
- Auth: **JWT** (access token) + **bcrypt** (parola). Cihaz auth: broker düzeyinde kabin başına MQTT kullanıcı/parola + topic ACL.
- Migration: **golang-migrate** (veya goose)
- Config: env / **viper**

---

## 1. SİSTEM GENEL BAKIŞ

```
[ESP32 Kabin]  --MQTT-->  [Mosquitto]  <--MQTT-->  [Go Backend]  <--WS/REST-->  [Web/Mobil UI]
   (firmware)                                          |
   - sensör okur                                  [PostgreSQL]
   - karar motoru (otonom)
   - aktüatör sürer
```

- **ESP32 (firmware, HAZIR):** SHT31 (ısı/nem), DS3231 (RTC), TDS, pH okur. FreeRTOS. Otonom **decision engine** eşiklere göre fan/LED/humidifier/su-pompası sürer. ILI9341 ekran + 4 buton. WiFi + captive-portal provisioning (WiFiManager). **MQTT henüz YOK** — bu kontrata göre firmware'e eklenecek (ayrı görev, Bölüm 9-ADIM F).
- **Backend (YAPILACAK):** MQTT'den telemetri/durum alır, Postgres'e yazar, WebSocket ile UI'ye yayar; UI'den gelen manuel komut/konfig'i MQTT ile cihaza gönderir; kullanıcı/kabin/yetki yönetimi.

---

## 2. FIRMWARE ENTEGRASYON KONTRATI ⚠️ (EN KRİTİK BÖLÜM)

Bu kontrat hem backend'in hem de firmware-MQTT modülünün uyacağı **tek gerçeğdir**. Aşağıdaki alan adları firmware'deki struct'larla birebir eşleşir.

### 2.1 Kimlik
- `kabin_id`: **otomatik**, format `"CAB-" + MAC_son6hex` (büyük harf), örn. `CAB-3778C4`. Firmware üretir, değişmez.
- `username`: provisioning ekranında kullanıcı girer ("Kullanici adi (backend)"), NVS'de saklanır. Kabini bir backend kullanıcısına **claim** etmek için kullanılır.

### 2.2 MQTT Topic Şeması
```
cabin/{kabin_id}/up/sensors     ESP32 -> backend   (telemetri, ~30sn; eşik aşımında 5sn)
cabin/{kabin_id}/up/state       ESP32 -> backend   (aktüatör durumu değişince)
cabin/{kabin_id}/up/heartbeat   ESP32 -> backend   (her 30sn; fw/ip/uptime/user)
cabin/{kabin_id}/up/alert       ESP32 -> backend   (uyarı oluşunca; opsiyonel)
cabin/{kabin_id}/up/status      ESP32 -> backend   (LWT retained: "online"/"offline")
cabin/{kabin_id}/down/command   backend -> ESP32   (manuel aktüatör komutu)
cabin/{kabin_id}/down/config    backend -> ESP32   (eşik + karar parametreleri güncelleme)
```
- **QoS 1** öneri (sensors için QoS 0 da olur). **LWT** (Last Will): cihaz `up/status`'a `offline` (retained) bırakır; bağlanınca `online` (retained) yazar → backend çevrimiçi durumunu buradan bilir.

### 2.3 Payload'lar (JSON)

**up/sensors** (firmware `SensorVeri`):
```json
{ "ts": 1718000000, "t": 24.5, "h": 62.3, "tds": 850, "ph": 6.10,
  "ok": { "sht": true, "rtc": true, "tds": true, "ph": true } }
```
`t`=°C, `h`=%, `tds`=ppm, `ph`=0-14. `ts`=unix saniye (RTC'den; RTC yoksa 0). `ok`=sensör sağlık bayrakları.

**up/state** (firmware `AktuatorDurumu` + kaynak):
```json
{ "ts": 1718000000, "humidifier": true, "havaMotoru": true,
  "cobLed": false, "fan1": 80, "fan2": 80, "source": "decision" }
```
`fan1/fan2`=0..255. `source` ∈ `button | serial | decision | backend` (firmware `KomutKaynagi`). **Bu, aktüatör durumunun TEK doğru kaynağıdır** — backend DB'yi buradan günceller, komut gönderirken DEĞİL.

**up/heartbeat**:
```json
{ "ts": 1718000000, "fw": "v0.1", "ip": "172.20.10.2",
  "uptime": 12345, "user": "melih" }
```
`user` = provisioning'de girilen username (claim için; atanana kadar gönderilir).

**up/alert** (firmware `AlertTipi`):
```json
{ "ts": 1718000000, "type": "KRITIK", "msg": "Sicaklik cok yuksek" }
```
`type` ∈ `KRITIK(kırmızı) | UYARI(sarı) | AKSIYON(yeşil) | NORMAL(mavi)`.

**down/command** (manuel kontrol):
```json
{ "actuator": "COB_LED", "state": true }
{ "actuator": "FAN1", "speed": 128 }
```
`actuator` ∈ `HUMIDIFIER | HAVA_MOTORU | COB_LED | FAN1 | FAN2` (firmware `AktuatorTipi`). Röleler için `state` (bool), fanlar için `speed` (0..255). Firmware bunu `kaynak=BACKEND` ile uygular ve sonucu `up/state` ile geri yayınlar.

**down/config** (eşik + karar; kısmi/tam):
```json
{
  "thresholds": {
    "sicaklikMin": 18, "sicaklikMax": 28, "sicaklikKritikMax": 32,
    "nemMin": 55, "nemMax": 75, "nemKritikMax": 85,
    "tdsMin": 800, "tdsMax": 1800, "phMin": 5.5, "phMax": 6.5
  },
  "decision": {
    "otomatikMod": true,
    "fanOrtaEsik": 28, "fanTamEsik": 30, "ledKisEsik": 32, "ledKapatEsik": 34,
    "fanBazHiz": 80, "fanOrtaHiz": 128, "fanTamHiz": 255, "ledKisikDuty": 128,
    "histerezisC": 0.5, "histerezisNem": 2.0,
    "havaMotoruMod": 1, "havaAcikSn": 900, "havaKapaliSn": 900
  }
}
```
Firmware bunları NVS'e yazar (kalıcı) ve uygular. `havaMotoruMod`: `0=MANUEL, 1=SUREKLI, 2=DONGU` (su havalandırma pompası = besin suyuna oksijen; iklimle ilgisi yok).

### 2.4 Firmware veri yapıları referansı (Go modelini buna göre kur)
- **EsikConfig (thresholds):** sicaklikMin/Max/KritikMax, nemMin/Max/KritikMax, tdsMin/Max, phMin/Max.
- **KararConfig (decision):** otomatikMod, fanOrtaEsik, fanTamEsik, ledKisEsik, ledKapatEsik, fanBazHiz, fanOrtaHiz, fanTamHiz, ledKisikDuty, histerezisC, histerezisNem, havaMotoruMod, havaAcikSn, havaKapaliSn.
- **Aktüatörler:** HUMIDIFIER, HAVA_MOTORU (su pompası), COB_LED, FAN1, FAN2.
- **Karar mantığı (cihazda):** sıcaklık→taban+kademeli fan + aşırı sıcakta LED kapat/soğuyunca aç; nem→humidifier; su pompası→moda göre. TDS/pH sadece uyarı (düzeltici aktüatör yok).

### 2.5 Claim (kabin → kullanıcı eşleştirme) akışı
1. Cihaz ilk MQTT bağlantısında `up/heartbeat` içinde `user` (username) yayınlar.
2. Backend `kabin_id` bilmiyorsa → **unclaimed cabin** kaydı oluşturur.
3. `user` bir kullanıcıyla eşleşiyorsa → kabini o kullanıcıya `owner` atar (veya kullanıcı UI'den "kabin ekle" ile `kabin_id` girip claim eder — alternatif/ek akış).
4. Atandıktan sonra cihaz `user` göndermeyi sürdürebilir (idempotent).

---

## 3. DOMAIN KATMANI (saf Go, sıfır dış bağımlılık)

> Düzeltme notu: senin ilk taslağında geçmiş sensör logları Cabin aggregate'inin içindeydi — bu **kaldırıldı**. Aggregate küçük ve tutarlılık-sınırı olmalı; time-series ayrı (Bölüm 3.4).

### 3.1 Bounded Context'ler
- **Identity:** User (aggregate).
- **Cabin Management:** Cabin (aggregate) — kimlik, sahiplik, sensör/aktüatör tanımları, **güncel** aktüatör durumu, config (thresholds + decision).
- **Telemetry:** Reading (aggregate DEĞİL; append-only fact). Ayrı yazma yolu + read model.

### 3.2 Value Objects (immutable + constructor validation)
- `Thresholds`: EsikConfig aynası. Constructor mutlak fiziksel sınırları denetler (örn. sıcaklık −20..60 dışı, min<max kuralı) ve geçersizse hata döner. Backend'de ekstra güvenlik katmanı (cihaz da kendi default'una sahip).
- `DecisionConfig`: KararConfig aynası. Aynı şekilde sınır/ tutarlılık doğrulaması.
- `ActuatorState`: tip + (on/off | speed).
- `CabinId` (CAB-XXXXXX format doğrulaması), `Email`/`Username`.

### 3.3 Entities & Aggregate Roots
- **User (root):** id, username, email, passwordHash, sahip olunan kabin id'leri.
- **Cabin (root):** kabin_id (PK), ownerUserId (claim'e kadar null olabilir), name, sensors[] (tip+sağlık), actuators[] (tip+güncel durum), thresholds (VO), decisionConfig (VO), online (bool), lastSeen.
  - **Invariant:** geçerli sayılmak için zorunlu sensör seti (ısı, nem, pH, TDS) tanımlı olmalı.
  - **Kural:** aktüatör/sensör doğrudan manipüle edilmez; tüm değişiklik Cabin metotlarından geçer. AMA güncel aktüatör durumu **cihazdan gelen `up/state` ile** güncellenir (kaynak = cihaz), komut gönderirken değil.

### 3.4 Telemetry (aggregate DIŞINDA)
- `Reading`: cabinId, ts, t, h, tds, ph, ok-bayrakları. Append-only. Domain invariant'ı yok; doğrudan time-series deposuna yazılır, ayrı read-model ile sorgulanır (CQRS-vari ayrım). Yüksek frekanslı veri Cabin aggregate'inden GEÇMEZ.

---

## 4. APPLICATION KATMANI (use case'ler + portlar)

### 4.1 Inbound Ports (Use Cases)
**Telemetri / veri:**
- `IngestReadingUseCase` — MQTT `up/sensors`'tan. Reading'i time-series'e yazar + `LiveBroadcastPort` ile yayar. (Cabin aggregate'ine dokunmaz.)
- `UpdateActuatorStateUseCase` — MQTT `up/state`'ten. Cabin'in **güncel** aktüatör durumunu (otoriter) günceller + yayar.
- `RecordHeartbeatUseCase` — MQTT `up/heartbeat`/LWT'den. online/lastSeen günceller; claim (username) işler.
- `RecordAlertUseCase` — MQTT `up/alert`'ten (ops.). Saklar + yayar.
- `GetSensorHistoryQuery` — UI grafik için zaman serisi filtreli okuma (read model).
- `GetCabinStateQuery` — kabinin güncel durumu/aktüatör/config.

**Aktüatör / config:**
- `SendActuatorCommandUseCase` (ManageActuator) — UI'den manuel komut. Yetki kontrolü → `ActuatorCommandPort` ile `down/command` publish. **DB'yi iyimser güncellemez**; gerçek durum cihazın `up/state`'iyle gelir.
- `UpdateCabinConfigUseCase` (ConfigureCabin) — UI'den eşik/karar güncelle. VO ile doğrula → persist → `CabinConfigPort` ile `down/config` publish.

**Sistem / kimlik:**
- `RegisterUserUseCase`, `LoginUseCase` (JWT üret).
- `ClaimCabinUseCase` — kabini kullanıcıya bağla (username eşleşmesi veya UI'den kabin_id ile).
- `CreateCabinUseCase` (gerekiyorsa elle).

### 4.2 Outbound Ports
- `UserRepository`, `CabinRepository` (küçük aggregate persist).
- `ReadingStore` (time-series yazma + sorgu) — **ayrı**.
- `AlertStore` (ops.).
- `ActuatorCommandPort` (→ MQTT `down/command`).
- `CabinConfigPort` (→ MQTT `down/config`).
- `LiveBroadcastPort` (→ WebSocket hub broadcast).

---

## 5. INFRASTRUCTURE (adapters)

**Primary (driving):**
- `HttpHandler` (Gin) — REST uçları + JWT middleware (userID'yi context'e koyar) → Inbound Port'lar.
- `MqttSubscriberAdapter` (paho) — topic'leri dinler, `up/*` mesajlarını ilgili use case'e yönlendirir.
- `WebSocketHandler` — UI bağlantılarını kabul eder (JWT ile), hub'a kaydeder.

**Secondary (driven):**
- `PostgresAdapter` (pgx/sqlc) — `UserRepository`, `CabinRepository`, `ReadingStore`, `AlertStore` implement eder.
- `MqttPublisherAdapter` (paho) — `ActuatorCommandPort` + `CabinConfigPort` implement eder.
- `WebSocketHub` — `LiveBroadcastPort` implement eder (bağlantı havuzu, kabin bazlı abonelik).

---

## 6. VERİ MODELİ (PostgreSQL)

```
users(id PK, username UNIQUE, email UNIQUE, password_hash, created_at)

cabins(id PK = kabin_id TEXT, owner_user_id FK->users NULL,
       name, online BOOL, last_seen TIMESTAMPTZ, created_at)

cabin_config(cabin_id PK/FK, thresholds JSONB, decision JSONB, updated_at)
   -- veya kolonlara açılmış hâli; JSONB başlangıç için pratik

actuator_state(cabin_id FK, actuator TEXT, state BOOL, speed INT,
               source TEXT, updated_at, PRIMARY KEY(cabin_id, actuator))

readings(id BIGSERIAL, cabin_id FK, ts TIMESTAMPTZ,
         temperature, humidity, tds, ph,
         sht_ok, rtc_ok, tds_ok, ph_ok)
   -- INDEX (cabin_id, ts DESC). Ölçek için TimescaleDB hypertable.

alerts(id BIGSERIAL, cabin_id FK, ts, type, message)   -- opsiyonel
```
- **Retention:** readings hızlı büyür (30sn'de bir). Politika belirle: ham veriyi N gün tut + downsample (saatlik ort.) veya Timescale continuous aggregate.

---

## 7. PROJE YAPISI (öneri)

```
backend/
├── cmd/server/main.go
├── internal/
│   ├── domain/                 # saf: entities, VO, aggregate, invariant + unit test
│   │   ├── cabin/
│   │   ├── user/
│   │   └── telemetry/
│   ├── app/                    # use case'ler + port arayüzleri (interface)
│   │   ├── ports/              # inbound + outbound interface'ler
│   │   └── usecases/
│   └── infra/
│       ├── http/               # Gin handler, JWT, DTO
│       ├── mqtt/               # subscriber + publisher
│       ├── ws/                 # hub
│       └── postgres/           # sqlc/pgx repo implementasyonları
├── migrations/
├── db/query/                   # sqlc .sql
└── docker-compose.yml          # postgres + mosquitto
```

---

## 8. GÜVENLİK
- **Kullanıcı:** JWT access token, bcrypt parola. REST uçları JWT middleware arkasında; kullanıcı sadece sahip olduğu kabinleri görür/yönetir (owner kontrolü use case'te).
- **Cihaz/MQTT:** kabin başına broker kullanıcı/parola; **topic ACL** ile kabin yalnızca kendi `cabin/{kendi_id}/*` topic'lerine erişir (Mosquitto ACL veya dynamic-security). Backend tüm topic'lere yetkili tek servis.
- **online durumu:** LWT (`up/status` retained) ile güvenilir.

---

## 9. FAZLARA BÖLÜNMÜŞ ADIMLAR

```
ADIM 1  Proje iskeleti: go mod, docker-compose (postgres+mosquitto), config, /health, migration altyapısı
ADIM 2  Domain: User + Cabin aggregate + Thresholds/DecisionConfig VO + invariant'lar (UNIT TEST, infra yok)
ADIM 3  Auth: RegisterUser, Login, JWT middleware (Gin HTTP adapter)
ADIM 4  Cabin: CRUD + Claim akışı (HTTP) + PostgresAdapter (Cabin/User repo)
ADIM 5  MQTT subscriber: up/sensors + up/state + up/heartbeat(LWT) -> DB (readings + actuator_state + online)
ADIM 6  WebSocket hub + LiveBroadcastPort: yeni reading & state'i UI'ye broadcast
ADIM 7  MQTT publisher: SendActuatorCommand (down/command) + UpdateCabinConfig (down/config) + VO doğrulama
ADIM 8  GetSensorHistory (read model) + retention/downsample
ADIM 9  Güvenlik sertleştirme: MQTT auth + topic ACL, JWT yetki testleri
ADIM F  (FIRMWARE, paralel) ESP32'ye MQTT modülü ekle: knolleary/PubSubClient,
        Bölüm 2 kontratını uygula (config -> config_manager setter'ları; up/state firmware'deki
        actuator_manager'dan; sensors sensor_manager'dan; heartbeat). LWT ayarla.
```
Her ADIM sonunda: derle/çalıştır, ilgili testi geç, sonra devam.

---

## 10. MİMARİ DÜZELTMELERİN GEREKÇESİ (kullanıcının ilk taslağına göre)
1. **Time-series, Cabin aggregate'inin İÇİNDE değil.** Aggregate = tutarlılık sınırı + RAM'e yüklenen şey; binlerce ölçümle şişerse her işlem yavaşlar. → ayrı `ReadingStore`/read model.
2. **Telemetri domain'den geçmez.** Ham veri seli için DDD invariant'ı gereksiz/yavaş. → ayrı ingestion yolu.
3. **Aktüatör durumunun kaynağı cihaz.** Komut göndermek "uygulandı" demek değil. DB'yi cihazın `up/state`'iyle güncelle, komutta iyimser güncelleme yapma (röle takılırsa gerçeklikle ayrışır).
4. **Async yazımda bounded worker/channel** kullan; fire-and-forget goroutine + hatasız = veri kaybı/leak.

---

## 11. AÇIK / İLERİ NOTLAR
- Cihaz offline iken telemetri tamponlama firmware'de YOK (boşluklara tolerans göster).
- `thresholds`/`decision` JSONB mi kolonlar mı — başta JSONB pratik, sorgu ihtiyacı artarsa kolonlaştır.
- Mobil uygulama varsa WebSocket + REST aynı kontratı kullanır.
- Zaman: cihaz `ts`'i RTC'den gelir; RTC ayarsızsa 0 olabilir — backend gerekirse alış zamanını da damgalasın.

---

*Bağlam: Firmware (WiFi+provisioning dahil) tamamlandı. Bu doküman backend + firmware MQTT eklentisini tarif eder. Firmware tarafı detayları için Bölüm 2 tek gerçektir.*

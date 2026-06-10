# Hidroponik Akıllı Kabin — Backend (Go)

ESP32 hidroponik kabinleri için backend. Cihazdan MQTT ile telemetri/durum alır,
PostgreSQL'e yazar, WebSocket ile UI'ye canlı yayar; UI'den gelen manuel komut ve
eşik/karar konfigürasyonunu MQTT ile cihaza gönderir. Kullanıcı/kabin/yetki yönetimi içerir.

Mimari: **Hexagonal (Ports & Adapters) + DDD**. Karar motoru CİHAZDA çalışır;
backend yalnızca eşik günceller + manuel komut gönderir (bkz. `backend_plan.md`).

## Gereksinimler
- Go 1.24+
- Docker + docker-compose (PostgreSQL + Mosquitto)
- `sqlc` ve `golang-migrate` (kod üretimi + migration):
  ```
  go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
  go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  ```

## Hızlı başlangıç
```bash
# 1) Altyapıyı başlat (postgres + mosquitto)
make up

# 2) MQTT auth: kullanıcı/parola dosyasını üret (mosquitto auth ile çalışır)
./deploy/mosquitto/setup-auth.sh init                      # backend kullanıcısı
./deploy/mosquitto/setup-auth.sh add-cabin CAB-3778C4 devpass   # örnek cihaz

# 3) Migration'ları uygula
make migrate-up

# 4) Konfigürasyon
cp .env.example .env        # JWT_SECRET'i değiştir

# 5) Çalıştır
make run
```

`GET /health` → `{"status":"ok","db":"up"}`.

## Proje yapısı
```
cmd/server/            # composition root (main)
internal/
  domain/              # saf Go: cabin, user, telemetry (VO + aggregate + invariant)
  app/
    ports/             # inbound/outbound interface'ler
    usecases/          # use case implementasyonları
    apperr/            # sentinel hatalar
  infra/
    http/              # Gin handler + JWT middleware + DTO
    mqtt/              # paho subscriber (up/*) + publisher (down/*)
    ws/                # WebSocket hub (LiveBroadcastPort)
    postgres/          # pgx + sqlc repo'lar
    security/          # bcrypt + JWT
    config/            # viper
migrations/            # golang-migrate SQL
db/query/              # sqlc sorguları
deploy/mosquitto/      # broker config + ACL + auth script
```

## HTTP API
| Metot | Yol | Açıklama |
|---|---|---|
| GET | `/health` | sağlık + DB ping |
| POST | `/auth/register` | kayıt (`username,email,password`) |
| POST | `/auth/login` | giriş → JWT (`identifier,password`) |
| GET | `/api/me` | token sahibinin id'si |
| POST | `/api/cabins` | kabin oluştur (`id,name`) |
| GET | `/api/cabins` | sahip olunan kabinler |
| POST | `/api/cabins/claim` | kabin claim et (`id`) |
| GET | `/api/cabins/:id` | kabin detay (config + aktüatör + online) |
| POST | `/api/cabins/:id/command` | manuel aktüatör komutu |
| PUT | `/api/cabins/:id/config` | eşik/karar güncelle |
| GET | `/api/cabins/:id/readings` | sensör geçmişi (`from,to,limit,bucket=raw\|hour`) |
| GET | `/ws?token=&cabin_id=` | canlı yayın (WebSocket) |

`/api/*` uçları JWT (`Authorization: Bearer <token>`) arkasındadır ve kullanıcı yalnızca
sahip olduğu kabinlere erişebilir.

## MQTT kontratı
Topic ve payload'lar `backend_plan.md` Bölüm 2 ile birebir aynıdır:
- `cabin/{id}/up/{sensors|state|heartbeat|alert|status}` — cihaz → backend
- `cabin/{id}/down/{command|config}` — backend → cihaz

Güvenlik: anonim erişim kapalı; backend tüm `cabin/#` topic'lerine yetkili;
her cihaz yalnızca `cabin/{kendi_id}/#` alanına erişir (ACL pattern `cabin/%u/#`,
cihaz MQTT kullanıcı adı = kabin_id).

## Test / Sunum Arayüzü (`web/index.html`)
Backend, `WEB_DIR` (varsayılan `web`) dizinindeki tek sayfalık paneli `/` adresinden sunar.
Saf HTML/CSS/JS (Chart.js + Mermaid.js CDN). Sunucu çalışırken **http://localhost:8080** açılır.

İki aşamalıdır:
- **Aşama 1:** Giriş/Kayıt (sadece kullanıcı adı + şifre; e-posta arka planda türetilir). Kabin yoksa
  "kabin bekleniyor" ekranı gösterilir ve arka planda 4 sn'de bir yeni atama yoklanır. ESP32 üzerinden
  (heartbeat `user`) eşleştirme yapıldığı an **sayfa yenilenmeden** Aşama 2'ye geçer.
- **Aşama 2 (split-screen):** Sol panelde canlı telemetri (WebSocket), sensör geçmiş grafiği (CQRS read model),
  manuel aktüatör kontrolü + eşik/mod yapılandırması (REST) ve cihaz offline (LWT) için kırmızı uyarı bandı.
  Sağ panelde Mermaid ile hexagonal mimari diyagramı + sol taraftaki olaylarla **canlı parlayan** veri akışı
  (telemetri=yeşil, DB kaydı=mavi, komut/config=turuncu) ve teknik özet.

> Not: Chart.js ve Mermaid CDN'den yüklenir; sunum ortamında internet gerekir.

## Geliştirme komutları (Makefile)
```
make up / down / logs     # docker-compose
make build / run / test
make sqlc                 # sqlc generate
make migrate-up / migrate-down
make migrate-create name=...
```

## Notlar
- Aktüatör durumunun OTORİTER kaynağı cihazdır: backend DB'yi cihazın `up/state`'iyle
  günceller, komut gönderirken iyimser güncelleme yapmaz.
- Telemetri Cabin aggregate'inden geçmez; ayrı time-series yolu + read model (CQRS-vari).
- `READING_RETENTION_DAYS` ile ham telemetri retention; `bucket=hour` ile saatlik downsample.
- WebSocket için prod'da `ALLOWED_ORIGINS` set edilmelidir.
- Firmware MQTT modülü ayrı bir görevdir (`backend_plan.md` ADIM F).

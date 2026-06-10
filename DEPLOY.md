# Railway Deploy Kılavuzu — HidroBackend

## Gereken Ortam Değişkenleri

| Değişken | Örnek Değer | Açıklama |
|---|---|---|
| `HTTP_PORT` | `8080` | Sunucu portu (Railway otomatik `PORT` atar, ama default yeterli) |
| `DATABASE_URL` | `postgres://user:pass@host:5432/hidro` | PostgreSQL bağlantı adresi (Railway'de otomatik, **aşağıya bak**) |
| `JWT_SECRET` | `9903bdc90415d4981f...` | JWT imzalama anahtarı (rastgele 32+ byte hex) |
| `JWT_TTL` | `24h` | Token geçerlilik süresi |
| `MQTT_BROKER` | `tls://66a0bc1ce4434ff4a592d0a51e6f7db5.s1.eu.hivemq.cloud:8883` | MQTT broker (dev: `tcp://localhost:1883`, prod: `tls://host:8883`) — **host, HiveMQ Cloud konsolundaki aktif cluster ile birebir aynı olmalı; ESP32 ile aynı cluster!** |
| `MQTT_CLIENT_ID` | `hidro-backend` | MQTT istemci ID'si |
| `MQTT_USERNAME` | `backend` | MQTT kullanıcı adı (HiveMQ Cloud -> Access Management) |
| `MQTT_PASSWORD` | `sifre-buraya` | MQTT şifresi |
| `ALLOWED_ORIGINS` | _(boş bırak)_ | CORS/WebSocket origin kısıtlaması. Boş = aynı origin serbest, dev'de hepsi serbest. Prod'da aynı origin'den sunulduğu için boş yeterlidir. |
| `WEB_DIR` | `/app/web` | Statik UI dizini (Docker'da `/app/web`) |
| `MIGRATIONS_DIR` | `/app/migrations` | Migration SQL dosyalarının dizini (Docker'da `/app/migrations`) |
| `READING_RETENTION_DAYS` | `30` | Ham telemetri saklama süresi (gün) |
| `LOG_LEVEL` | `info` | Log seviyesi (`info`, `debug`, `warn`, `error`) |

## Railway'de Adım Adım Deploy

### 1. GitHub'a Push

Önce projeyi bir GitHub reposuna gönder (Railway GitHub'dan okuyacak):

```bash
git init
git add -A
git commit -m "Initial: HidroBackend"
git remote add origin git@github.com:KULLANICI/REPO.git
git push -u origin main
```

### 2. Railway'de Yeni Proje Oluştur

1. [Railway Dashboard](https://railway.app/dashboard)'a git
2. **New Project** → **Deploy from GitHub repo**
3. GitHub reponu seç (HidroBackend)
4. Railway Dockerfile'ı otomatik algılar

### 3. PostgreSQL Ekle

1. Proje dashboard'unda sağ üstte **+ New** → **Database** → **PostgreSQL**
2. Railway otomatik olarak `DATABASE_URL` environment variable'ını projeye enjekte eder
3. **ÖNEMLİ**: Kendi `.env`/env değişkenlerinde manuel `DATABASE_URL` tanımlama — Railway'inkiyle çakışır. Railway'in otomatik enjekte ettiği geçerli olur.

### 4. Ortam Değişkenlerini Gir

Proje dashboard'unda **Variables** sekmesine tıkla ve yukarıdaki tablodaki değişkenleri ekle:

- `JWT_SECRET` — yeni rastgele değer üret: `openssl rand -hex 32`
- `MQTT_BROKER` — `tls://66a0bc1ce4434ff4a592d0a51e6f7db5.s1.eu.hivemq.cloud:8883` (HiveMQ konsolundaki aktif cluster URL'si)
- `MQTT_USERNAME` — HiveMQ Cloud'dan aldığın kullanıcı adı
- `MQTT_PASSWORD` — HiveMQ Cloud'dan aldığın şifre
- `MQTT_CLIENT_ID` — `hidro-backend` (ya da benzersiz bir isim)
- `MIGRATIONS_DIR` — `/app/migrations`
- `WEB_DIR` — `/app/web`

`DATABASE_URL` ve `PORT`'u **elle ekleme** — Railway bunları otomatik yönetir.

### 5. Healthcheck (Otomatik)

Railway, `/health` endpoint'ini otomatik kontrol eder. Endpoint:
- `200 OK` — DB bağlantısı sağlıklı
- `503 Service Unavailable` — DB erişilemez

Dockerfile'da `EXPOSE ${HTTP_PORT}` ile port açıktır; Railway `PORT` enjekte ettiği için `HTTP_PORT` değişkeniyle otomatik eşleşir.

### 6. İlk Deploy

Railway, GitHub'a push yaptığında otomatik deploy alır. İlk deploy'da:
1. Dockerfile build edilir
2. Container başlatılır
3. Migration'lar otomatik uygulanır (tablolar `IF NOT EXISTS` ile oluşturulur)
4. `/health` başarılı olunca deploy tamamlanır

### 7. Bağlantı Türleri

| Ortam | MQTT Broker Şeması | Açıklama |
|---|---|---|
| Dev (docker-compose) | `tcp://localhost:1883` | Yerel Mosquitto, düz TCP |
| Prod (HiveMQ Cloud) | `tls://HOST:8883` | TLS, sistem kök sertifikaları kullanılır |

Uygulama, `tls://` / `ssl://` / `mqtts://` şemasını algılayıp TLS'yi otomatik açar. `tcp://` kullanılırsa TLS kapatılır.

### Notlar

- Web UI ve API aynı HTTPS origin'inden sunulur (`/` → `index.html`, `/api/...` → REST, `/ws` → WebSocket)
- WebSocket, `wss://AyniOrigin/ws?token=...&cabin_id=...` üzerinden bağlanır
- `ALLOWED_ORIGINS` boşken WebSocket aynı origin'den gelen bağlantıları kabul eder
- Docker image: Alpine tabanlı, `ca-certificates` paketi HiveMQ Cloud TLS için gömülü

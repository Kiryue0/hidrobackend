#!/usr/bin/env bash
# Sahte ESP32 cihazı: gerçek firmware MQTT modülü (ADIM F) yazılana kadar
# kabini verilen backend kullanıcısına CLAIM eder ve sürekli canlı telemetri yayar.
# Böylece dashboard'da Aşama 2 (split-screen) görülebilir.
#
# Kullanım:
#   ./deploy/sim/esp32-sim.sh <backend_kullanici_adi> [kabin_id] [kabin_parola]
#
# Örnek: önce panelden "melih" kullanıcısı oluştur, sonra:
#   ./deploy/sim/esp32-sim.sh melih
#
# Notlar:
#  - Varsayılan kabin CAB-3778C4 / devpass (deploy/mosquitto/setup-auth.sh ile eklendi).
#  - Farklı bir kabin kullanacaksan önce MQTT kullanıcısını ekle:
#      ./deploy/mosquitto/setup-auth.sh add-cabin CAB-ABCDEF <parola>
#  - Ctrl+C ile durdurur; dururken status'u offline bırakır (dashboard kırmızı banner gösterir).
set -euo pipefail

USER_NAME="${1:?Kullanım: $0 <backend_kullanici_adi> [kabin_id] [kabin_parola]}"
CABIN="${2:-CAB-3778C4}"
PASS="${3:-devpass}"
CONTAINER="${MOSQUITTO_CONTAINER:-hidro-mosquitto}"

pub(){ docker exec "$CONTAINER" mosquitto_pub -u "$CABIN" -P "$PASS" -t "cabin/$CABIN/$1" -m "$2" -q 1; }

cleanup(){ echo; echo "[sim] çevrimdışı bırakılıyor (LWT/offline)..."; pub "up/status" "offline" || true; exit 0; }
trap cleanup INT TERM

echo "[sim] ESP32 simülatörü: kabin=$CABIN -> kullanıcı='$USER_NAME'  (durdurmak için Ctrl+C)"

# 1) Çevrimiçi + claim (heartbeat'teki user backend tarafında kabini bu kullanıcıya atar)
pub "up/status" "online"
pub "up/heartbeat" "{\"ts\":0,\"fw\":\"sim-1.0\",\"ip\":\"10.0.0.9\",\"uptime\":0,\"user\":\"$USER_NAME\"}"
# 2) Başlangıç aktüatör durumu (otoriter kaynak cihaz)
pub "up/state" '{"ts":0,"humidifier":true,"havaMotoru":false,"cobLed":true,"fan1":120,"fan2":80,"source":"decision"}'
echo "[sim] claim + ilk durum yayınlandı. Telemetri akışı başlıyor (5 sn'de bir)..."

i=0
while true; do
  i=$((i+1))
  # Locale-bağımsız ondalık üretimi (virgül sorunu olmasın diye tam sayı + kesir)
  T="$(( 22 + RANDOM % 9 )).$(( RANDOM % 10 ))"      # ~22-30 °C
  H="$(( 55 + RANDOM % 25 )).$(( RANDOM % 10 ))"     # ~55-80 %
  TDS="$(( 800 + RANDOM % 500 ))"                    # ~800-1300 ppm
  PH="$(( 5 + RANDOM % 2 )).$(( RANDOM % 100 ))"     # ~5.x-6.x

  pub "up/sensors" "{\"ts\":0,\"t\":$T,\"h\":$H,\"tds\":$TDS,\"ph\":$PH,\"ok\":{\"sht\":true,\"rtc\":false,\"tds\":true,\"ph\":true}}"

  # Her ~30 sn'de bir heartbeat + ara sıra durum/uyarı (canlılık için)
  if [ $((i % 6)) -eq 0 ]; then
    pub "up/heartbeat" "{\"ts\":0,\"fw\":\"sim-1.0\",\"ip\":\"10.0.0.9\",\"uptime\":$((i*5)),\"user\":\"$USER_NAME\"}"
  fi
  if [ $((i % 10)) -eq 0 ]; then
    FAN=$(( 80 + RANDOM % 175 ))
    pub "up/state" "{\"ts\":0,\"humidifier\":true,\"havaMotoru\":false,\"cobLed\":true,\"fan1\":$FAN,\"fan2\":$((FAN-40)),\"source\":\"decision\"}"
  fi
  if [ $((i % 13)) -eq 0 ]; then
    pub "up/alert" '{"ts":0,"type":"UYARI","msg":"Nem ust esige yaklasti"}'
  fi

  sleep 5
done

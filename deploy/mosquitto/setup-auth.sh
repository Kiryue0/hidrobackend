#!/usr/bin/env bash
# Mosquitto kullanıcı/parola dosyasını üretir (passwd) ve cihaz kullanıcısı ekler.
# Cihaz kullanıcı adı == kabin_id olmalıdır (acl pattern'i %u'ya dayanır).
#
# Kullanım:
#   ./setup-auth.sh init                       # backend kullanıcısını oluşturur (dosyayı sıfırlar)
#   ./setup-auth.sh add-cabin CAB-3778C4 <pw>  # cihaz kullanıcısı ekler/günceller
#
# Ortam: hidro-mosquitto container'ı çalışıyor olmalı.
set -euo pipefail

CONTAINER="${MOSQUITTO_CONTAINER:-hidro-mosquitto}"
PASSWD="/mosquitto/config/passwd"

cmd="${1:-}"
case "$cmd" in
  init)
    BACKEND_PW="${BACKEND_MQTT_PASSWORD:-backendpass}"
    # -c dosyayı (yeniden) oluşturur
    docker exec "$CONTAINER" mosquitto_passwd -b -c "$PASSWD" backend "$BACKEND_PW"
    echo "backend kullanıcısı oluşturuldu (parola: $BACKEND_PW)"
    docker exec -u root "$CONTAINER" chown mosquitto:mosquitto "$PASSWD"
    docker exec "$CONTAINER" sh -c "kill -HUP 1" || true
    ;;
  add-cabin)
    cabin="${2:?kabin_id gerekli}"
    pw="${3:?parola gerekli}"
    docker exec "$CONTAINER" mosquitto_passwd -b "$PASSWD" "$cabin" "$pw"
    docker exec -u root "$CONTAINER" chown mosquitto:mosquitto "$PASSWD"
    echo "cihaz kullanıcısı eklendi: $cabin"
    docker exec "$CONTAINER" sh -c "kill -HUP 1" || true
    ;;
  *)
    echo "Kullanım: $0 {init | add-cabin <kabin_id> <parola>}" >&2
    exit 1
    ;;
esac

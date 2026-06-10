-- ADIM 1: Başlangıç şeması (backend_plan.md Bölüm 6).

CREATE TABLE IF NOT EXISTS users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      TEXT        NOT NULL UNIQUE,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS cabins (
    id            TEXT        PRIMARY KEY,                 -- kabin_id, format CAB-XXXXXX
    owner_user_id BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    name          TEXT        NOT NULL DEFAULT '',
    online        BOOLEAN     NOT NULL DEFAULT false,
    last_seen     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cabins_owner ON cabins(owner_user_id);

CREATE TABLE IF NOT EXISTS cabin_config (
    cabin_id   TEXT        PRIMARY KEY REFERENCES cabins(id) ON DELETE CASCADE,
    thresholds JSONB       NOT NULL,
    decision   JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS actuator_state (
    cabin_id   TEXT        NOT NULL REFERENCES cabins(id) ON DELETE CASCADE,
    actuator   TEXT        NOT NULL,                       -- HUMIDIFIER|HAVA_MOTORU|COB_LED|FAN1|FAN2
    state      BOOLEAN     NOT NULL DEFAULT false,
    speed      INTEGER     NOT NULL DEFAULT 0,             -- 0..255 (fanlar)
    source     TEXT        NOT NULL DEFAULT '',            -- button|serial|decision|backend
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (cabin_id, actuator)
);

CREATE TABLE IF NOT EXISTS readings (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cabin_id    TEXT        NOT NULL REFERENCES cabins(id) ON DELETE CASCADE,
    ts          TIMESTAMPTZ NOT NULL,
    temperature DOUBLE PRECISION,
    humidity    DOUBLE PRECISION,
    tds         DOUBLE PRECISION,
    ph          DOUBLE PRECISION,
    sht_ok      BOOLEAN     NOT NULL DEFAULT true,
    rtc_ok      BOOLEAN     NOT NULL DEFAULT true,
    tds_ok      BOOLEAN     NOT NULL DEFAULT true,
    ph_ok       BOOLEAN     NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_readings_cabin_ts ON readings(cabin_id, ts DESC);

CREATE TABLE IF NOT EXISTS alerts (
    id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    cabin_id TEXT        NOT NULL REFERENCES cabins(id) ON DELETE CASCADE,
    ts       TIMESTAMPTZ NOT NULL,
    type     TEXT        NOT NULL,                          -- KRITIK|UYARI|AKSIYON|NORMAL
    message  TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_alerts_cabin_ts ON alerts(cabin_id, ts DESC);

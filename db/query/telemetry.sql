-- name: InsertReading :exec
INSERT INTO readings (cabin_id, ts, temperature, humidity, tds, ph, sht_ok, rtc_ok, tds_ok, ph_ok)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: UpsertActuatorState :exec
INSERT INTO actuator_state (cabin_id, actuator, state, speed, source, updated_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (cabin_id, actuator)
DO UPDATE SET state = EXCLUDED.state,
              speed = EXCLUDED.speed,
              source = EXCLUDED.source,
              updated_at = now();

-- name: MarkCabinOnline :exec
UPDATE cabins SET online = true, last_seen = $2 WHERE id = $1;

-- name: MarkCabinOffline :exec
UPDATE cabins SET online = false WHERE id = $1;

-- name: EnsureCabin :exec
INSERT INTO cabins (id) VALUES ($1)
ON CONFLICT (id) DO NOTHING;

-- name: EnsureCabinConfig :exec
INSERT INTO cabin_config (cabin_id, thresholds, decision)
VALUES ($1, $2, $3)
ON CONFLICT (cabin_id) DO NOTHING;

-- name: InsertAlert :exec
INSERT INTO alerts (cabin_id, ts, type, message)
VALUES ($1, $2, $3, $4);

-- name: GetReadings :many
SELECT cabin_id, ts, temperature, humidity, tds, ph, sht_ok, rtc_ok, tds_ok, ph_ok
FROM readings
WHERE cabin_id = $1 AND ts >= $2 AND ts <= $3
ORDER BY ts DESC
LIMIT $4;

-- name: GetReadingsHourly :many
SELECT date_trunc('hour', ts)::timestamptz AS bucket,
       AVG(temperature)::float8 AS temperature,
       AVG(humidity)::float8    AS humidity,
       AVG(tds)::float8         AS tds,
       AVG(ph)::float8          AS ph,
       COUNT(*)::bigint         AS samples
FROM readings
WHERE cabin_id = $1 AND ts >= $2 AND ts <= $3
GROUP BY bucket
ORDER BY bucket DESC
LIMIT $4;

-- name: DeleteReadingsOlderThan :execrows
DELETE FROM readings WHERE ts < $1;

-- name: GetLatestReading :one
SELECT cabin_id, ts, temperature, humidity, tds, ph, sht_ok, rtc_ok, tds_ok, ph_ok
FROM readings
WHERE cabin_id = $1
ORDER BY ts DESC
LIMIT 1;

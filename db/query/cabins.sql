-- name: CreateCabin :one
INSERT INTO cabins (id, owner_user_id, name)
VALUES ($1, $2, $3)
RETURNING id, owner_user_id, name, online, last_seen, created_at;

-- name: InsertCabinConfig :exec
INSERT INTO cabin_config (cabin_id, thresholds, decision)
VALUES ($1, $2, $3);

-- name: GetCabin :one
SELECT id, owner_user_id, name, online, last_seen, created_at
FROM cabins WHERE id = $1;

-- name: ListCabinsByOwner :many
SELECT id, owner_user_id, name, online, last_seen, created_at
FROM cabins WHERE owner_user_id = $1
ORDER BY created_at;

-- name: GetCabinConfig :one
SELECT cabin_id, thresholds, decision, updated_at
FROM cabin_config WHERE cabin_id = $1;

-- name: ListActuatorStates :many
SELECT cabin_id, actuator, state, speed, source, updated_at
FROM actuator_state WHERE cabin_id = $1;

-- name: ClaimCabin :execrows
UPDATE cabins
SET owner_user_id = $2
WHERE id = $1 AND (owner_user_id IS NULL OR owner_user_id = $2);

-- name: CabinExists :one
SELECT EXISTS(SELECT 1 FROM cabins WHERE id = $1);

-- name: UpdateCabinConfig :exec
UPDATE cabin_config
SET thresholds = $2, decision = $3, updated_at = now()
WHERE cabin_id = $1;

-- name: DeleteCabinsByOwner :execrows
DELETE FROM cabins WHERE owner_user_id = $1;

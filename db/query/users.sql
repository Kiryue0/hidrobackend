-- name: CreateUser :one
INSERT INTO users (username, email, password_hash)
VALUES ($1, $2, $3)
RETURNING id, username, email, password_hash, created_at;

-- name: GetUserByID :one
SELECT id, username, email, password_hash, created_at
FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, email, password_hash, created_at
FROM users WHERE username = $1;

-- name: GetUserByEmail :one
SELECT id, username, email, password_hash, created_at
FROM users WHERE email = $1;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;

// Package postgres: outbound persistans adapter'ları (pgx + sqlc).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kiryue0/hidrobackend/internal/app/apperr"
	"github.com/kiryue0/hidrobackend/internal/domain/user"
	"github.com/kiryue0/hidrobackend/internal/infra/postgres/db"
)

// UserRepo ports.UserRepository implementasyonu.
type UserRepo struct {
	q *db.Queries
}

// NewUserRepo sqlc Queries ile repo üretir.
func NewUserRepo(q *db.Queries) *UserRepo {
	return &UserRepo{q: q}
}

func toDomainUser(row db.User) *user.User {
	return user.Reconstruct(row.ID, row.Username, row.Email, row.PasswordHash)
}

// isUniqueViolation Postgres 23505 (unique_violation) hatasını yakalar.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Create kullanıcıyı ekler; benzersizlik ihlalinde apperr.ErrConflict döner.
func (r *UserRepo) Create(ctx context.Context, u *user.User) error {
	row, err := r.q.CreateUser(ctx, db.CreateUserParams{
		Username:     u.Username(),
		Email:        u.Email(),
		PasswordHash: u.PasswordHash(),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return apperr.ErrConflict
		}
		return err
	}
	u.SetID(row.ID)
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*user.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return toDomainUser(row), nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	row, err := r.q.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return toDomainUser(row), nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, mapNotFound(err)
	}
	return toDomainUser(row), nil
}

// mapNotFound pgx.ErrNoRows -> apperr.ErrNotFound.
func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.ErrNotFound
	}
	return err
}

// Delete kullanıcıyı siler. Etkilenen satır yoksa (id geçersiz) ErrNotFound döner.
func (r *UserRepo) Delete(ctx context.Context, id int64) (int64, error) {
	n, err := r.q.DeleteUser(ctx, id)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, apperr.ErrNotFound
	}
	return n, nil
}

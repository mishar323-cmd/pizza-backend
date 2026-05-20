package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Admin struct {
	ID           int64  `json:"id"`
	Login        string `json:"login"`
	PasswordHash string `json:"-"`
	Name         string `json:"name"`
	Role         string `json:"role"`
}

type Admins struct{ pool *pgxpool.Pool }

func NewAdmins(pool *pgxpool.Pool) *Admins { return &Admins{pool: pool} }

func (r *Admins) GetByLogin(ctx context.Context, login string) (*Admin, error) {
	var a Admin
	err := r.pool.QueryRow(ctx,
		`SELECT id, login, password_hash, name, role FROM admins WHERE login=$1`, login,
	).Scan(&a.ID, &a.Login, &a.PasswordHash, &a.Name, &a.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Admins) Create(ctx context.Context, login, hash, name, role string) (*Admin, error) {
	var a Admin
	err := r.pool.QueryRow(ctx,
		`INSERT INTO admins(login, password_hash, name, role) VALUES($1,$2,$3,$4)
		 RETURNING id, login, password_hash, name, role`,
		login, hash, name, role,
	).Scan(&a.ID, &a.Login, &a.PasswordHash, &a.Name, &a.Role)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *Admins) Count(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM admins`).Scan(&n)
	return n, err
}

func (r *Admins) List(ctx context.Context) ([]Admin, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, login, password_hash, name, role FROM admins ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Admin
	for rows.Next() {
		var a Admin
		if err := rows.Scan(&a.ID, &a.Login, &a.PasswordHash, &a.Name, &a.Role); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

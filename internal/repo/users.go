package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID          int64      `json:"id"`
	Phone       string     `json:"phone"`
	Name        string     `json:"name"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
}

type Users struct{ pool *pgxpool.Pool }

func NewUsers(pool *pgxpool.Pool) *Users { return &Users{pool: pool} }

// GetOrCreate looks up a user by phone, creating one if it doesn't exist.
// Updates last_login_at on every call.
func (r *Users) GetOrCreate(ctx context.Context, phone string) (*User, error) {
	if phone == "" {
		return nil, errors.New("phone required")
	}
	u := &User{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users(phone, last_login_at)
		VALUES ($1, now())
		ON CONFLICT (phone) DO UPDATE SET last_login_at = now()
		RETURNING id, phone, COALESCE(name, ''), created_at, last_login_at`,
		phone,
	).Scan(&u.ID, &u.Phone, &u.Name, &u.CreatedAt, &u.LastLoginAt)
	return u, err
}

// Get returns the user by id or nil if missing.
func (r *Users) Get(ctx context.Context, id int64) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, phone, COALESCE(name, ''), created_at, last_login_at
		FROM users WHERE id=$1`,
		id,
	).Scan(&u.ID, &u.Phone, &u.Name, &u.CreatedAt, &u.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// UpdateName sets the display name (set after the user types it during checkout).
func (r *Users) UpdateName(ctx context.Context, id int64, name string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET name=$2 WHERE id=$1`, id, name)
	return err
}

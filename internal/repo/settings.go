package repo

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Settings struct{ pool *pgxpool.Pool }

func NewSettings(pool *pgxpool.Pool) *Settings { return &Settings{pool: pool} }

func (r *Settings) Get(ctx context.Context, key string) (json.RawMessage, error) {
	var v json.RawMessage
	err := r.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return v, err
}

func (r *Settings) Put(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO settings(key, value) VALUES($1, $2::jsonb)
		ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value, updated_at=now()
	`, key, string(raw))
	return err
}

func (r *Settings) GetRaw(ctx context.Context, key string, defaultJSON string) (json.RawMessage, error) {
	v, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return json.RawMessage(defaultJSON), nil
	}
	return v, nil
}

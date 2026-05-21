package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OTP struct {
	Phone     string
	CodeHash  string
	Channel   string
	Attempts  int
	SentAt    time.Time
	ExpiresAt time.Time
}

type OTPs struct{ pool *pgxpool.Pool }

func NewOTPs(pool *pgxpool.Pool) *OTPs { return &OTPs{pool: pool} }

// Upsert replaces any pending OTP for the phone with a fresh one.
func (r *OTPs) Upsert(ctx context.Context, phone, codeHash, channel string, ttl time.Duration) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO otp_codes(phone, code_hash, channel, attempts, sent_at, expires_at)
		VALUES ($1, $2, $3, 0, now(), now() + ($4 * INTERVAL '1 second'))
		ON CONFLICT (phone) DO UPDATE SET
			code_hash  = EXCLUDED.code_hash,
			channel    = EXCLUDED.channel,
			attempts   = 0,
			sent_at    = now(),
			expires_at = EXCLUDED.expires_at`,
		phone, codeHash, channel, int(ttl.Seconds()),
	)
	return err
}

// Get returns the active OTP for phone, or nil if absent/expired.
func (r *OTPs) Get(ctx context.Context, phone string) (*OTP, error) {
	o := &OTP{}
	err := r.pool.QueryRow(ctx, `
		SELECT phone, code_hash, channel, attempts, sent_at, expires_at
		FROM otp_codes WHERE phone=$1`,
		phone,
	).Scan(&o.Phone, &o.CodeHash, &o.Channel, &o.Attempts, &o.SentAt, &o.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return o, err
}

// IncrAttempts bumps the attempts counter; used after a wrong code submission.
func (r *OTPs) IncrAttempts(ctx context.Context, phone string) error {
	_, err := r.pool.Exec(ctx, `UPDATE otp_codes SET attempts = attempts + 1 WHERE phone=$1`, phone)
	return err
}

// Delete removes the OTP row (after successful verification or admin reset).
func (r *OTPs) Delete(ctx context.Context, phone string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM otp_codes WHERE phone=$1`, phone)
	return err
}

// PurgeExpired drops rows older than now. Call periodically.
func (r *OTPs) PurgeExpired(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM otp_codes WHERE expires_at < now()`)
	return err
}

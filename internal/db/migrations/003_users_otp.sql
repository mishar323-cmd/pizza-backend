-- Customers (phone-based auth) — separate from admins table.
CREATE TABLE IF NOT EXISTS users (
  id            BIGSERIAL PRIMARY KEY,
  phone         TEXT UNIQUE NOT NULL,         -- +7XXXXXXXXXX normalized
  name          TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_login_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS users_phone_idx ON users(phone);

-- Pending OTP challenges. One row per phone (latest wins). Hashed codes never
-- stored plaintext. Rate-limit fields (attempts, sent_at) prevent abuse.
CREATE TABLE IF NOT EXISTS otp_codes (
  phone        TEXT PRIMARY KEY,              -- +7XXXXXXXXXX normalized
  code_hash    TEXT NOT NULL,                 -- bcrypt
  channel      TEXT NOT NULL,                 -- 'call' | 'sms'
  attempts     INT  NOT NULL DEFAULT 0,
  sent_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS otp_codes_expires_idx ON otp_codes(expires_at);

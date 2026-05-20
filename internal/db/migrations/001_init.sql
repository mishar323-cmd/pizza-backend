CREATE TABLE IF NOT EXISTS admins (
  id BIGSERIAL PRIMARY KEY,
  login TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  name TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'admin',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
  id BIGSERIAL PRIMARY KEY,
  number INT,
  customer_name TEXT NOT NULL,
  customer_phone TEXT NOT NULL,
  address TEXT,
  zone TEXT,
  comment TEXT,
  receive_method TEXT NOT NULL,
  pay_method TEXT NOT NULL,
  delivery_time TEXT NOT NULL,
  items JSONB NOT NULL,
  total NUMERIC(10,2) NOT NULL,
  delivery NUMERIC(10,2) DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'new',
  eta_minutes INT DEFAULT 35,
  assigned_to TEXT,
  payment_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS orders_created_idx ON orders(created_at DESC);
CREATE INDEX IF NOT EXISTS orders_status_idx ON orders(status);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE SEQUENCE IF NOT EXISTS order_number_seq START 1001;

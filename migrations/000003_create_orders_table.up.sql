CREATE TABLE IF NOT EXISTS orders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number      TEXT NOT NULL UNIQUE,
    user_id     UUID NOT NULL REFERENCES users(id),
    status      TEXT NOT NULL DEFAULT 'NEW',
    accrual     BIGINT NULL,
    uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

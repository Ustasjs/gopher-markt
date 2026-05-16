CREATE TYPE order_status AS ENUM ('NEW', 'PROCESSING', 'PROCESSED', 'INVALID');

CREATE TABLE IF NOT EXISTS orders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    number      VARCHAR(50) NOT NULL UNIQUE,
    user_id     UUID NOT NULL REFERENCES users(id),
    status      order_status NOT NULL DEFAULT 'NEW',
    accrual     BIGINT NULL,
    uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

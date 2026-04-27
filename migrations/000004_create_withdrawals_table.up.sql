CREATE TABLE IF NOT EXISTS withdrawals (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id),
    order_number TEXT NOT NULL,
    sum          BIGINT NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

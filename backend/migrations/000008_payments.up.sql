CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    recurrent_payment_id UUID
        REFERENCES recurrent_payments(id) ON DELETE SET NULL,

    amount BIGINT NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL,
    description TEXT NOT NULL DEFAULT '',

    due_date DATE NOT NULL,
    paid_at TIMESTAMPTZ,

    status VARCHAR(10) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'overdue')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CHECK (
        (status = 'paid' AND paid_at IS NOT NULL)
        OR
        (status IN ('pending', 'overdue') AND paid_at IS NULL)
    )
);

CREATE INDEX idx_payments_user_id
    ON payments(user_id);

CREATE INDEX idx_payments_recurrent_payment_id
    ON payments(recurrent_payment_id);

CREATE INDEX idx_payments_user_status_due_date
    ON payments(user_id, status, due_date);
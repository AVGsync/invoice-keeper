CREATE TABLE IF NOT EXISTS user_recurrent_payments (
    user_id UUID NOT NULL
        REFERENCES users(id) ON DELETE CASCADE,

    recurrent_payment_id UUID NOT NULL
        REFERENCES recurrent_payments(id) ON DELETE CASCADE,

    PRIMARY KEY (user_id, recurrent_payment_id)
);

CREATE INDEX idx_user_recurrent_payments_payment_id
    ON user_recurrent_payments(recurrent_payment_id);
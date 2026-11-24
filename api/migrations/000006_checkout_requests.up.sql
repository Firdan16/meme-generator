CREATE TABLE checkout_requests (
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    idempotency_key text NOT NULL,
    payment_id      uuid NOT NULL REFERENCES payments (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, idempotency_key)
);

INSERT INTO checkout_requests (user_id, idempotency_key, payment_id)
SELECT s.user_id, p.idempotency_key, p.id
FROM payments p JOIN subscriptions s ON s.id = p.subscription_id
WHERE p.idempotency_key IS NOT NULL;

CREATE INDEX checkout_requests_payment_idx ON checkout_requests (payment_id);
CREATE INDEX payments_subscription_idx ON payments (subscription_id);

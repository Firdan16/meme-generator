BEGIN;

-- Checkout key uniqueness is scoped to the account by checkout_requests.
DROP INDEX payments_idempotency_key_idx;
CREATE INDEX payments_idempotency_key_idx
    ON payments (idempotency_key) WHERE idempotency_key IS NOT NULL;

-- Version 6 kept keys only in checkout_requests. For payments with multiple
-- aliases, pick a stable one because the aliases have no creation timestamps.
UPDATE payments p
SET idempotency_key = stored.key
FROM (
    SELECT payment_id, min(idempotency_key) AS key
    FROM checkout_requests
    GROUP BY payment_id
) stored
WHERE p.id = stored.payment_id AND p.idempotency_key IS NULL;

COMMIT;

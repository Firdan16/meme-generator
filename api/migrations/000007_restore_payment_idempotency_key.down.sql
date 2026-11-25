BEGIN;

DROP INDEX payments_idempotency_key_idx;
-- Keep all account-scoped mappings in checkout_requests while restoring
-- the old global index for payments.
WITH ranked AS (
    SELECT id, row_number() OVER (
        PARTITION BY idempotency_key ORDER BY created_at, id
    ) AS position
    FROM payments WHERE idempotency_key IS NOT NULL
)
UPDATE payments p
SET idempotency_key = NULL
FROM ranked
WHERE p.id = ranked.id AND ranked.position > 1;

CREATE UNIQUE INDEX payments_idempotency_key_idx
    ON payments (idempotency_key) WHERE idempotency_key IS NOT NULL;

COMMIT;

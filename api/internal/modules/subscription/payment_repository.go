package subscription

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ConfirmPayment atomically records a mock payment and starts its subscription.
func (r *Repository) ConfirmPayment(ctx context.Context, reference string) (Subscription, Payment, error) {
	var userID string
	err := r.pool.QueryRow(ctx, `
		SELECT s.user_id FROM payments p JOIN subscriptions s ON s.id = p.subscription_id
		WHERE p.reference = $1`, reference).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, Payment{}, ErrPaymentFailed
	}
	if err != nil {
		return Subscription{}, Payment{}, err
	}

	var res CheckoutResponse
	err = r.withUserLock(ctx, userID, func(tx pgx.Tx) error {
		var err error
		res, _, err = scanCheckout(tx.QueryRow(ctx, selectCheckout+" WHERE p.reference = $1 AND s.user_id = $2", reference, userID))
		if err != nil {
			return err
		}
		if res.Payment.Status == "paid" {
			return nil
		}
		if res.Payment.Status != "pending" || res.Subscription.Status != "pending" {
			return ErrPaymentFailed
		}
		if _, _, err := activeByUser(ctx, tx, userID); err == nil {
			return ErrAlreadySubscribed
		} else if !errors.Is(err, ErrNotSubscribed) {
			return err
		}

		const activate = `
			WITH paid AS (
				UPDATE payments SET status = 'paid', paid_at = statement_timestamp()
				WHERE id = $1 AND status = 'pending'
				RETURNING subscription_id, paid_at
			)
			UPDATE subscriptions s
			SET status = 'active', starts_at = paid.paid_at,
			    ends_at = paid.paid_at + make_interval(days => p.duration_days)
			FROM paid, plans p
			WHERE s.id = paid.subscription_id AND s.plan_id = p.id AND s.status = 'pending'
			RETURNING s.starts_at, s.ends_at`
		if err := tx.QueryRow(ctx, activate, res.Payment.ID).
			Scan(&res.Subscription.StartsAt, &res.Subscription.EndsAt); err != nil {
			return err
		}
		res.Payment.Status = "paid"
		res.Subscription.Status = "active"
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, Payment{}, ErrPaymentFailed
	}
	if err != nil {
		return Subscription{}, Payment{}, err
	}
	return res.Subscription, res.Payment, nil
}

package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/nyxsr/meme-portal/api/internal/platform/database"
)

func (r *Repository) withUserLock(ctx context.Context, userID string, run func(pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		if err := database.LockUser(ctx, tx, userID); err != nil {
			return err
		}
		return run(tx)
	})
}

func getPlanByCode(ctx context.Context, tx pgx.Tx, code string) (Plan, error) {
	const query = `
		SELECT id, code, name, price_idr, duration_days, max_memes, active
		FROM plans WHERE code = $1 AND active = true`
	var p Plan
	err := tx.QueryRow(ctx, query, code).Scan(&p.ID, &p.Code, &p.Name, &p.PriceIDR, &p.Duration, &p.MaxMemes, &p.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrPlanNotFound
	}
	return p, err
}

const selectCheckout = `
	SELECT s.id, s.user_id, s.plan_id, s.status, s.starts_at, s.ends_at,
	       p.id, p.subscription_id, p.amount_idr, p.status, p.reference, plan.code
	FROM payments p
	JOIN subscriptions s ON s.id = p.subscription_id
	JOIN plans plan ON plan.id = s.plan_id`

func scanCheckout(row pgx.Row) (CheckoutResponse, string, error) {
	var res CheckoutResponse
	var planCode string
	sub, pay := &res.Subscription, &res.Payment
	err := row.Scan(
		&sub.ID, &sub.UserID, &sub.PlanID, &sub.Status, &sub.StartsAt, &sub.EndsAt,
		&pay.ID, &pay.SubscriptionID, &pay.AmountIDR, &pay.Status, &pay.Reference, &planCode,
	)
	if err != nil {
		return CheckoutResponse{}, "", err
	}
	res.MockGatewayURL = mockConfirmationURL(pay.Reference)
	return res, planCode, nil
}

func checkoutByKey(ctx context.Context, tx pgx.Tx, userID, key string) (CheckoutResponse, string, error) {
	return scanCheckout(tx.QueryRow(ctx, selectCheckout+`
		JOIN checkout_requests cr ON cr.payment_id = p.id AND cr.user_id = s.user_id
		WHERE cr.user_id = $1 AND cr.idempotency_key = $2`, userID, key))
}

func pendingCheckout(ctx context.Context, tx pgx.Tx, userID, planID string) (CheckoutResponse, error) {
	res, _, err := scanCheckout(tx.QueryRow(ctx, selectCheckout+`
		WHERE s.user_id = $1 AND s.plan_id = $2 AND s.status = 'pending' AND p.status = 'pending'
		ORDER BY p.created_at, p.id LIMIT 1`, userID, planID))
	return res, err
}

func createCheckout(ctx context.Context, tx pgx.Tx, userID string, p Plan, reference string) (CheckoutResponse, error) {
	var res CheckoutResponse
	sub := &res.Subscription
	err := tx.QueryRow(ctx, `
		INSERT INTO subscriptions (user_id, plan_id, status)
		VALUES ($1, $2, 'pending')
		RETURNING id, user_id, plan_id, status, starts_at, ends_at`, userID, p.ID,
	).Scan(&sub.ID, &sub.UserID, &sub.PlanID, &sub.Status, &sub.StartsAt, &sub.EndsAt)
	if err != nil {
		return CheckoutResponse{}, fmt.Errorf("create subscription: %w", err)
	}
	pay := &res.Payment
	err = tx.QueryRow(ctx, `
		INSERT INTO payments (subscription_id, amount_idr, status, reference)
		VALUES ($1, $2, 'pending', $3)
		RETURNING id, subscription_id, amount_idr, status, reference`, sub.ID, p.PriceIDR, reference,
	).Scan(&pay.ID, &pay.SubscriptionID, &pay.AmountIDR, &pay.Status, &pay.Reference)
	if err != nil {
		return CheckoutResponse{}, fmt.Errorf("create payment: %w", err)
	}
	return res, nil
}

func recordCheckoutKey(ctx context.Context, tx pgx.Tx, userID, key, paymentID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO checkout_requests (user_id, idempotency_key, payment_id)
		VALUES ($1, $2, $3)`, userID, key, paymentID)
	return err
}

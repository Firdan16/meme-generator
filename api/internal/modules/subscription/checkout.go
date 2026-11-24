package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/nyxsr/meme-portal/api/internal/platform/token"
)

// Checkout serializes requests from one account, including requests from different tabs.
func (s *Service) Checkout(ctx context.Context, userID string, dto CheckoutDTO, key string) (CheckoutResponse, error) {
	var res CheckoutResponse
	err := s.repo.withUserLock(ctx, userID, func(tx pgx.Tx) error {
		previous, previousPlan, err := checkoutByKey(ctx, tx, userID, key)
		if err == nil {
			if previousPlan != dto.PlanCode {
				return ErrCheckoutKeyConflict
			}
			res = previous
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		plan, err := getPlanByCode(ctx, tx, dto.PlanCode)
		if err != nil {
			return err
		}
		if _, _, err := activeByUser(ctx, tx, userID); err == nil {
			return ErrAlreadySubscribed
		} else if !errors.Is(err, ErrNotSubscribed) {
			return err
		}

		res, err = pendingCheckout(ctx, tx, userID, plan.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			res, err = createCheckout(ctx, tx, userID, plan, token.RandomHex(12))
			if err == nil {
				res.MockGatewayURL, err = s.gateway.CreateCharge(ctx, res.Payment.AmountIDR, res.Payment.Reference)
			}
		}
		if err != nil {
			return fmt.Errorf("subscription service: checkout: %w", err)
		}
		return recordCheckoutKey(ctx, tx, userID, key, res.Payment.ID)
	})
	if err != nil {
		return CheckoutResponse{}, err
	}
	return res, nil
}

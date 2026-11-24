package subscription

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/testutil"
)

func TestCheckoutReusesPendingPaymentAcrossRequests(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	ctx := t.Context()
	service := NewService(NewPostgresRepository(pool), MockGateway{})
	input := CheckoutDTO{PlanCode: "premium_1d"}

	first, err := service.Checkout(ctx, userID, input, "checkout-first")
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		checkout CheckoutResponse
		err      error
	}
	const requests = 8
	results := make(chan result, requests)
	for i := range requests {
		go func() {
			key := fmt.Sprintf("checkout-retry-%d", i)
			res, err := service.Checkout(ctx, userID, input, key)
			results <- result{res, err}
		}()
	}
	for range requests {
		res := <-results
		if res.err != nil || res.checkout.Payment.ID != first.Payment.ID {
			t.Errorf("same pending checkout returned a different payment: %v", res.err)
		}
	}
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM payments p JOIN subscriptions s ON s.id = p.subscription_id
		WHERE s.user_id = $1`, userID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("saved payment count = %d, %v; want one", count, err)
	}
	if _, _, err := service.Confirm(ctx, first.Payment.Reference); err != nil {
		t.Fatal(err)
	}
	replayed, err := service.Checkout(ctx, userID, input, "checkout-retry-0")
	if err != nil || replayed.Payment.ID != first.Payment.ID || replayed.Payment.Status != "paid" {
		t.Fatalf("replay after payment = %+v, %v; want same paid payment", replayed.Payment, err)
	}
	if _, err := service.Checkout(ctx, userID, input, "a-new-checkout"); !errors.Is(err, ErrAlreadySubscribed) {
		t.Fatalf("new checkout while active = %v; want ErrAlreadySubscribed", err)
	}
}

func TestCheckoutKeysAreScopedToAccountAndPlan(t *testing.T) {
	pool := testutil.Postgres(t)
	firstUser, secondUser := testutil.User(t, pool), testutil.User(t, pool)
	service := NewService(NewPostgresRepository(pool), MockGateway{})
	ctx := t.Context()
	input := CheckoutDTO{PlanCode: "premium_1d"}
	first, err := service.Checkout(ctx, firstUser, input, "a-sample-intent")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Checkout(ctx, secondUser, input, "a-sample-intent")
	if err != nil || second.Subscription.UserID != secondUser || second.Payment.ID == first.Payment.ID {
		t.Fatal("the second account did not get its own checkout")
	}
	_, err = service.Checkout(ctx, firstUser, CheckoutDTO{PlanCode: "premium_3d"}, "a-sample-intent")
	if !errors.Is(err, ErrCheckoutKeyConflict) {
		t.Fatalf("same intent with another plan = %v; want checkout conflict", err)
	}
}

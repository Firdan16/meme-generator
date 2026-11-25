package subscription

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/testutil"
)

func TestCheckoutStoresOnePaymentForConcurrentReplays(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	ctx := t.Context()
	service := NewService(NewPostgresRepository(pool), MockGateway{})
	input := CheckoutDTO{PlanCode: "premium_1d"}
	const key = "same-checkout-intent"
	const requests = 8
	type checkoutResult struct {
		response CheckoutResponse
		err      error
	}
	start := make(chan struct{})
	results := make(chan checkoutResult, requests)

	for range requests {
		go func() {
			<-start
			response, err := service.Checkout(ctx, userID, input, key)
			results <- checkoutResult{response, err}
		}()
	}
	close(start)

	var paymentID string
	for range requests {
		result := <-results
		if result.err != nil {
			t.Errorf("checkout failed: %v", result.err)
			continue
		}
		if paymentID == "" {
			paymentID = result.response.Payment.ID
		}
		if result.response.Payment.ID != paymentID {
			t.Errorf("got payment %q, want %q", result.response.Payment.ID, paymentID)
		}
	}

	var total, withKey int
	err := pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE p.idempotency_key = $2)
		FROM payments p JOIN subscriptions s ON s.id = p.subscription_id
		WHERE s.user_id = $1`, userID, key).Scan(&total, &withKey)
	if err != nil || total != 1 || withKey != 1 {
		t.Fatalf("saved payments: total=%d with requested key=%d, %v; want one of each", total, withKey, err)
	}
}

func TestCheckoutReusesPendingPaymentAcrossRequests(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	ctx := t.Context()
	service := NewService(NewPostgresRepository(pool), MockGateway{})
	input := CheckoutDTO{PlanCode: "premium_1d"}
	const firstKey = "checkout-first"

	first, err := service.Checkout(ctx, userID, input, firstKey)
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
	var count, withFirstKey int
	err = pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE p.idempotency_key = $2)
		FROM payments p JOIN subscriptions s ON s.id = p.subscription_id
		WHERE s.user_id = $1`, userID, firstKey).Scan(&count, &withFirstKey)
	if err != nil || count != 1 || withFirstKey != 1 {
		t.Fatalf("saved payments after alias replays: total=%d with original key=%d, %v; want one of each", count, withFirstKey, err)
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

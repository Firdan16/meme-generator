package subscription

import (
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/testutil"
)

func TestMockPaymentConfirmationCanBeSafelyRetried(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	ctx := t.Context()
	service := NewService(NewPostgresRepository(pool), MockGateway{})
	checkout, err := service.Checkout(ctx, userID, CheckoutDTO{PlanCode: "premium_1d"}, "confirm-fixture")
	if err != nil {
		t.Fatal(err)
	}
	first, pay, err := service.Confirm(ctx, checkout.Payment.Reference)
	if err != nil || pay.Status != "paid" || first.EndsAt == nil || first.StartsAt == nil {
		t.Fatalf("first confirmation = %+v, %+v, %v", first, pay, err)
	}

	retry, retryPay, err := service.Confirm(ctx, checkout.Payment.Reference)
	if err != nil {
		t.Fatalf("repeating the confirmed payment = %v; want its original result", err)
	}
	if retry.ID != first.ID || retryPay.ID != pay.ID || retry.StartsAt == nil || retry.EndsAt == nil ||
		!retry.StartsAt.Equal(*first.StartsAt) || !retry.EndsAt.Equal(*first.EndsAt) {
		t.Fatal("payment retry changed the original subscription period")
	}
}

package subscription

import (
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/testutil"
)

func TestStatusIgnoresExpiredSubscriptionBeforeCleanup(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	ctx := t.Context()
	var subID string
	err := pool.QueryRow(ctx, `
		INSERT INTO subscriptions (user_id, plan_id, status, starts_at, ends_at)
		SELECT $1, id, 'active', now() - interval '1 hour', now() + interval '1 hour'
		FROM plans WHERE code = 'premium_1d'
		RETURNING id`, userID).Scan(&subID)
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(NewPostgresRepository(pool), MockGateway{})
	if status, err := service.Status(ctx, userID); err != nil || !status.Subscribed {
		t.Fatalf("unexpired subscription = %+v, %v; want subscribed", status, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE subscriptions SET ends_at = now() WHERE id = $1", subID); err != nil {
		t.Fatal(err)
	}
	if status, err := service.Status(ctx, userID); err != nil || status.Subscribed {
		t.Fatalf("expired subscription = %+v, %v; want unsubscribed before cleanup", status, err)
	}
}

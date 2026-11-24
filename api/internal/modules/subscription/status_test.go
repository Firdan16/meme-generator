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

func TestStatusUsesActualSubscriptionStartForQuota(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	ctx := t.Context()
	if _, err := pool.Exec(ctx, `
		INSERT INTO subscriptions (user_id, plan_id, status, starts_at, ends_at)
		SELECT $1, id, 'active', now() - interval '1 minute', now() + interval '1 day'
		FROM plans WHERE code = 'premium_1d'`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO memes (user_id, base_url, created_at)
		VALUES ($1, 'base/previous.png', now() - interval '2 minutes'),
		       ($1, 'base/current.png', now())`, userID); err != nil {
		t.Fatal(err)
	}
	status, err := NewService(NewPostgresRepository(pool), MockGateway{}).Status(ctx, userID)
	if err != nil || !status.Subscribed || status.MemesUsed != 1 {
		t.Fatalf("current-period quota = %d, %v; want one meme", status.MemesUsed, err)
	}
}

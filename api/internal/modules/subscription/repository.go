package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nyxsr/meme-portal/api/internal/db"
)

// Repository akses data langganan & pembayaran.
type Repository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListPlans(ctx context.Context) ([]Plan, error) {
	const q = `
		SELECT id, code, name, price_idr, duration_days, max_memes, active
		FROM plans WHERE active = true ORDER BY price_idr ASC`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("subscription repository: list plans gagal: %w", err)
	}
	defer rows.Close()
	var plans []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.PriceIDR, &p.Duration, &p.MaxMemes, &p.Active); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (r *Repository) ActiveByUser(ctx context.Context, userID string) (Subscription, Plan, error) {
	return activeByUser(ctx, r.pool, userID)
}

func activeByUser(ctx context.Context, q db.DBTX, userID string) (Subscription, Plan, error) {
	const query = `
		SELECT s.id, s.user_id, s.plan_id, s.status,
		       COALESCE(s.starts_at, s.ends_at - make_interval(days => p.duration_days)), s.ends_at,
		       p.id, p.code, p.name, p.price_idr, p.duration_days, p.max_memes, p.active,
		       statement_timestamp()
		FROM subscriptions s
		JOIN plans p ON p.id = s.plan_id
		WHERE s.user_id = $1 AND s.status = 'active' AND s.ends_at > statement_timestamp()
		  AND (s.starts_at IS NULL OR s.starts_at <= statement_timestamp())
		ORDER BY s.ends_at DESC LIMIT 1`
	var (
		sub Subscription
		p   Plan
	)
	err := q.QueryRow(ctx, query, userID).Scan(
		&sub.ID, &sub.UserID, &sub.PlanID, &sub.Status, &sub.StartsAt, &sub.EndsAt,
		&p.ID, &p.Code, &p.Name, &p.PriceIDR, &p.Duration, &p.MaxMemes, &p.Active,
		&sub.checkedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, Plan{}, ErrNotSubscribed
	}
	if err != nil {
		return Subscription{}, Plan{}, fmt.Errorf("subscription repository: active by user gagal: %w", err)
	}
	return sub, p, nil
}

// MarkExpired menandai semua langganan aktif yang sudah lewat ends_at
// menjadi 'expired'. Dipanggil janitor periodik; lazy check di
// ActiveByUser (ends_at > now()) tetap menjadi sumber kebenaran runtime.
func (r *Repository) MarkExpired(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`UPDATE subscriptions SET status = 'expired'
		 WHERE status = 'active' AND ends_at <= now()`)
	if err != nil {
		return 0, fmt.Errorf("subscription repository: mark expired gagal: %w", err)
	}
	return tag.RowsAffected(), nil
}

// memesUsed counts reservations in this subscription period.
func memesUsed(ctx context.Context, q db.DBTX, userID string, sub Subscription) (int32, error) {
	var n int32
	err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM memes WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`,
		userID, sub.StartsAt, sub.EndsAt,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("subscription repository: memes used gagal: %w", err)
	}
	return n, nil
}

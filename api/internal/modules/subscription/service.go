package subscription

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// PaymentGateway abstraksi payment provider; saat ini mock.
type PaymentGateway interface {
	CreateCharge(ctx context.Context, amount int64, reference string) (checkoutURL string, err error)
}

// MockGateway mensimulasikan payment gateway: charge langsung "berhasil dibuat"
// dan konfirmasi dilakukan lewat endpoint confirm milik API sendiri.
type MockGateway struct{}

func (MockGateway) CreateCharge(_ context.Context, _ int64, reference string) (string, error) {
	return mockConfirmationURL(reference), nil
}

func mockConfirmationURL(reference string) string {
	return "/api/v1/subscription/payments/" + reference + "/confirm"
}

type Service struct {
	repo    *Repository
	gateway PaymentGateway
}

func NewService(repo *Repository, gateway PaymentGateway) *Service {
	return &Service{repo: repo, gateway: gateway}
}

func (s *Service) ListPlans(ctx context.Context) ([]Plan, error) {
	return s.repo.ListPlans(ctx)
}

// Confirm mensimulasikan callback payment gateway sukses.
func (s *Service) Confirm(ctx context.Context, reference string) (Subscription, Payment, error) {
	return s.repo.ConfirmPayment(ctx, reference)
}

// MarkExpired menandai langganan aktif yang sudah lewat masa aktif
// (dipanggil janitor periodik).
func (s *Service) MarkExpired(ctx context.Context) (int64, error) {
	return s.repo.MarkExpired(ctx)
}

func (s *Service) Status(ctx context.Context, userID string) (SubscriptionStatus, error) {
	sub, plan, err := s.repo.ActiveByUser(ctx, userID)
	if err == nil {
		used, err := memesUsed(ctx, s.repo.pool, userID, sub)
		if err != nil {
			return SubscriptionStatus{}, err
		}
		var endsAt *string
		if sub.EndsAt != nil {
			s := sub.EndsAt.UTC().Format(time.RFC3339Nano)
			endsAt = &s
		}
		return SubscriptionStatus{
			Subscribed: true,
			Status:     sub.Status,
			PlanCode:   plan.Code,
			PlanName:   plan.Name,
			EndsAt:     endsAt,
			MemesUsed:  used,
			MaxMemes:   plan.MaxMemes,
		}, nil
	}
	if errors.Is(err, ErrNotSubscribed) {
		return SubscriptionStatus{}, nil
	}
	return SubscriptionStatus{}, err
}

func (s *Service) RequireActive(ctx context.Context, userID string) error {
	_, _, err := s.repo.ActiveByUser(ctx, userID)
	return err
}

// CheckMemeQuota runs inside the same locked transaction as the new reservation.
func (s *Service) CheckMemeQuota(ctx context.Context, tx pgx.Tx, userID string) (time.Time, error) {
	sub, plan, err := activeByUser(ctx, tx, userID)
	if err != nil {
		return time.Time{}, err
	}
	if plan.MaxMemes > 0 {
		used, err := memesUsed(ctx, tx, userID, sub)
		if err != nil {
			return time.Time{}, err
		}
		if used >= plan.MaxMemes {
			return time.Time{}, ErrQuotaExhausted
		}
	}
	return sub.checkedAt, nil
}

package meme

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5"

	"github.com/nyxsr/meme-portal/api/internal/config"
)

// Gate separates a new quota reservation from work on an existing meme.
type Gate interface {
	CheckMemeQuota(ctx context.Context, tx pgx.Tx, userID string) (time.Time, error)
	RequireActive(ctx context.Context, userID string) error
}

type Service struct {
	repo      *Repository
	s3        *s3.Client
	bucket    string
	gate      Gate
	uploadKey [32]byte
}

func NewService(repo *Repository, s3Client *s3.Client, cfg config.StorageConfig, gate Gate, secret string) *Service {
	return &Service{
		repo: repo, s3: s3Client, bucket: cfg.Bucket, gate: gate,
		uploadKey: sha256.Sum256([]byte("meme-upload-v1:" + secret)),
	}
}

func (s *Service) List(ctx context.Context, userID string, limit, offset int32) (ListResponse, error) {
	items, total, err := s.repo.ListByUser(ctx, userID, limit, offset)
	if err != nil {
		return ListResponse{}, err
	}
	if items == nil {
		items = []Meme{}
	}
	return ListResponse{Items: items, Total: total}, nil
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, id, userID)
}

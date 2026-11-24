package meme

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/golang-jwt/jwt/v5"
	_ "golang.org/x/image/webp"

	"github.com/nyxsr/meme-portal/api/internal/platform/token"
)

const uploadAudience = "meme-image-upload"

type preparedUpload struct {
	id    string
	grant string
}

type uploadClaims struct {
	MemeID string `json:"memeId"`
	Kind   string `json:"kind"`
	jwt.RegisteredClaims
}

func (s *Service) prepareGrant(userID, id, kind string) (preparedUpload, error) {
	claims := uploadClaims{
		MemeID: id,
		Kind:   kind,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Audience:  jwt.ClaimStrings{uploadAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(uploadTTL)),
		},
	}
	grant, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.uploadKey[:])
	if err != nil {
		return preparedUpload{}, fmt.Errorf("meme service: sign upload grant: %w", err)
	}
	return preparedUpload{id: id, grant: grant}, nil
}

func (s *Service) PrepareUpload(ctx context.Context, userID, title, ext, contentType string) (preparedUpload, error) {
	expectedType := imageContentType(ext)
	if expectedType == "" {
		return preparedUpload{}, ErrInvalidFile
	}
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != expectedType {
			return preparedUpload{}, ErrInvalidFile
		}
	}
	if err := s.gate.CanCreate(ctx, userID); err != nil {
		return preparedUpload{}, err
	}
	m, err := s.repo.Create(ctx, Meme{
		UserID:       userID,
		Title:        title,
		BaseImageURL: baseImage + "/" + token.RandomHex(12) + strings.ToLower(ext),
	})
	if err != nil {
		return preparedUpload{}, err
	}
	return s.prepareGrant(userID, m.ID, baseImage)
}

func (s *Service) ResultUpload(ctx context.Context, userID, id string) (preparedUpload, error) {
	if err := s.gate.CanCreate(ctx, userID); err != nil {
		return preparedUpload{}, err
	}
	m, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return preparedUpload{}, err
	}
	return s.prepareGrant(userID, m.ID, resultImage)
}

func uploadResponse(r *http.Request, upload preparedUpload, kind string) UploadResponse {
	u := requestURL(r, "/api/v1/memes/"+upload.id+"/images/"+kind)
	query := u.Query()
	query.Set("grant", upload.grant)
	u.RawQuery = query.Encode()
	return UploadResponse{ID: upload.id, UploadURL: u.String(), BaseURL: imageURL(r, upload.id, kind)}
}

func (s *Service) uploadKeyForGrant(ctx context.Context, id, kind, raw string) (string, error) {
	var claims uploadClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(_ *jwt.Token) (any, error) {
		return s.uploadKey[:], nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(), jwt.WithAudience(uploadAudience))
	if err != nil || claims.Subject == "" || claims.MemeID != id || claims.Kind != kind {
		return "", ErrUploadGrant
	}
	if err := s.gate.CanCreate(ctx, claims.Subject); err != nil {
		return "", err
	}
	m, err := s.repo.GetByID(ctx, id, claims.Subject)
	if err != nil {
		return "", err
	}
	switch kind {
	case baseImage:
		return baseObjectKey(m.BaseImageURL)
	case resultImage:
		return resultObjectKey(m.ID), nil
	default:
		return "", ErrUploadGrant
	}
}

func (h *Handler) UploadImage(w http.ResponseWriter, r *http.Request) {
	key, err := h.service.uploadKeyForGrant(r.Context(), r.PathValue("id"), r.PathValue("kind"), r.URL.Query().Get("grant"))
	if err != nil {
		writeError(w, err)
		return
	}
	if r.ContentLength > maxUploadBytes {
		writeError(w, ErrFileTooLarge)
		return
	}
	body := http.MaxBytesReader(w, r.Body, maxUploadBytes)
	defer body.Close()
	contentType := imageContentType(path.Ext(key))
	data, err := readImage(body, contentType)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.service.storeImage(r.Context(), key, contentType, data); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readImage(body io.Reader, expectedType string) ([]byte, error) {
	data, err := io.ReadAll(body)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return nil, ErrFileTooLarge
	}
	if err != nil || http.DetectContentType(data) != expectedType {
		return nil, ErrInvalidFile
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxImagePixels/cfg.Height {
		return nil, ErrInvalidFile
	}
	return data, nil
}

func (s *Service) storeImage(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &key,
		Body:          bytes.NewReader(data),
		ContentType:   &contentType,
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return fmt.Errorf("meme service: store image: %w", err)
	}
	return nil
}

func (s *Service) Complete(ctx context.Context, userID, id string) (Meme, error) {
	if err := s.gate.CanCreate(ctx, userID); err != nil {
		return Meme{}, err
	}
	m, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return Meme{}, err
	}
	key := resultObjectKey(m.ID)
	_, err = s.s3.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	var missing *types.NotFound
	if errors.As(err, &missing) {
		return Meme{}, ErrUploadIncomplete
	}
	if err != nil {
		return Meme{}, fmt.Errorf("meme service: check result: %w", err)
	}
	return s.repo.SetResultURL(ctx, m.ID, userID, key)
}

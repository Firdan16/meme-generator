package meme

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/nyxsr/meme-portal/api/internal/middleware"
	"github.com/nyxsr/meme-portal/api/internal/response"
)

const (
	baseImage   = "base"
	resultImage = "result"
)

func resultObjectKey(id string) string {
	return resultImage + "/" + id + ".png"
}

func imageContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	return ""
}

func baseObjectKey(location string) (string, error) {
	// Old rows stored the full Garage website URL. Only the key is used.
	u, err := url.Parse(location)
	if err != nil || path.Base(path.Dir(u.Path)) != baseImage || imageContentType(path.Ext(u.Path)) == "" {
		return "", ErrNotFound
	}
	return baseImage + "/" + path.Base(u.Path), nil
}

func (s *Service) image(ctx context.Context, userID, id, kind string) (*s3.GetObjectOutput, string, error) {
	m, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, "", err
	}
	var key string
	switch kind {
	case baseImage:
		key, err = baseObjectKey(m.BaseImageURL)
		if err != nil {
			return nil, "", err
		}
	case resultImage:
		if m.ResultURL == nil {
			return nil, "", ErrNotFound
		}
		key = resultObjectKey(m.ID)
	default:
		return nil, "", ErrNotFound
	}
	object, err := s.s3.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	var missing *types.NoSuchKey
	if errors.As(err, &missing) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("meme service: read image: %w", err)
	}
	return object, imageContentType(path.Ext(key)), nil
}

func requestURL(r *http.Request, resourcePath string) url.URL {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return url.URL{Scheme: scheme, Host: r.Host, Path: resourcePath}
}

func imageURL(r *http.Request, id, kind string) string {
	u := requestURL(r, "/api/v1/memes/"+id+"/images/"+kind)
	return u.String()
}

func withImageURLs(r *http.Request, m Meme) Meme {
	m.BaseImageURL = imageURL(r, m.ID, baseImage)
	if m.ResultURL != nil {
		u := imageURL(r, m.ID, resultImage)
		m.ResultURL = &u
	}
	return m
}

func (h *Handler) ViewImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "unauthorized")
		return
	}
	object, contentType, err := h.service.image(r.Context(), userID, r.PathValue("id"), r.PathValue("kind"))
	if err != nil {
		writeError(w, err)
		return
	}
	defer object.Body.Close()
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if object.ContentLength != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(*object.ContentLength, 10))
	}
	if r.Method != http.MethodHead {
		if _, err := io.Copy(w, object.Body); err != nil && r.Context().Err() == nil {
			slog.Warn("meme handler: stream image", "err", err)
		}
	}
}

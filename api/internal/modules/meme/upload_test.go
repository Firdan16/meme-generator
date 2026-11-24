package meme

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/nyxsr/meme-portal/api/internal/config"
	"github.com/nyxsr/meme-portal/api/internal/middleware"
	"github.com/nyxsr/meme-portal/api/internal/platform/token"
	"github.com/nyxsr/meme-portal/api/internal/testutil"
)

type allowFixtureUploads struct{}

func (allowFixtureUploads) CanCreate(context.Context, string) error { return nil }

func TestImageReaderValidatesFormatAndBodyLimit(t *testing.T) {
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}

	if got, err := readImage(bytes.NewReader(pngBytes.Bytes()), "image/png"); err != nil || len(got) != pngBytes.Len() {
		t.Fatalf("valid PNG read = %d bytes, %v", len(got), err)
	}
	if _, err := readImage(strings.NewReader("plain fixture text"), "image/png"); !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("non-image data: %v; want ErrInvalidFile", err)
	}
	if _, err := readImage(bytes.NewReader(pngBytes.Bytes()[:8]), "image/png"); !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("truncated PNG header: %v; want ErrInvalidFile", err)
	}
	body := http.MaxBytesReader(httptest.NewRecorder(), io.NopCloser(bytes.NewReader(pngBytes.Bytes())), 32)
	defer body.Close()
	if _, err := readImage(body, "image/png"); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("body exceeding limit: %v; want ErrFileTooLarge", err)
	}
}

func TestPrepareImageReturnsValidationEndpoint(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	ctx := t.Context()
	tokens := token.NewManager("fixture jwt key", "test", time.Minute, time.Hour, nil)
	pair, err := tokens.Issue(ctx, userID, "user")
	if err != nil {
		t.Fatal(err)
	}
	s3Client := s3.New(s3.Options{
		Region:       "test",
		BaseEndpoint: aws.String("http://storage.example.test"),
		Credentials:  credentials.NewStaticCredentialsProvider("fixture-access", "fixture-secret", ""),
		UsePathStyle: true,
	})
	handler := NewModule(pool, s3Client, config.StorageConfig{Bucket: "fixture"}, allowFixtureUploads{}, "fixture jwt key")
	mux := http.NewServeMux()
	RegisterRoutes(mux, handler, middleware.Auth(tokens))

	req := httptest.NewRequest(http.MethodPost, "http://api.example.test/api/v1/memes/upload", strings.NewReader(
		`{"title":"example","filename":"meme.png","contentType":"image/png","size":80}`,
	))
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("prepare a PNG = %d, %s; want 201", rec.Code, rec.Body)
	}
	var envelope struct{ Data UploadResponse }
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	upload, err := url.Parse(envelope.Data.UploadURL)
	if err != nil || upload.Host != req.Host {
		t.Fatal("prepared image did not go through the API validation endpoint")
	}
}

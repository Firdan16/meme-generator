package subscription

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nyxsr/meme-portal/api/internal/middleware"
	"github.com/nyxsr/meme-portal/api/internal/platform/token"
	"github.com/nyxsr/meme-portal/api/internal/testutil"
)

func TestSubscriptionStatusIsNotCacheable(t *testing.T) {
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	tokens := token.NewManager("test key", "meme-test", time.Minute, time.Hour, nil)
	pair, err := tokens.Issue(t.Context(), userID, "user")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewModule(pool), middleware.Auth(tokens))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscription/status", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("private subscription status Cache-Control = %q; want private, no-store", got)
	}

	guest := httptest.NewRecorder()
	mux.ServeHTTP(guest, httptest.NewRequest(http.MethodGet, "/api/v1/subscription/status", nil))
	if guest.Code != http.StatusUnauthorized || guest.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("guest response = %d, %q; want 401 and private, no-store", guest.Code, guest.Header().Get("Cache-Control"))
	}
}

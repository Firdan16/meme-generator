package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckoutPreflightAllowsIdempotencyKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/subscription/checkout", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type,idempotency-key")
	rec := httptest.NewRecorder()
	CORS("http://localhost:3000")(http.NotFoundHandler()).ServeHTTP(rec, req)
	allowed := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers"))
	if rec.Code != http.StatusNoContent || !strings.Contains(allowed, "idempotency-key") {
		t.Fatalf("preflight = %d with %q; want 204 permitting idempotency-key", rec.Code, allowed)
	}
}

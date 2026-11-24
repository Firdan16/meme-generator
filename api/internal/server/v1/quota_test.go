package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nyxsr/meme-portal/api/internal/config"
	"github.com/nyxsr/meme-portal/api/internal/modules/meme"
	"github.com/nyxsr/meme-portal/api/internal/platform/token"
	"github.com/nyxsr/meme-portal/api/internal/response"
	"github.com/nyxsr/meme-portal/api/internal/testutil"
)

type quotaFixture struct {
	pool   *pgxpool.Pool
	userID string
	token  string
	max    int
	mux    *http.ServeMux
}

func setupQuota(t *testing.T) quotaFixture {
	t.Helper()
	pool := testutil.Postgres(t)
	userID := testutil.User(t, pool)
	ctx := t.Context()
	var limit int
	if err := pool.QueryRow(ctx, `
		WITH plan AS (SELECT id, max_memes FROM plans WHERE code = 'premium_1d'),
		active AS (
			INSERT INTO subscriptions (user_id, plan_id, status, starts_at, ends_at)
			SELECT $1, id, 'active', now() - interval '1 hour', now() + interval '1 hour' FROM plan
			RETURNING plan_id
		)
		SELECT plan.max_memes FROM plan JOIN active ON active.plan_id = plan.id`, userID).Scan(&limit); err != nil {
		t.Fatal(err)
	}
	if limit <= 0 {
		t.Fatal("fixture plan must have a finite quota")
	}
	cfg := config.Config{JWT: config.JWTConfig{Secret: "fixture jwt secret"}}
	tokens := token.NewManager(cfg.JWT.Secret, "fixture", time.Minute, time.Hour, nil)
	pair, err := tokens.Issue(ctx, userID, "user")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Mount(mux, pool, nil, nil, tokens, cfg)
	return quotaFixture{pool: pool, userID: userID, token: pair.AccessToken, max: limit, mux: mux}
}

func (f quotaFixture) request(method, path, jsonBody string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(jsonBody))
	req.Header.Set("Authorization", "Bearer "+f.token)
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

const validDraft = `{"title":"fixture","filename":"meme.png","contentType":"image/png","size":80}`

func TestLastReservedSlotCanRequestResultUpload(t *testing.T) {
	f := setupQuota(t)
	if _, err := f.pool.Exec(t.Context(), `
		INSERT INTO memes (user_id, base_url)
		SELECT $1, 'base/fixture.png' FROM generate_series(1, $2::int)`,
		f.userID, f.max-1); err != nil {
		t.Fatal(err)
	}

	draft := f.request(http.MethodPost, "/api/v1/memes/upload", validDraft)
	if draft.Code != http.StatusCreated {
		t.Fatalf("prepare last slot = %d; want 201", draft.Code)
	}
	var res struct{ Data meme.UploadResponse }
	if err := json.NewDecoder(draft.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	result := f.request(http.MethodPost, "/api/v1/memes/"+res.Data.ID+"/result-upload", "")
	if result.Code != http.StatusOK {
		t.Fatalf("prepare result for last slot = %d; want 200", result.Code)
	}
}

func TestConcurrentReservationsRespectThePlanLimit(t *testing.T) {
	f := setupQuota(t)
	start := make(chan struct{})
	responses := make(chan *httptest.ResponseRecorder, f.max+4)
	for range f.max + 4 {
		go func() {
			<-start
			responses <- f.request(http.MethodPost, "/api/v1/memes/upload", validDraft)
		}()
	}
	close(start)

	accepted := 0
	for range f.max + 4 {
		rec := <-responses
		switch rec.Code {
		case http.StatusCreated:
			accepted++
		case http.StatusForbidden:
			var body response.Body
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body.Code != response.CodeMemeQuotaExceeded {
				t.Errorf("unexpected limit response: %+v, %v", body, err)
			}
		default:
			t.Errorf("unexpected reservation status: %d", rec.Code)
		}
	}
	var persisted int
	err := f.pool.QueryRow(t.Context(), "SELECT COUNT(*) FROM memes WHERE user_id = $1", f.userID).Scan(&persisted)
	if err != nil || accepted != f.max || persisted != f.max {
		t.Fatalf("reservations accepted = %d, persisted = %d, error = %v; want %d", accepted, persisted, err, f.max)
	}
}

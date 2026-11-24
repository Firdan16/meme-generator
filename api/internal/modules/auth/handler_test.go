package auth

import (
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/platform/cookie"
)

func TestClearAuthCookiesRemovesSessionAtRoot(t *testing.T) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := url.Parse("http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	issue := httptest.NewRecorder()
	setAuthCookies(issue, AuthResponse{
		Token: TokenPayload{AccessToken: "fixture-access", RefreshToken: "fixture-refresh", ExpiresIn: 60},
	})
	jar.SetCookies(origin, issue.Result().Cookies())
	if len(jar.Cookies(origin)) != 2 {
		t.Fatal("fixture did not set both session cookies at the root")
	}
	logout := httptest.NewRecorder()
	clearAuthCookies(logout)
	jar.SetCookies(origin, logout.Result().Cookies())
	if len(jar.Cookies(origin)) != 0 {
		t.Fatal("logout left session cookies at the root path")
	}
	for _, c := range issue.Result().Cookies() {
		if (c.Name == cookie.AccessTokenName || c.Name == cookie.RefreshTokenName) && !c.HttpOnly {
			t.Errorf("%s must be HttpOnly", c.Name)
		}
	}
}

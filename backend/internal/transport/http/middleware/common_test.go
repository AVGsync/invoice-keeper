package middleware

import (
	"net/http"
	"testing"
)

func TestExtractTokenBearerWinsOverCookie(t *testing.T) {
	const cookieName = "AccessToken"

	newReq := func(bearer, cookie string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		if bearer != "" {
			r.Header.Set("Authorization", bearer)
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
		}
		return r
	}

	cases := []struct {
		name    string
		bearer  string
		cookie  string
		wantTok string
		wantOK  bool
	}{
		{"bearer wins over cookie", "Bearer header-token", "cookie-token", "header-token", true},
		{"cookie only", "", "cookie-token", "cookie-token", true},
		{"bearer only", "Bearer header-token", "", "header-token", true},
		{"empty bearer falls back to cookie", "Bearer   ", "cookie-token", "cookie-token", true},
		{"no bearer prefix falls back to cookie", "Token xyz", "cookie-token", "cookie-token", true},
		{"case-insensitive bearer wins", "bearer header-token", "cookie-token", "header-token", true},
		{"nothing", "", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tok, ok := extractToken(newReq(c.bearer, c.cookie), cookieName)
			if ok != c.wantOK || tok != c.wantTok {
				t.Errorf("extractToken = (%q, %v), want (%q, %v)", tok, ok, c.wantTok, c.wantOK)
			}
		})
	}
}

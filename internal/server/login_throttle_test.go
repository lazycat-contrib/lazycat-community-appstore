package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAdminLoginFailuresRequireCaptchaBlockAndExpire(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	srv := &Server{
		adminLoginFailures: make(map[string]adminLoginFailure),
		authNow:            func() time.Time { return now },
	}
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	req.RemoteAddr = "192.0.2.10:4321"

	for attempt := 1; attempt <= adminLoginBlockThreshold; attempt++ {
		state := srv.recordAdminLoginFailure(req, "admin")
		if state.Attempts != attempt {
			t.Fatalf("attempts = %d, want %d", state.Attempts, attempt)
		}
		if attempt == adminCaptchaFailedAttempts && state.Attempts < adminCaptchaFailedAttempts {
			t.Fatal("third failure did not require captcha")
		}
	}
	state, ok := srv.adminLoginFailureForRequest(req, "admin")
	if !ok || !state.BlockedUntil.Equal(now.Add(adminLoginBlockDuration)) {
		t.Fatalf("blocked state = %+v, ok=%v", state, ok)
	}
	recorder := httptest.NewRecorder()
	writeAdminLoginRateLimit(recorder, state.BlockedUntil.Sub(now), state)
	if recorder.Code != 429 || recorder.Header().Get("Retry-After") != "30" {
		t.Fatalf("rate limit response code=%d retry-after=%q", recorder.Code, recorder.Header().Get("Retry-After"))
	}

	srv.clearAdminLoginFailures(req, "admin")
	if _, ok := srv.adminLoginFailureForRequest(req, "admin"); ok {
		t.Fatal("successful login did not clear failure state")
	}
	srv.recordAdminLoginFailure(req, "admin")
	now = now.Add(adminLoginFailureTTL + time.Second)
	if _, ok := srv.adminLoginFailureForRequest(req, "admin"); ok {
		t.Fatal("expired failure state was retained")
	}
}

func TestAdminLoginFailureConcurrentCountIsExact(t *testing.T) {
	srv := &Server{adminLoginFailures: make(map[string]adminLoginFailure)}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "192.0.2.10:4321"
	const callers = 128
	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			srv.recordAdminLoginFailure(req, "admin")
		})
	}
	wg.Wait()
	state, ok := srv.adminLoginFailureForRequest(req, "admin")
	if !ok || state.Attempts != callers {
		t.Fatalf("failure state = %+v, ok=%v, want %d attempts", state, ok, callers)
	}
}

func TestAdminLoginHandlerBlocksSixthFailureAndClearsAfterSuccess(t *testing.T) {
	app := newTestApp(t)
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	app.server.authNow = func() time.Time { return now }
	for attempt := 1; attempt <= adminLoginBlockThreshold; attempt++ {
		rec := app.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "wrong-password"})
		if attempt < adminLoginBlockThreshold && rec.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d, body = %s", attempt, rec.Code, rec.Body.String())
		}
		if attempt == adminLoginBlockThreshold {
			if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "30" {
				t.Fatalf("sixth failure status=%d retry=%q body=%s", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
			}
		}
	}
	blocked := app.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "changeme"})
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("correct password during block status = %d", blocked.Code)
	}
	now = now.Add(adminLoginBlockDuration + time.Second)
	success := app.do(http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "admin", "password": "changeme"})
	if success.Code != http.StatusOK {
		t.Fatalf("login after block status = %d, body = %s", success.Code, success.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	if _, ok := app.server.adminLoginFailureForRequest(req, "admin"); ok {
		t.Fatal("successful login did not clear handler failure state")
	}
}

func TestAdminLoginFailureKeysAreBounded(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	srv := &Server{
		adminLoginFailures: make(map[string]adminLoginFailure),
		authNow:            func() time.Time { return now },
	}
	for i := range 5000 {
		req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
		req.RemoteAddr = fmt.Sprintf("192.0.2.%d:4321", i)
		srv.recordAdminLoginFailure(req, fmt.Sprintf("admin-%d", i))
	}
	if got := len(srv.adminLoginFailures); got != maxAdminLoginFailureKeys {
		t.Fatalf("failure key count = %d, want %d", got, maxAdminLoginFailureKeys)
	}
}

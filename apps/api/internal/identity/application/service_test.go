package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/domain"
)

type fakeVerifier string

func (f fakeVerifier) Verify(p string) bool { return p == string(f) }

type fakeCodec struct{}

func (fakeCodec) Encode(s domain.Session) string { return s.ExpiresAt.Format(time.RFC3339) }

func (fakeCodec) Decode(tok string) (domain.Session, error) {
	exp, err := time.Parse(time.RFC3339, tok)
	if err != nil {
		return domain.Session{}, domain.ErrInvalidSession
	}
	return domain.Session{IssuedAt: exp.Add(-time.Hour), ExpiresAt: exp}, nil
}

type fakeThrottle struct{ allow bool }

func (f fakeThrottle) Allow(string) bool { return f.allow }

func newService(now time.Time, allow bool) *application.Service {
	return application.NewService(fakeVerifier("hunter2"), fakeCodec{}, fakeThrottle{allow}, time.Hour,
		func() time.Time { return now })
}

func TestLogin(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	token, s, err := newService(now, true).Login(t.Context(), "ip", "hunter2")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" || !s.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("token=%q session=%+v", token, s)
	}

	if _, _, err := newService(now, true).Login(t.Context(), "ip", "wrong"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("wrong password err = %v", err)
	}
	if _, _, err := newService(now, false).Login(t.Context(), "ip", "hunter2"); !errors.Is(err, domain.ErrTooManyAttempts) {
		t.Fatalf("throttled err = %v", err)
	}
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	svc := newService(now, true)
	token, _, _ := svc.Login(t.Context(), "ip", "hunter2")

	if _, err := svc.Authenticate(t.Context(), token); err != nil {
		t.Fatalf("valid token: %v", err)
	}
	later := newService(now.Add(2*time.Hour), true)
	if _, err := later.Authenticate(t.Context(), token); !errors.Is(err, domain.ErrInvalidSession) {
		t.Fatalf("expired token err = %v", err)
	}
	for _, bad := range []string{"", "garbage"} {
		if _, err := svc.Authenticate(t.Context(), bad); !errors.Is(err, domain.ErrInvalidSession) {
			t.Fatalf("token %q err = %v", bad, err)
		}
	}
}

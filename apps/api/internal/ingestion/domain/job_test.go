package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

var t0 = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func TestUploadLifecycle(t *testing.T) {
	t.Parallel()

	origin := "switch"
	j, err := domain.NewUploadJob("abc", `C:\Users\me\Downloads\INSIDE [0100D2D009028000][v0].nsp`, 100, &origin, t0)
	if err != nil {
		t.Fatal(err)
	}
	if j.FileName != "INSIDE [0100D2D009028000][v0].nsp" || j.Status != domain.StatusUploading {
		t.Fatalf("new job = %+v", j)
	}

	j.RecordProgress(40, t0.Add(time.Second))
	j.RecordProgress(30, t0.Add(2*time.Second)) // out-of-order report is ignored
	if j.Received != 40 {
		t.Fatalf("received = %d, want 40", j.Received)
	}

	if err := j.MarkUploaded("/staging/abc", t0.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if j.Received != 100 || j.StoragePath != "/staging/abc" || !j.UpdatedAt.Equal(t0.Add(3*time.Second)) {
		t.Fatalf("uploaded job = %+v", j)
	}
	j.RecordProgress(10, t0.Add(4*time.Second))
	if j.Received != 100 {
		t.Fatal("progress after upload must not change the job")
	}

	if err := j.Cancel(t0); err != nil {
		t.Fatal(err)
	}
	if err := j.Fail("boom", t0); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("fail after cancel err = %v", err)
	}
}

func TestStateMachine(t *testing.T) {
	t.Parallel()

	allowed := [][2]domain.Status{
		{domain.StatusUploading, domain.StatusUploaded},
		{domain.StatusUploaded, domain.StatusExtracting},
		{domain.StatusUploaded, domain.StatusReview}, // a raw (non-archive) file skips extraction
		{domain.StatusExtracting, domain.StatusNeedsPassword},
		{domain.StatusNeedsPassword, domain.StatusExtracting},
		{domain.StatusExtracting, domain.StatusReview},
		{domain.StatusReview, domain.StatusCommitting},
		{domain.StatusCommitting, domain.StatusDone},
		{domain.StatusCommitting, domain.StatusReview}, // commit rolled back
	}
	for _, tr := range allowed {
		if !tr[0].CanTransitionTo(tr[1]) {
			t.Errorf("%s → %s should be allowed", tr[0], tr[1])
		}
	}

	forbidden := [][2]domain.Status{
		{domain.StatusUploading, domain.StatusReview},
		{domain.StatusDone, domain.StatusFailed},
		{domain.StatusCancelled, domain.StatusUploading},
		{domain.StatusCommitting, domain.StatusCancelled}, // too late to cancel mid-commit
	}
	for _, tr := range forbidden {
		if tr[0].CanTransitionTo(tr[1]) {
			t.Errorf("%s → %s should be forbidden", tr[0], tr[1])
		}
	}

	for _, s := range []domain.Status{domain.StatusDone, domain.StatusFailed, domain.StatusCancelled} {
		if !s.Terminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	if domain.StatusReview.Terminal() {
		t.Error("review is not terminal")
	}
}

func TestCleanFileName(t *testing.T) {
	t.Parallel()

	ok := map[string]string{
		"game.rar":       "game.rar",
		"/etc/passwd":    "passwd",
		`..\..\evil.nsp`: "evil.nsp",
		"  Animal Crossing™꞉ New Horizons.nsp  ":  "Animal Crossing™꞉ New Horizons.nsp",
		"Animal Crossing_ New Horizons [EUR].rar": "Animal Crossing_ New Horizons [EUR].rar",
	}
	for in, want := range ok {
		got, err := domain.CleanFileName(in)
		if err != nil || got != want {
			t.Errorf("CleanFileName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "  ", "..", "/", "a\x00b.rar", "bad\nname.rar"} {
		if _, err := domain.CleanFileName(bad); !errors.Is(err, domain.ErrInvalidFileName) {
			t.Errorf("CleanFileName(%q) err = %v", bad, err)
		}
	}
}

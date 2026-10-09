package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

var t0 = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

var spec = domain.Spec{Console: "switch", Title: "Limbo"}

func TestUploadLifecycle(t *testing.T) {
	t.Parallel()

	j, err := domain.NewUploadJob("abc", `C:\Users\me\Downloads\Limbo [0100A8E005E7C000].nsp`, 100, domain.Spec{Console: "switch", Title: "  Limbo  "}, t0)
	if err != nil {
		t.Fatal(err)
	}
	if j.FileName != "Limbo [0100A8E005E7C000].nsp" || j.Status != domain.StatusUploading || j.Title != "Limbo" {
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
	if err := j.Cancel(t0); err != nil {
		t.Fatal(err)
	}
	if err := j.Fail("boom", t0); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("fail after cancel err = %v", err)
	}
}

func TestNewUploadJobNeedsItsForm(t *testing.T) {
	t.Parallel()

	bad := []domain.Spec{
		{Title: "Limbo"},
		{Console: "switch"},
		{Console: "switch", Title: "   "},
		{Console: "switch", Title: "Limbo", GroupID: "g"},
		{Console: "switch", Title: "Limbo", GroupSize: 2},
	}
	for _, s := range bad {
		if _, err := domain.NewUploadJob("x", "a.nsp", 1, s, t0); !errors.Is(err, domain.ErrInvalidSpec) {
			t.Errorf("spec %+v: err = %v", s, err)
		}
	}
}

func TestStateMachine(t *testing.T) {
	t.Parallel()

	allowed := [][2]domain.Status{
		{domain.StatusUploading, domain.StatusUploaded},
		{domain.StatusUploaded, domain.StatusExtracting},
		{domain.StatusUploaded, domain.StatusConfirm}, // a raw file skips extraction
		{domain.StatusUploaded, domain.StatusInvalid},
		{domain.StatusExtracting, domain.StatusNeedsPassword},
		{domain.StatusNeedsPassword, domain.StatusExtracting},
		{domain.StatusExtracting, domain.StatusConfirm},
		{domain.StatusInvalid, domain.StatusConfirm}, // another console
		{domain.StatusInvalid, domain.StatusUnassigned},
		{domain.StatusInvalid, domain.StatusTrashed},
		{domain.StatusConfirm, domain.StatusCommitting},
		{domain.StatusCommitting, domain.StatusDone},
		{domain.StatusCommitting, domain.StatusConfirm}, // commit rolled back
	}
	for _, tr := range allowed {
		if !tr[0].CanTransitionTo(tr[1]) {
			t.Errorf("%s → %s should be allowed", tr[0], tr[1])
		}
	}
	forbidden := [][2]domain.Status{
		{domain.StatusUploading, domain.StatusConfirm},
		{domain.StatusConfirm, domain.StatusUnassigned}, // only invalid uploads are set aside
		{domain.StatusDone, domain.StatusFailed},
		{domain.StatusCommitting, domain.StatusCancelled}, // too late to cancel mid-commit
	}
	for _, tr := range forbidden {
		if tr[0].CanTransitionTo(tr[1]) {
			t.Errorf("%s → %s should be forbidden", tr[0], tr[1])
		}
	}
	for _, s := range []domain.Status{domain.StatusDone, domain.StatusFailed, domain.StatusCancelled, domain.StatusUnassigned, domain.StatusTrashed} {
		if !s.Terminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	if domain.StatusInvalid.Terminal() || domain.StatusConfirm.Terminal() {
		t.Error("confirm and invalid are not terminal")
	}
}

func TestCleanFileName(t *testing.T) {
	t.Parallel()

	ok := map[string]string{
		"game.rar":       "game.rar",
		"/etc/passwd":    "passwd",
		`..\..\evil.nsp`: "evil.nsp",
		"  Animal Crossing™꞉ New Horizons.nsp  ": "Animal Crossing™꞉ New Horizons.nsp",
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

func TestExtractionAndValidation(t *testing.T) {
	t.Parallel()

	j, _ := domain.NewUploadJob("x", "game.rar", 10, spec, t0)
	_ = j.MarkUploaded("/up/x", t0)
	if err := j.StartExtraction(t0); err != nil {
		t.Fatal(err)
	}
	j.RecordExtractionProgress(40, t0)
	j.RecordExtractionProgress(20, t0) // never goes back
	if j.Progress != 40 {
		t.Fatalf("progress = %d", j.Progress)
	}
	if err := j.RequirePassword("Contraseña incorrecta.", t0); err != nil || j.Progress != 0 || j.Error == "" {
		t.Fatalf("needs password: %v %+v", err, j)
	}
	if err := j.StartExtraction(t0); err != nil || j.Error != "" {
		t.Fatalf("retry clears the error: %v %+v", err, j)
	}
	if err := j.Validated("switch", domain.InvalidNone, t0); err != nil || j.Status != domain.StatusInvalid || j.InvalidReason != domain.InvalidNone {
		t.Fatalf("invalid: %v %+v", err, j)
	}
	if err := j.Validated("wii", "", t0); err != nil || j.Status != domain.StatusConfirm || j.Console != "wii" || j.InvalidReason != "" {
		t.Fatalf("another console: %v %+v", err, j)
	}
}

func TestCommitAndSetAside(t *testing.T) {
	t.Parallel()

	j := &domain.UploadJob{ID: "abc", Status: domain.StatusConfirm, Error: "old"}
	if err := j.StartCommit(t0); err != nil || j.Status != domain.StatusCommitting || j.Error != "" {
		t.Fatalf("start = %+v, %v", j, err)
	}
	if err := j.AbortCommit("disk full", t0); err != nil || j.Status != domain.StatusConfirm || j.Error != "disk full" {
		t.Fatalf("abort = %+v, %v", j, err)
	}
	if err := j.SetAside(true, t0); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("set aside a confirmable job: %v", err)
	}
	j.Status = domain.StatusInvalid
	if err := j.SetAside(true, t0); err != nil || j.Status != domain.StatusTrashed {
		t.Fatalf("trash = %+v, %v", j, err)
	}
}

func TestDetectArchive(t *testing.T) {
	t.Parallel()

	tests := map[string]domain.ArchiveFormat{
		"PK\x03\x04rest":             domain.ArchiveZip,
		"7z\xBC\xAF\x27\x1C\x00\x04": domain.Archive7z,
		"Rar!\x1A\x07\x00x":          domain.ArchiveRar, // RAR 4
		"Rar!\x1A\x07\x01\x00":       domain.ArchiveRar, // RAR 5 (real Switch dumps)
	}
	for header, want := range tests {
		if got, ok := domain.DetectArchive([]byte(header)); !ok || got != want {
			t.Errorf("%q = %q %v, want %q", header, got, ok, want)
		}
	}
	for _, notArchive := range []string{"PFS0", "", "Rar!", "CD001"} {
		if _, ok := domain.DetectArchive([]byte(notArchive)); ok {
			t.Errorf("%q detected as archive", notArchive)
		}
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	known := []string{".nsp", ".xci", ".iso", ".wbfs", ".rvz", ".nkit.iso", ".cso"}
	sw := domain.ConsoleRule{Slug: "switch", Extensions: []string{".nsp", ".xci"}, MultipleFiles: true}
	wii := domain.ConsoleRule{Slug: "wii", Extensions: []string{".iso", ".wbfs", ".rvz", ".nkit.iso"}}
	psp := domain.ConsoleRule{Slug: "psp", Extensions: []string{".iso", ".cso"}}
	files := func(paths ...string) []domain.StagedFile {
		out := make([]domain.StagedFile, len(paths))
		for i, p := range paths {
			out[i] = domain.StagedFile{Path: p}
		}
		return out
	}

	tests := []struct {
		name   string
		rule   domain.ConsoleRule
		files  []domain.StagedFile
		valid  int
		reason domain.InvalidReason
	}{
		{"switch with junk", sw, files("a/Base.NSP", "a/upd.xci", "readme.txt"), 2, ""},
		{"switch without game files", sw, files("game.iso"), 0, domain.InvalidNone},
		{"one wii image", wii, files("Okami (USA).nkit.iso", "info.nfo"), 1, ""},
		{"two wii images", wii, files("a.iso", "b.wbfs"), 2, domain.InvalidMany},
		{"nkit is not an iso for psp", psp, files("Okami.nkit.iso"), 0, domain.InvalidNone},
		{"no files", psp, nil, 0, domain.InvalidNone},
	}
	for _, tt := range tests {
		valid, reason := domain.Validate(tt.rule, tt.files, known)
		if len(valid) != tt.valid || reason != tt.reason {
			t.Errorf("%s: valid %d %q, want %d %q", tt.name, len(valid), reason, tt.valid, tt.reason)
		}
	}
}

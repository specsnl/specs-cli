package template

import (
	"path/filepath"
	"testing"
	"time"
)

func TestIsStale_Fresh(t *testing.T) {
	s := &TemplateStatus{
		CheckedAt: JSONTime{Time: time.Now().Add(-1 * time.Hour)},
	}
	if s.IsStale() {
		t.Error("expected IsStale() = false for status checked 1 hour ago")
	}
}

func TestIsStale_Old(t *testing.T) {
	s := &TemplateStatus{
		CheckedAt: JSONTime{Time: time.Now().Add(-25 * time.Hour)},
	}
	if !s.IsStale() {
		t.Error("expected IsStale() = true for status checked 25 hours ago")
	}
}

func TestLoadStatus_Missing(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadStatus(dir)
	if err != nil {
		t.Fatalf("LoadStatus: expected nil error, got %v", err)
	}
	if s != nil {
		t.Errorf("LoadStatus: expected nil status for missing file, got %+v", s)
	}
}

func TestStatusRoundtrip(t *testing.T) {
	dir := t.TempDir()

	original := &TemplateStatus{
		CheckedAt:     JSONTime{Time: time.Now().UTC().Truncate(time.Second)},
		IsUpToDate:    true,
		LatestVersion: "v2.0.0",
		ErrorKind:     "",
	}

	if err := SaveStatus(dir, original); err != nil {
		t.Fatalf("SaveStatus: %v", err)
	}

	loaded, err := LoadStatus(dir)
	if err != nil {
		t.Fatalf("LoadStatus: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadStatus: expected non-nil status")
	}
	if loaded.IsUpToDate != original.IsUpToDate {
		t.Errorf("IsUpToDate: got %v, want %v", loaded.IsUpToDate, original.IsUpToDate)
	}
	if loaded.LatestVersion != original.LatestVersion {
		t.Errorf("LatestVersion: got %q, want %q", loaded.LatestVersion, original.LatestVersion)
	}
	if loaded.ErrorKind != original.ErrorKind {
		t.Errorf("ErrorKind: got %q, want %q", loaded.ErrorKind, original.ErrorKind)
	}
	if !loaded.CheckedAt.Time.Equal(original.CheckedAt.Time) {
		t.Errorf("CheckedAt: got %v, want %v", loaded.CheckedAt.Time, original.CheckedAt.Time)
	}

	// Verify it was written to the correct path.
	if _, err := LoadStatus(filepath.Join(dir, "nonexistent")); err != nil {
		t.Errorf("LoadStatus on nonexistent dir: expected nil error, got %v", err)
	}
}

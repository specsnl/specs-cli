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

func TestNeedsRefresh(t *testing.T) {
	fresh := JSONTime{Time: time.Now().Add(-1 * time.Hour)}
	stale := JSONTime{Time: time.Now().Add(-25 * time.Hour)}

	tests := []struct {
		name       string
		checkedAt  JSONTime
		version    string
		current    string
		wantResult bool
	}{
		{"fresh same version", fresh, "1.0.0", "1.0.0", false},
		{"fresh different version", fresh, "0.9.0", "1.0.0", true},
		{"fresh empty stored version", fresh, "", "1.0.0", true},
		{"stale same version", stale, "1.0.0", "1.0.0", true},
		{"stale different version", stale, "0.9.0", "1.0.0", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &TemplateStatus{CheckedAt: tc.checkedAt, SpecsVersion: tc.version}
			if got := s.NeedsRefresh(tc.current); got != tc.wantResult {
				t.Errorf("NeedsRefresh(%q) with stored %q = %v, want %v", tc.current, tc.version, got, tc.wantResult)
			}
		})
	}
}

func TestStatus_SpecsVersionRoundtrip(t *testing.T) {
	dir := t.TempDir()
	original := &TemplateStatus{
		CheckedAt:    JSONTime{Time: time.Now().UTC().Truncate(time.Second)},
		IsUpToDate:   true,
		SpecsVersion: "1.2.3",
	}

	if err := SaveStatus(dir, original); err != nil {
		t.Fatalf("SaveStatus: %v", err)
	}

	loaded, err := LoadStatus(dir)
	if err != nil {
		t.Fatalf("LoadStatus: %v", err)
	}

	if loaded.SpecsVersion != "1.2.3" {
		t.Errorf("SpecsVersion: got %q, want %q", loaded.SpecsVersion, "1.2.3")
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

	if !loaded.CheckedAt.Equal(original.CheckedAt.Time) {
		t.Errorf("CheckedAt: got %v, want %v", loaded.CheckedAt.Time, original.CheckedAt.Time)
	}

	// Verify it was written to the correct path.
	if _, err := LoadStatus(filepath.Join(dir, "nonexistent")); err != nil {
		t.Errorf("LoadStatus on nonexistent dir: expected nil error, got %v", err)
	}
}

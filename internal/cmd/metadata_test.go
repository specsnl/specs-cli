package cmd

import (
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

func TestWriteMetadata_PreservesSuppliedCreated(t *testing.T) {
	dir := t.TempDir()

	// RFC1123Z has second-level precision, so truncate before round-tripping.
	want := time.Now().Add(-30 * 24 * time.Hour).UTC().Truncate(time.Second)

	if err := pkgtemplate.SaveMetadata(dir, "tpl", "https://example.com/repo", "main", "abc123", "v1.0.0", want); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	got, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if got == nil {
		t.Fatal("LoadMetadata returned nil metadata")
	}
	if !got.Created.Time.Equal(want) {
		t.Errorf("Created = %s, want %s", got.Created.Time, want)
	}
}

// Verifies the upgrade-flow contract: passing meta.Created.Time back into
// SaveMetadata leaves the timestamp unchanged after a round-trip.
func TestWriteMetadata_UpgradeRoundTripPreservesCreated(t *testing.T) {
	dir := t.TempDir()

	original := time.Now().Add(-90 * 24 * time.Hour).UTC().Truncate(time.Second)

	// Initial install.
	if err := pkgtemplate.SaveMetadata(dir, "tpl", "https://example.com/repo", "main", "old-sha", "v1.0.0", original); err != nil {
		t.Fatalf("SaveMetadata (install): %v", err)
	}

	meta, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if meta == nil {
		t.Fatal("LoadMetadata returned nil metadata")
	}

	// Simulate an upgrade: re-write metadata with new commit/version but the
	// original Created threaded through, the same way registry.Upgrade does.
	if err := pkgtemplate.SaveMetadata(dir, "tpl", "https://example.com/repo", "main", "new-sha", "v1.1.0", meta.Created.Time); err != nil {
		t.Fatalf("SaveMetadata (upgrade): %v", err)
	}

	upgraded, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata after upgrade: %v", err)
	}
	if upgraded == nil {
		t.Fatal("LoadMetadata returned nil metadata after upgrade")
	}
	if !upgraded.Created.Time.Equal(original) {
		t.Errorf("Created after upgrade = %s, want %s", upgraded.Created.Time, original)
	}
	if upgraded.Commit != "new-sha" {
		t.Errorf("Commit after upgrade = %q, want %q", upgraded.Commit, "new-sha")
	}
	if upgraded.Version != "v1.1.0" {
		t.Errorf("Version after upgrade = %q, want %q", upgraded.Version, "v1.1.0")
	}
}

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

	if err := pkgtemplate.SaveMetadata(dir, "tpl", "https://example.com/repo", "main", "abc123", "v1.0.0", want, want); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	got, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	if got == nil {
		t.Fatal("LoadMetadata returned nil metadata")
	}

	if !got.Created.Equal(want) {
		t.Errorf("Created = %s, want %s", got.Created.Time, want)
	}

	if !got.Updated.Equal(want) {
		t.Errorf("Updated = %s, want %s", got.Updated.Time, want)
	}
}

// Verifies the upgrade-flow contract: passing meta.Created.Time back into
// SaveMetadata leaves the timestamp unchanged after a round-trip.
func TestWriteMetadata_UpgradeRoundTripPreservesCreated(t *testing.T) {
	dir := t.TempDir()

	original := time.Now().Add(-90 * 24 * time.Hour).UTC().Truncate(time.Second)

	// Initial install: created and updated are the same.
	if err := pkgtemplate.SaveMetadata(dir, "tpl", "https://example.com/repo", "main", "old-sha", "v1.0.0", original, original); err != nil {
		t.Fatalf("SaveMetadata (install): %v", err)
	}

	meta, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}

	if meta == nil {
		t.Fatal("LoadMetadata returned nil metadata")
	}

	// Simulate an upgrade: re-write metadata with new commit/version and a fresh
	// Updated timestamp but the original Created threaded through, the same way
	// registry.Upgrade does.
	upgradedAt := time.Now().Add(-1 * 24 * time.Hour).UTC().Truncate(time.Second)
	if err := pkgtemplate.SaveMetadata(dir, "tpl", "https://example.com/repo", "main", "new-sha", "v1.1.0", meta.Created.Time, upgradedAt); err != nil {
		t.Fatalf("SaveMetadata (upgrade): %v", err)
	}

	upgraded, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata after upgrade: %v", err)
	}

	if upgraded == nil {
		t.Fatal("LoadMetadata returned nil metadata after upgrade")
	}

	if !upgraded.Created.Equal(original) {
		t.Errorf("Created after upgrade = %s, want %s", upgraded.Created.Time, original)
	}

	if !upgraded.Updated.Equal(upgradedAt) {
		t.Errorf("Updated after upgrade = %s, want %s", upgraded.Updated.Time, upgradedAt)
	}

	if upgraded.Commit != "new-sha" {
		t.Errorf("Commit after upgrade = %q, want %q", upgraded.Commit, "new-sha")
	}

	if upgraded.Version != "v1.1.0" {
		t.Errorf("Version after upgrade = %q, want %q", upgraded.Version, "v1.1.0")
	}
}

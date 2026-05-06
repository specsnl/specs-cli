package cmd

import (
	"testing"
	"time"
)

func TestWriteMetadata_PreservesSuppliedCreated(t *testing.T) {
	dir := t.TempDir()

	// RFC1123Z has second-level precision, so truncate before round-tripping.
	want := time.Now().Add(-30 * 24 * time.Hour).UTC().Truncate(time.Second)

	if err := writeMetadata(dir, "tpl", "https://example.com/repo", "main", "abc123", "v1.0.0", want); err != nil {
		t.Fatalf("writeMetadata: %v", err)
	}

	got, err := loadMetadataForListing(dir)
	if err != nil {
		t.Fatalf("loadMetadataForListing: %v", err)
	}
	if got == nil {
		t.Fatal("loadMetadataForListing returned nil metadata")
	}
	if !got.Created.Time.Equal(want) {
		t.Errorf("Created = %s, want %s", got.Created.Time, want)
	}
}

// Verifies the upgrade-flow contract: passing meta.Created.Time back into
// writeMetadata leaves the timestamp unchanged after a round-trip.
func TestWriteMetadata_UpgradeRoundTripPreservesCreated(t *testing.T) {
	dir := t.TempDir()

	original := time.Now().Add(-90 * 24 * time.Hour).UTC().Truncate(time.Second)

	// Initial install.
	if err := writeMetadata(dir, "tpl", "https://example.com/repo", "main", "old-sha", "v1.0.0", original); err != nil {
		t.Fatalf("writeMetadata (install): %v", err)
	}

	meta, err := loadMetadataForListing(dir)
	if err != nil {
		t.Fatalf("loadMetadataForListing: %v", err)
	}
	if meta == nil {
		t.Fatal("loadMetadataForListing returned nil metadata")
	}

	// Simulate an upgrade: re-write metadata with new commit/version but the
	// original Created threaded through, the same way template_upgrade.go does.
	if err := writeMetadata(dir, "tpl", "https://example.com/repo", "main", "new-sha", "v1.1.0", meta.Created.Time); err != nil {
		t.Fatalf("writeMetadata (upgrade): %v", err)
	}

	upgraded, err := loadMetadataForListing(dir)
	if err != nil {
		t.Fatalf("loadMetadataForListing after upgrade: %v", err)
	}
	if upgraded == nil {
		t.Fatal("loadMetadataForListing returned nil metadata after upgrade")
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


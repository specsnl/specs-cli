package template_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	"github.com/specsnl/specs-cli/internal/specs"
)

// --- JSONTime.MarshalJSON / UnmarshalJSON ---

func TestJSONTime_RoundTrip(t *testing.T) {
	original := pkgtemplate.JSONTime{Time: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var restored pkgtemplate.JSONTime
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	if !original.Time.Equal(restored.Time) {
		t.Errorf("round-trip mismatch: got %v, want %v", restored.Time, original.Time)
	}
}

func TestJSONTime_MarshalJSON_UsesRFC1123Z(t *testing.T) {
	jt := pkgtemplate.JSONTime{Time: time.Date(2024, 1, 2, 15, 4, 5, 0, time.UTC)}
	data, err := json.Marshal(jt)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	// Marshalled value is a JSON string containing the RFC1123Z representation.
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("unexpected format: %v", err)
	}
	want := jt.Time.Format(time.RFC1123Z)
	if s != want {
		t.Errorf("MarshalJSON = %q, want %q", s, want)
	}
}

func TestJSONTime_UnmarshalJSON_InvalidString(t *testing.T) {
	var jt pkgtemplate.JSONTime
	if err := json.Unmarshal([]byte(`"not a date"`), &jt); err == nil {
		t.Error("expected error for invalid date string, got nil")
	}
}

func TestJSONTime_UnmarshalJSON_InvalidJSON(t *testing.T) {
	var jt pkgtemplate.JSONTime
	if err := json.Unmarshal([]byte(`123`), &jt); err == nil {
		t.Error("expected error for non-string JSON, got nil")
	}
}

// --- JSONTime.String ---

func TestJSONTime_String_JustNow(t *testing.T) {
	jt := pkgtemplate.JSONTime{Time: time.Now().Add(-10 * time.Second)}
	if got := jt.String(); got != "just now" {
		t.Errorf("String() = %q, want %q", got, "just now")
	}
}

func TestJSONTime_String_MinutesAgo(t *testing.T) {
	jt := pkgtemplate.JSONTime{Time: time.Now().Add(-30 * time.Minute)}
	got := jt.String()
	if !strings.HasSuffix(got, "minutes ago") {
		t.Errorf("String() = %q, want suffix 'minutes ago'", got)
	}
}

func TestJSONTime_String_HoursAgo(t *testing.T) {
	jt := pkgtemplate.JSONTime{Time: time.Now().Add(-3 * time.Hour)}
	got := jt.String()
	if !strings.HasSuffix(got, "hours ago") {
		t.Errorf("String() = %q, want suffix 'hours ago'", got)
	}
}

func TestJSONTime_String_DaysAgo(t *testing.T) {
	jt := pkgtemplate.JSONTime{Time: time.Now().Add(-48 * time.Hour)}
	got := jt.String()
	if !strings.HasSuffix(got, "days ago") {
		t.Errorf("String() = %q, want suffix 'days ago'", got)
	}
}

// --- Metadata loaded via Get ---

func TestGet_LoadsMetadata(t *testing.T) {
	root := buildTemplate(t, "Name: test\n", nil)

	created := time.Date(2024, 3, 10, 9, 0, 0, 0, time.UTC)
	meta := pkgtemplate.Metadata{
		Name:       "my-tpl",
		Repository: "user/repo",
		Created:    pkgtemplate.JSONTime{Time: created},
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "__metadata.json"), data, 0644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tmpl.Metadata == nil {
		t.Fatal("Metadata is nil, expected it to be loaded")
	}
	if tmpl.Metadata.Name != "my-tpl" {
		t.Errorf("Name = %q, want %q", tmpl.Metadata.Name, "my-tpl")
	}
	if tmpl.Metadata.Repository != "user/repo" {
		t.Errorf("Repository = %q, want %q", tmpl.Metadata.Repository, "user/repo")
	}
	if !tmpl.Metadata.Created.Time.Equal(created) {
		t.Errorf("Created = %v, want %v", tmpl.Metadata.Created.Time, created)
	}
}

func TestGet_MissingMetadata_ReturnsNil(t *testing.T) {
	root := buildTemplate(t, "Name: test\n", nil)

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tmpl.Metadata != nil {
		t.Errorf("expected nil Metadata when __metadata.json is absent, got %+v", tmpl.Metadata)
	}
}

func TestGet_MalformedMetadata_ReturnsError(t *testing.T) {
	root := buildTemplate(t, "Name: test\n", nil)

	if err := os.WriteFile(filepath.Join(root, "__metadata.json"), []byte(`{invalid`), 0644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	// LoadMetadata errors are silently ignored by Get (missing/malformed metadata is non-fatal).
	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: unexpected error for malformed metadata: %v", err)
	}
	if tmpl.Metadata != nil {
		t.Errorf("expected nil Metadata for malformed file, got %+v", tmpl.Metadata)
	}
}

// --- LoadMetadata ---

func TestLoadMetadata_Missing(t *testing.T) {
	dir := t.TempDir()

	m, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: unexpected error for missing file: %v", err)
	}
	if m != nil {
		t.Errorf("expected nil for missing __metadata.json, got %+v", m)
	}
}

func TestLoadMetadata_Malformed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, specs.MetadataFile), []byte(`{bad json`), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := pkgtemplate.LoadMetadata(dir)
	if err == nil {
		t.Fatal("LoadMetadata: expected error for malformed file, got nil")
	}
	if m != nil {
		t.Errorf("expected nil for malformed __metadata.json, got %+v", m)
	}
}

// --- SaveMetadata / LoadMetadata round-trip ---

func TestSaveMetadata_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	created := time.Now().Add(-7 * 24 * time.Hour).UTC().Truncate(time.Second)

	updated := time.Now().Add(-2 * 24 * time.Hour).UTC().Truncate(time.Second)

	if err := pkgtemplate.SaveMetadata(dir, "my-tpl", "https://example.com/repo", "main", "abc123", "v1.2.3", created, updated); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	m, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if m == nil {
		t.Fatal("LoadMetadata returned nil after SaveMetadata")
	}
	if m.Name != "my-tpl" {
		t.Errorf("Name = %q, want %q", m.Name, "my-tpl")
	}
	if m.Repository != "https://example.com/repo" {
		t.Errorf("Repository = %q, want %q", m.Repository, "https://example.com/repo")
	}
	if m.Branch != "main" {
		t.Errorf("Branch = %q, want %q", m.Branch, "main")
	}
	if m.Commit != "abc123" {
		t.Errorf("Commit = %q, want %q", m.Commit, "abc123")
	}
	if m.Version != "v1.2.3" {
		t.Errorf("Version = %q, want %q", m.Version, "v1.2.3")
	}
	if !m.Created.Time.Equal(created) {
		t.Errorf("Created = %v, want %v", m.Created.Time, created)
	}
	if !m.Updated.Time.Equal(updated) {
		t.Errorf("Updated = %v, want %v", m.Updated.Time, updated)
	}
}

func TestSaveMetadata_PreservesCreatedOnUpgrade(t *testing.T) {
	dir := t.TempDir()
	original := time.Now().Add(-90 * 24 * time.Hour).UTC().Truncate(time.Second)

	if err := pkgtemplate.SaveMetadata(dir, "tpl", "https://example.com/repo", "main", "old-sha", "v1.0.0", original, original); err != nil {
		t.Fatalf("SaveMetadata (install): %v", err)
	}

	m, _ := pkgtemplate.LoadMetadata(dir)
	if m == nil {
		t.Fatal("LoadMetadata returned nil")
	}

	// Simulate upgrade: re-write with new sha/version, original created preserved
	// but a fresh updated timestamp.
	upgradedAt := time.Now().UTC().Truncate(time.Second)
	if err := pkgtemplate.SaveMetadata(dir, "tpl", "https://example.com/repo", "main", "new-sha", "v1.1.0", m.Created.Time, upgradedAt); err != nil {
		t.Fatalf("SaveMetadata (upgrade): %v", err)
	}

	upgraded, _ := pkgtemplate.LoadMetadata(dir)
	if upgraded == nil {
		t.Fatal("LoadMetadata returned nil after upgrade")
	}
	if !upgraded.Created.Time.Equal(original) {
		t.Errorf("Created after upgrade = %v, want %v", upgraded.Created.Time, original)
	}
	if !upgraded.Updated.Time.Equal(upgradedAt) {
		t.Errorf("Updated after upgrade = %v, want %v", upgraded.Updated.Time, upgradedAt)
	}
	if upgraded.Commit != "new-sha" {
		t.Errorf("Commit = %q, want %q", upgraded.Commit, "new-sha")
	}
}

// Metadata written before the Updated field existed has no Updated key; LoadMetadata
// must fall back to Created so such templates still display a sensible timestamp.
func TestLoadMetadata_MissingUpdated_FallsBackToCreated(t *testing.T) {
	dir := t.TempDir()
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	// Hand-write legacy metadata without an Updated field.
	legacy := `{
  "Name": "legacy-tpl",
  "Repository": "https://example.com/repo",
  "Branch": "main",
  "Created": "` + created.Format(time.RFC1123Z) + `",
  "Commit": "abc123",
  "Version": "v1.0.0"
}`
	if err := os.WriteFile(filepath.Join(dir, specs.MetadataFile), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := pkgtemplate.LoadMetadata(dir)
	if err != nil {
		t.Fatalf("LoadMetadata: %v", err)
	}
	if m == nil {
		t.Fatal("LoadMetadata returned nil")
	}
	if !m.Updated.Time.Equal(created) {
		t.Errorf("Updated = %v, want fallback to Created %v", m.Updated.Time, created)
	}
}

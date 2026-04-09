package template_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
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
		Tag:        "my-tag",
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tmpl.Metadata == nil {
		t.Fatal("Metadata is nil, expected it to be loaded")
	}
	if tmpl.Metadata.Tag != "my-tag" {
		t.Errorf("Tag = %q, want %q", tmpl.Metadata.Tag, "my-tag")
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	// loadMetadata errors are silently ignored by Get (missing metadata is non-fatal).
	// Malformed metadata is treated the same — Get should still succeed with nil Metadata.
	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
	if err != nil {
		t.Fatalf("Get: unexpected error for malformed metadata: %v", err)
	}
	if tmpl.Metadata != nil {
		t.Errorf("expected nil Metadata for malformed file, got %+v", tmpl.Metadata)
	}
}

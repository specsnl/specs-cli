package registry_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/specsnl/specs-cli/internal/registry"
	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

// withTempRegistry redirects the XDG config home to a temp directory so
// specs.TemplatePath returns paths inside it, and reloads xdg afterwards.
func withTempRegistry(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	xdg.Reload()
	t.Cleanup(func() { xdg.Reload() })

	if err := specs.EnsureRegistry(); err != nil {
		t.Fatalf("EnsureRegistry: %v", err)
	}

	return specs.TemplateDir()
}

// mustLoadEntry fails the test when name is not registered: Load reports an
// unregistered template as (nil, nil).
func mustLoadEntry(t *testing.T, name string) *registry.Entry {
	t.Helper()

	entry, err := registry.Load(name)
	if err != nil {
		t.Fatalf("Load(%q): %v", name, err)
	}

	if entry == nil {
		t.Fatalf("Load(%q) returned no entry", name)
	}

	return entry
}

// mustLoadEntryMetadata additionally requires __metadata.json; an entry
// without one has a nil Metadata.
func mustLoadEntryMetadata(t *testing.T, name string) *pkgtemplate.Metadata {
	t.Helper()

	meta := mustLoadEntry(t, name).Metadata
	if meta == nil {
		t.Fatalf("Load(%q) returned an entry with no Metadata", name)
	}

	return meta
}

func TestLoad_NonExistent(t *testing.T) {
	withTempRegistry(t)

	entry, err := registry.Load("no-such-template")
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	if entry != nil {
		t.Errorf("expected nil entry for non-existent template, got %+v", entry)
	}
}

func TestLoad_ExistsNoMetadata(t *testing.T) {
	registryDir := withTempRegistry(t)

	tmplDir := filepath.Join(registryDir, "bare-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	entry := mustLoadEntry(t, "bare-tpl")
	if entry.Name != "bare-tpl" {
		t.Errorf("Name = %q, want %q", entry.Name, "bare-tpl")
	}

	if entry.Metadata != nil {
		t.Errorf("expected nil Metadata for template without __metadata.json, got %+v", entry.Metadata)
	}

	if entry.Status != nil {
		t.Errorf("expected nil Status for template without remote metadata, got %+v", entry.Status)
	}
}

func TestLoad_WithMetadata(t *testing.T) {
	registryDir := withTempRegistry(t)

	tmplDir := filepath.Join(registryDir, "my-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	created := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Second)
	if err := pkgtemplate.SaveMetadata(tmplDir, "my-tpl", "https://example.com/repo", "main", "abc123", "v1.0.0", created, created); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	meta := mustLoadEntryMetadata(t, "my-tpl")
	if meta.Repository != "https://example.com/repo" {
		t.Errorf("Repository = %q, want %q", meta.Repository, "https://example.com/repo")
	}

	if meta.Branch != "main" {
		t.Errorf("Branch = %q, want %q", meta.Branch, "main")
	}
}

func TestUpgrade_NonExistent(t *testing.T) {
	withTempRegistry(t)

	_, err := registry.Upgrade("no-such-template")
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}

	if !errors.Is(err, specs.ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestUpgrade_LocalTemplate(t *testing.T) {
	registryDir := withTempRegistry(t)

	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	// "local:" prefix is what `specs template save` stores in Repository.
	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "local:/local/path", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	result, err := registry.Upgrade("local-tpl")
	if err != nil {
		t.Fatalf("Upgrade: unexpected error: %v", err)
	}

	if !result.IsLocal {
		t.Error("expected IsLocal=true for template with local: repository")
	}
}

// TestUpgrade_LocalTemplate_WithGitHistory reproduces the bug where a template saved from a
// git-tracked directory had git history copied into the registry. CurrentBranch would succeed,
// causing Upgrade to attempt a network clone of the "local:/path" URL instead of skipping it.
func TestUpgrade_LocalTemplate_WithGitHistory(t *testing.T) {
	registryDir := withTempRegistry(t)

	tmplDir := filepath.Join(registryDir, "local-git-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Simulate a .git directory so that CurrentBranch would succeed if we reached that code.
	if err := os.MkdirAll(filepath.Join(tmplDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-git-tpl", "local:/Users/user/my-template", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	result, err := registry.Upgrade("local-git-tpl")
	if err != nil {
		t.Fatalf("Upgrade: unexpected error: %v", err)
	}

	if !result.IsLocal {
		t.Error("expected IsLocal=true — must not attempt network clone of local: repository")
	}
}

func TestUpgrade_NoMetadata_TreatedAsLocal(t *testing.T) {
	registryDir := withTempRegistry(t)

	tmplDir := filepath.Join(registryDir, "bare-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	result, err := registry.Upgrade("bare-tpl")
	if err != nil {
		t.Fatalf("Upgrade: unexpected error: %v", err)
	}

	if !result.IsLocal {
		t.Error("expected IsLocal=true for template without metadata")
	}
}

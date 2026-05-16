package registry_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/specsnl/specs-cli/pkg/registry"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
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

	entry, err := registry.Load("bare-tpl")
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry for existing template")
	}
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
	if err := pkgtemplate.SaveMetadata(tmplDir, "my-tpl", "https://example.com/repo", "main", "abc123", "v1.0.0", created); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	entry, err := registry.Load("my-tpl")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Metadata == nil {
		t.Fatal("expected non-nil Metadata")
	}
	if entry.Metadata.Repository != "https://example.com/repo" {
		t.Errorf("Repository = %q, want %q", entry.Metadata.Repository, "https://example.com/repo")
	}
	if entry.Metadata.Branch != "main" {
		t.Errorf("Branch = %q, want %q", entry.Metadata.Branch, "main")
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
	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "local:/local/path", "", "", "", time.Now().UTC()); err != nil {
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

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-git-tpl", "local:/Users/user/my-template", "", "", "", time.Now().UTC()); err != nil {
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

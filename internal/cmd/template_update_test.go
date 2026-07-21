package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

func TestUpdate_NoArgs_EmptyRegistry(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "update")
	if err != nil {
		t.Fatalf("template update with empty registry: %v", err)
	}
}

func TestUpdate_NamedLocalTemplate_Skipped(t *testing.T) {
	registryDir := withTempRegistry(t)

	// "local:" prefix is what `specs template save` stores in Repository.
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "local:/some/local/path", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// Should succeed: local templates are silently skipped.
	_, err := executeCmd("template", "update", "local-tpl")
	if err != nil {
		t.Fatalf("template update local-tpl: %v", err)
	}
}

func TestUpdate_TooManyArgs(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "update", "a", "b")
	if err == nil {
		t.Fatal("expected error when too many args given")
	}
}

func TestUpdate_NamedLocalTemplate_ProducesNoOutput(t *testing.T) {
	registryDir := withTempRegistry(t)

	// "local:" prefix is what `specs template save` stores in Repository — silently skipped.
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "local:/some/local/path", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "update", "local-tpl")
	if err != nil {
		t.Fatalf("template update local-tpl: %v", err)
	}

	if out != "" {
		t.Errorf("expected no output for local/skipped template, got: %q", out)
	}
}

// TestUpdate_LocalTemplate_WithGitHistory_ProducesNoOutput reproduces the bug where a template
// saved from a git-tracked directory had git history in the registry copy. CurrentBranch would
// succeed, causing CheckRemote to be called with the "local:/path" URL and triggering a DNS error.
func TestUpdate_LocalTemplate_WithGitHistory_ProducesNoOutput(t *testing.T) {
	registryDir := withTempRegistry(t)

	tmplDir := filepath.Join(registryDir, "local-git-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Simulate a .git directory so CurrentBranch would succeed if we reached that code.
	if err := os.MkdirAll(filepath.Join(tmplDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-git-tpl", "local:/Users/user/my-template", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "update", "local-git-tpl")
	if err != nil {
		t.Fatalf("template update local-git-tpl: %v", err)
	}

	if out != "" {
		t.Errorf("expected no output — must not attempt network check of local: repository, got: %q", out)
	}
}

func TestUpdate_NoArgs_EmptyRegistry_ProducesNoOutput(t *testing.T) {
	withTempRegistry(t)

	out, err := executeCmd("template", "update")
	if err != nil {
		t.Fatalf("template update with empty registry: %v", err)
	}

	if out != "" {
		t.Errorf("expected no output for empty registry, got: %q", out)
	}
}

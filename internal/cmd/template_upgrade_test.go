package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

func TestUpgrade_LocalSkipped(t *testing.T) {
	registryDir := withTempRegistry(t)

	// "local:" prefix is what `specs template save` stores in Repository.
	tmplDir := filepath.Join(registryDir, "local-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "local-tpl", "local:/some/local/path", "", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	// Should succeed: local templates are skipped with a notice.
	_, err := executeCmd("template", "upgrade", "local-tpl")
	if err != nil {
		t.Fatalf("template upgrade local-tpl: %v", err)
	}
}

// TestUpgrade_LocalTemplate_WithGitHistory reproduces the bug where a template saved from a
// git-tracked directory had git history in the registry copy. CurrentBranch would succeed,
// causing Upgrade to attempt a network clone of the "local:/path" URL instead of skipping it.
func TestUpgrade_LocalTemplate_WithGitHistory(t *testing.T) {
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

	// Should succeed — must not attempt a network clone of the local: repository.
	_, err := executeCmd("template", "upgrade", "local-git-tpl")
	if err != nil {
		t.Fatalf("template upgrade local-git-tpl: %v", err)
	}
}

func TestUpgrade_NoArgs_EmptyRegistry(t *testing.T) {
	withTempRegistry(t)

	// No args on an empty registry should succeed (nothing to upgrade).
	_, err := executeCmd("template", "upgrade")
	if err != nil {
		t.Fatalf("template upgrade with no args on empty registry: %v", err)
	}
}

func TestUpgrade_NonexistentTemplate(t *testing.T) {
	withTempRegistry(t)

	_, err := executeCmd("template", "upgrade", "does-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
}

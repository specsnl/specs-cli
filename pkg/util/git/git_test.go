//go:build integration

package git_test

import (
	"os"
	"testing"

	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

// TestClone_ShallowClone clones a small public repository and verifies the result.
// Requires network access. Run with: go test -tags=integration ./pkg/util/git/...
func TestClone_ShallowClone(t *testing.T) {
	dir := t.TempDir()

	err := pkggit.Clone("https://github.com/specsnl/php85", dir, pkggit.CloneOptions{
		Depth: 1,
	})
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Verify the repo has content.
	if _, err := os.Stat(dir + "/README.md"); os.IsNotExist(err) {
		t.Error("cloned repo missing README.md")
	}
}

func TestClone_SpecificBranch(t *testing.T) {
	dir := t.TempDir()

	err := pkggit.Clone("https://github.com/specsnl/php85", dir, pkggit.CloneOptions{
		Branch: "main",
		Depth:  1,
	})
	if err != nil {
		t.Fatalf("Clone with branch: %v", err)
	}

	if _, err := os.Stat(dir + "/README.md"); os.IsNotExist(err) {
		t.Error("cloned repo missing README.md")
	}
}

func TestClone_SpecificTag(t *testing.T) {
	dir := t.TempDir()

	err := pkggit.Clone("https://github.com/specsnl/boilr-laravel-project", dir, pkggit.CloneOptions{
		Branch: "0.1.0",
		Depth:  1,
	})
	if err != nil {
		t.Fatalf("Clone with tag: %v", err)
	}

	if _, err := os.Stat(dir + "/composer.json"); os.IsNotExist(err) {
		t.Error("cloned repo missing composer.json")
	}
}

func TestClone_InvalidURL(t *testing.T) {
	dir := t.TempDir()

	err := pkggit.Clone("https://github.com/specsnl/this-repo-does-not-exist-xyz", dir, pkggit.CloneOptions{})
	if err == nil {
		t.Fatal("expected error for non-existent repository, got nil")
	}
}

// TestClone_SSH is intentionally not implemented. SSH transport correctness is delegated
// to go-git's own test suite. Wiring up a self-contained SSH server inside the test
// container would require credentials or a local sshd, neither of which is worth the
// complexity here. SSH URL parsing is covered by pkg/host tests.
func TestClone_SSH(t *testing.T) {
	t.Skip("SSH clone not tested — see comment above")
}

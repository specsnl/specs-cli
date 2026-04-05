package git_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/git"
)

// TestClone_Integration clones a small public repository into a temporary
// directory and verifies the result. This test requires network access and is
// skipped when -short is passed (e.g. in CI).
func TestClone_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network integration test (-short)")
	}

	dir := t.TempDir()

	if err := git.Clone("https://github.com/specsnl/specs-cli.git", dir, ""); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// The cloned repository must contain a .git directory.
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf("expected .git directory in cloned repo: %v", err)
	}
}

package git_test

import (
	"os"
	"path/filepath"
	"testing"

	pkggit "github.com/specsnl/specs-cli/internal/util/git"
)

func TestCheckLocalSource_Missing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	got := pkggit.CheckLocalSource(missing, "abc", "v1.0.0")
	if got.ErrorKind != pkggit.CheckErrorSourceMissing {
		t.Errorf("ErrorKind = %q, want %q", got.ErrorKind, pkggit.CheckErrorSourceMissing)
	}
}

func TestCheckLocalSource_NotARepo(t *testing.T) {
	got := pkggit.CheckLocalSource(t.TempDir(), "abc", "v1.0.0")
	if got.ErrorKind != pkggit.CheckErrorSourceMissing {
		t.Errorf("ErrorKind = %q, want %q", got.ErrorKind, pkggit.CheckErrorSourceMissing)
	}
}

func TestCheckLocalSource_UpToDate(t *testing.T) {
	dir, repo := initRepo(t)
	hash := addCommit(t, repo, dir, "init")
	tagCommit(t, repo, "v1.0.0", hash, false)

	desc, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}

	got := pkggit.CheckLocalSource(dir, desc.Commit, desc.Version)
	if got.ErrorKind != pkggit.CheckErrorNone {
		t.Fatalf("ErrorKind = %q, want none", got.ErrorKind)
	}
	if !got.IsUpToDate {
		t.Errorf("IsUpToDate = false, want true when source matches saved commit/version")
	}
	if got.LatestVersion != "" {
		t.Errorf("LatestVersion = %q, want empty when up-to-date", got.LatestVersion)
	}
}

func TestCheckLocalSource_SourceAdvanced(t *testing.T) {
	dir, repo := initRepo(t)
	base := addCommit(t, repo, dir, "base")
	tagCommit(t, repo, "v1.0.0", base, false)

	saved, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe (saved): %v", err)
	}

	// Source moves forward after the template was saved.
	addCommit(t, repo, dir, "second")
	newDesc, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe (new): %v", err)
	}

	got := pkggit.CheckLocalSource(dir, saved.Commit, saved.Version)
	if got.ErrorKind != pkggit.CheckErrorNone {
		t.Fatalf("ErrorKind = %q, want none", got.ErrorKind)
	}
	if got.IsUpToDate {
		t.Errorf("IsUpToDate = true, want false when source advanced")
	}
	if got.LatestVersion != newDesc.Version {
		t.Errorf("LatestVersion = %q, want %q (current source version)", got.LatestVersion, newDesc.Version)
	}
}

// A clean source saved earlier that later becomes dirty (uncommitted changes on
// the same commit) must not be reported as an update — the "-dirty" marker is
// transient working-tree state, not a new commit (issue #97).
func TestCheckLocalSource_DirtyAfterSaveIsUpToDate(t *testing.T) {
	dir, repo := initRepo(t)
	hash := addCommit(t, repo, dir, "init")
	tagCommit(t, repo, "v1.0.0", hash, false)

	saved, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe (saved): %v", err)
	}

	// Working tree becomes dirty after the template was saved.
	if err := os.WriteFile(filepath.Join(dir, "init.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dirtyDesc, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe (dirty): %v", err)
	}
	if dirtyDesc.Version == saved.Version {
		t.Fatalf("precondition: dirty version %q should differ from saved %q", dirtyDesc.Version, saved.Version)
	}

	got := pkggit.CheckLocalSource(dir, saved.Commit, saved.Version)
	if got.ErrorKind != pkggit.CheckErrorNone {
		t.Fatalf("ErrorKind = %q, want none", got.ErrorKind)
	}
	if !got.IsUpToDate {
		t.Errorf("IsUpToDate = false, want true when only the working tree turned dirty on the saved commit")
	}
	if got.LatestVersion != "" {
		t.Errorf("LatestVersion = %q, want empty when up-to-date", got.LatestVersion)
	}
}

// A template saved while its source was dirty must not be reported as an update
// once the source becomes clean again at the same commit — the reverse of the
// scenario above (issue #97).
func TestCheckLocalSource_SavedDirtyNowCleanIsUpToDate(t *testing.T) {
	dir, repo := initRepo(t)
	hash := addCommit(t, repo, dir, "init")
	tagCommit(t, repo, "v1.0.0", hash, false)

	// Make the tree dirty and capture the version recorded at save time.
	if err := os.WriteFile(filepath.Join(dir, "init.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	savedDirty, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe (dirty): %v", err)
	}

	// Source becomes clean again on the same commit.
	if err := os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0644); err != nil {
		t.Fatalf("WriteFile (restore): %v", err)
	}

	got := pkggit.CheckLocalSource(dir, savedDirty.Commit, savedDirty.Version)
	if got.ErrorKind != pkggit.CheckErrorNone {
		t.Fatalf("ErrorKind = %q, want none", got.ErrorKind)
	}
	if !got.IsUpToDate {
		t.Errorf("IsUpToDate = false, want true when the source returned clean on the saved commit")
	}
}

func TestCheckLocalSource_ErrIsSourceMissing(t *testing.T) {
	got := pkggit.CheckLocalSource(filepath.Join(t.TempDir(), "nope"), "abc", "v1.0.0")
	if err := got.Err(); err != pkggit.ErrCheckSourceMissing {
		t.Errorf("Err() = %v, want %v", err, pkggit.ErrCheckSourceMissing)
	}
}

package git_test

import (
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

func TestCheckLocalSource_ErrIsSourceMissing(t *testing.T) {
	got := pkggit.CheckLocalSource(filepath.Join(t.TempDir(), "nope"), "abc", "v1.0.0")
	if err := got.Err(); err != pkggit.ErrCheckSourceMissing {
		t.Errorf("Err() = %v, want %v", err, pkggit.ErrCheckSourceMissing)
	}
}

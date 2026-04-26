package git_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

var testSig = &object.Signature{
	Name:  "Test",
	Email: "test@example.com",
	When:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
}

func initRepo(t *testing.T) (string, *gogit.Repository) {
	t.Helper()
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	return dir, repo
}

// addCommit writes a uniquely-named file, stages it, and commits.
func addCommit(t *testing.T, repo *gogit.Repository, dir, label string) plumbing.Hash {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, label+".txt"), []byte(label), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := wt.Add(label + ".txt"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	hash, err := wt.Commit(label, &gogit.CommitOptions{Author: testSig})
	if err != nil {
		t.Fatalf("Commit(%q): %v", label, err)
	}
	return hash
}

// tagCommit creates a lightweight tag when annotated is false, or an annotated tag otherwise.
func tagCommit(t *testing.T, repo *gogit.Repository, name string, hash plumbing.Hash, annotated bool) {
	t.Helper()
	var opts *gogit.CreateTagOptions
	if annotated {
		opts = &gogit.CreateTagOptions{Tagger: testSig, Message: name}
	}
	if _, err := repo.CreateTag(name, hash, opts); err != nil {
		t.Fatalf("CreateTag(%q): %v", name, err)
	}
}

func TestDescribe_ExactLightweightTag(t *testing.T) {
	dir, repo := initRepo(t)
	hash := addCommit(t, repo, dir, "init")
	tagCommit(t, repo, "v1.0.0", hash, false)

	got, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got.Commit != hash.String() {
		t.Errorf("Commit = %q, want %q", got.Commit, hash.String())
	}
	if got.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", got.Version, "v1.0.0")
	}
}

func TestDescribe_ExactAnnotatedTag(t *testing.T) {
	dir, repo := initRepo(t)
	hash := addCommit(t, repo, dir, "init")
	tagCommit(t, repo, "v2.0.0", hash, true)

	got, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got.Version != "v2.0.0" {
		t.Errorf("Version = %q, want %q", got.Version, "v2.0.0")
	}
}

func TestDescribe_AheadOfTag(t *testing.T) {
	dir, repo := initRepo(t)
	base := addCommit(t, repo, dir, "base")
	tagCommit(t, repo, "v1.0.0", base, false)
	addCommit(t, repo, dir, "second")
	head := addCommit(t, repo, dir, "third")

	got, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got.Commit != head.String() {
		t.Errorf("Commit = %q, want %q", got.Commit, head.String())
	}
	want := fmt.Sprintf("v1.0.0-2-g%s", got.Commit[:7])
	if got.Version != want {
		t.Errorf("Version = %q, want %q", got.Version, want)
	}
}

func TestDescribe_NoTags(t *testing.T) {
	dir, repo := initRepo(t)
	hash := addCommit(t, repo, dir, "init")

	got, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got.Version != hash.String()[:7] {
		t.Errorf("Version = %q, want %q", got.Version, hash.String()[:7])
	}
}

func TestDescribe_DirtyWorktree(t *testing.T) {
	dir, repo := initRepo(t)
	hash := addCommit(t, repo, dir, "init")
	tagCommit(t, repo, "v1.0.0", hash, false)
	// Modify a tracked file without staging.
	if err := os.WriteFile(filepath.Join(dir, "init.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got.Version != "v1.0.0-dirty" {
		t.Errorf("Version = %q, want %q", got.Version, "v1.0.0-dirty")
	}
}

func TestDescribe_UntrackedFileIsNotDirty(t *testing.T) {
	dir, repo := initRepo(t)
	hash := addCommit(t, repo, dir, "init")
	tagCommit(t, repo, "v1.0.0", hash, false)
	// Untracked files must not trigger -dirty (matches git describe --dirty behaviour).
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got.Version != "v1.0.0" {
		t.Errorf("Version = %q, want %q", got.Version, "v1.0.0")
	}
}

func TestDescribe_AheadAndDirty(t *testing.T) {
	dir, repo := initRepo(t)
	base := addCommit(t, repo, dir, "base")
	tagCommit(t, repo, "v1.0.0", base, false)
	addCommit(t, repo, dir, "second")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := pkggit.Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	want := fmt.Sprintf("v1.0.0-1-g%s-dirty", got.Commit[:7])
	if got.Version != want {
		t.Errorf("Version = %q, want %q", got.Version, want)
	}
}

func TestDescribe_NotARepo(t *testing.T) {
	_, err := pkggit.Describe(t.TempDir())
	if err == nil {
		t.Error("Describe: expected error for non-repo directory, got nil")
	}
}

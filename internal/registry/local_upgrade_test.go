package registry_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/specsnl/specs-cli/internal/registry"
	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
	"github.com/specsnl/specs-cli/internal/util/osutil"
)

var upgradeSig = &object.Signature{Name: "Test", Email: "test@example.com", When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}

// newSourceRepo creates a git repo with a single tagged commit and returns its path.
func newSourceRepo(t *testing.T) (string, *gogit.Repository) {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	commitFile(t, repo, dir, "init")

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}

	if _, err := repo.CreateTag("1.0.0", head.Hash(), nil); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	return dir, repo
}

func commitFile(t *testing.T, repo *gogit.Repository, dir, label string) plumbing.Hash {
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

	hash, err := wt.Commit(label, &gogit.CommitOptions{Author: upgradeSig})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	return hash
}

// saveLocalTemplate mimics `specs template save`: copies src into the registry and
// writes metadata with a "local:" repository and the source's describe output.
func saveLocalTemplate(t *testing.T, registryDir, name, src string) string {
	t.Helper()

	dest := filepath.Join(registryDir, name)
	if err := osutil.CopyDir(src, dest); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	desc, err := pkggit.Describe(src)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}

	now := time.Now().UTC()
	if err := pkgtemplate.SaveMetadata(dest, name, "local:"+src, "", desc.Commit, desc.Version, now, now); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	return dest
}

func TestUpgrade_LocalGitSource_UpToDate(t *testing.T) {
	registryDir := withTempRegistry(t)
	src, _ := newSourceRepo(t)
	saveLocalTemplate(t, registryDir, "loc", src)

	res, err := registry.Upgrade("loc")
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	if !res.AlreadyUpToDate {
		t.Errorf("expected AlreadyUpToDate=true when source is unchanged, got %+v", res)
	}

	if res.IsLocal {
		t.Error("expected IsLocal=false for a git-tracked local source")
	}
}

func TestUpgrade_LocalGitSource_Advanced(t *testing.T) {
	registryDir := withTempRegistry(t)
	src, repo := newSourceRepo(t)
	dest := saveLocalTemplate(t, registryDir, "loc", src)

	before, _ := pkgtemplate.LoadMetadata(dest)

	// Source advances after save.
	commitFile(t, repo, src, "second")
	newDesc, _ := pkggit.Describe(src)

	res, err := registry.Upgrade("loc")
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	if res.AlreadyUpToDate || res.IsLocal {
		t.Fatalf("expected an actual upgrade, got %+v", res)
	}

	after, _ := pkgtemplate.LoadMetadata(dest)
	if after.Commit == before.Commit {
		t.Error("expected Commit to change after upgrade")
	}

	if after.Commit != newDesc.Commit {
		t.Errorf("Commit = %q, want %q", after.Commit, newDesc.Commit)
	}

	if !after.Created.Equal(before.Created.Time) {
		t.Error("Created should be preserved across upgrade")
	}
	// The freshly copied file must be present in the registry.
	if _, err := os.Stat(filepath.Join(dest, "second.txt")); err != nil {
		t.Errorf("expected 'second.txt' copied into registry: %v", err)
	}
	// Stale status must be removed so it regenerates.
	if _, err := os.Stat(filepath.Join(dest, specs.StatusFile)); !os.IsNotExist(err) {
		t.Errorf("expected status file removed after upgrade, stat err = %v", err)
	}
}

func TestUpgrade_LocalGitSource_Missing(t *testing.T) {
	registryDir := withTempRegistry(t)
	src, _ := newSourceRepo(t)
	saveLocalTemplate(t, registryDir, "loc", src)

	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}

	_, err := registry.Upgrade("loc")
	if !errors.Is(err, specs.ErrLocalSourceMissing) {
		t.Errorf("expected ErrLocalSourceMissing, got %v", err)
	}
}

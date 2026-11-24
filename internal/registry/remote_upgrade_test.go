package registry_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/specsnl/specs-cli/internal/registry"
	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
)

// newRemoteTemplate creates an untagged git repo holding a template and
// registers a clone of it as name, the way `template download` does.
func newRemoteTemplate(t *testing.T, name string) (src string, repo *gogit.Repository, root string) {
	t.Helper()

	src = t.TempDir()

	repo, err := gogit.PlainInit(src, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	writeAndCommit(t, repo, src, map[string]string{"project.yaml": "Name: x\n", "template/a.txt": "a"}, nil)

	// A file:// URL rather than a path, which would make it a local template.
	url := "file://" + filepath.ToSlash(src)

	root = specs.TemplatePath(name)
	if err := pkggit.Clone(url, root, pkggit.CloneOptions{Branch: "master"}); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	desc, _ := pkggit.Describe(root)

	now := time.Now().UTC()
	if err := pkgtemplate.SaveMetadata(root, name, url, "master", desc.Commit, desc.Version, now, now); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	return src, repo, root
}

func writeAndCommit(t *testing.T, repo *gogit.Repository, dir string, files, links map[string]string) {
	t.Helper()

	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for rel, target := range links {
		if err := os.Symlink(target, filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}

	if err := wt.AddGlob("."); err != nil {
		t.Fatalf("AddGlob: %v", err)
	}

	if _, err := wt.Commit("commit", &gogit.CommitOptions{Author: upgradeSig}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
}

func TestUpgrade_Remote_ReplacesTheClone(t *testing.T) {
	withTempRegistry(t)
	src, repo, root := newRemoteTemplate(t, "remote")

	writeAndCommit(t, repo, src, map[string]string{"template/.nvmrc": "26"}, map[string]string{"template/.node-version": ".nvmrc"})

	res, err := registry.Upgrade("remote")
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	if res.AlreadyUpToDate || res.IsLocal {
		t.Fatalf("expected an actual upgrade, got %+v", res)
	}

	if got, err := os.Readlink(filepath.Join(root, "template", ".node-version")); err != nil || got != ".nvmrc" {
		t.Errorf(".node-version → %q (%v), want .nvmrc", got, err)
	}

	entries, err := os.ReadDir(specs.ConfigDir())
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if e.Name() != specs.TemplateDirName {
			t.Errorf("staging left behind: %s", e.Name())
		}
	}
}

func TestUpgrade_Remote_OutsideLinkKeepsTheRegisteredTemplate(t *testing.T) {
	withTempRegistry(t)
	src, repo, root := newRemoteTemplate(t, "remote")

	before, _ := pkgtemplate.LoadMetadata(root)

	writeAndCommit(t, repo, src, nil, map[string]string{"template/leak": "../../outside"})

	_, err := registry.Upgrade("remote")
	if !errors.Is(err, specs.ErrSymlinkOutside) {
		t.Fatalf("Upgrade error = %v, want ErrSymlinkOutside", err)
	}

	after, err := pkgtemplate.LoadMetadata(root)
	if err != nil {
		t.Fatalf("the registered template should survive a rejected upgrade: %v", err)
	}

	if after.Commit != before.Commit {
		t.Errorf("Commit = %q, want the pre-upgrade %q", after.Commit, before.Commit)
	}

	if _, err := os.Lstat(filepath.Join(root, "template", "leak")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the rejected link should not reach the registry: %v", err)
	}
}

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
)

var localSig = &object.Signature{Name: "t", Email: "t@t.com", When: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}

func localCommit(t *testing.T, repo *gogit.Repository, dir, label string) plumbing.Hash {
	t.Helper()

	wt, _ := repo.Worktree()

	if err := os.WriteFile(filepath.Join(dir, label+".txt"), []byte(label), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := wt.Add(label + ".txt"); err != nil {
		t.Fatal(err)
	}

	h, err := wt.Commit(label, &gogit.CommitOptions{Author: localSig})
	if err != nil {
		t.Fatal(err)
	}

	return h
}

// makeLocalTemplate creates a source git repo (tagged 1.0.0) and registers it as a
// "local:" template, returning (sourcePath, templateDir).
func makeLocalTemplate(t *testing.T, registryDir, name string) (string, *gogit.Repository, string) {
	t.Helper()
	src := t.TempDir()

	repo, err := gogit.PlainInit(src, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	h := localCommit(t, repo, src, "init")
	if _, err := repo.CreateTag("1.0.0", h, nil); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	desc, err := pkggit.Describe(src)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}

	tmplDir := filepath.Join(registryDir, name)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	if err := pkgtemplate.SaveMetadata(tmplDir, name, "local:"+src, "", desc.Commit, desc.Version, now, now); err != nil {
		t.Fatalf("SaveMetadata: %v", err)
	}

	return src, repo, tmplDir
}

func TestList_LocalSource_UpToDate(t *testing.T) {
	registryDir := withTempRegistry(t)
	makeLocalTemplate(t, registryDir, "loc")

	out, err := executeCmd("template", "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}

	if !strings.Contains(out, "up-to-date") {
		t.Errorf("expected 'up-to-date' for unchanged local source, got: %q", out)
	}
}

func TestList_LocalSource_Advanced(t *testing.T) {
	registryDir := withTempRegistry(t)
	src, repo, _ := makeLocalTemplate(t, registryDir, "loc")

	// Source path moves ahead of what was saved.
	localCommit(t, repo, src, "second")

	out, err := executeCmd("template", "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}

	if !strings.Contains(out, "update") {
		t.Errorf("expected an 'update' status when local source advanced, got: %q", out)
	}

	if strings.Contains(out, "up-to-date") {
		t.Errorf("expected NOT up-to-date when local source advanced, got: %q", out)
	}
}

func TestList_LocalSource_Missing(t *testing.T) {
	registryDir := withTempRegistry(t)
	src, _, _ := makeLocalTemplate(t, registryDir, "loc")

	if err := os.RemoveAll(src); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("template", "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}

	if !strings.Contains(out, "source missing") {
		t.Errorf("expected 'source missing' when local source path is gone, got: %q", out)
	}
}

// TestList_VersionMismatchForcesRefresh verifies that a fresh status written by a
// different specs version is re-checked rather than trusted.
func TestList_VersionMismatchForcesRefresh(t *testing.T) {
	registryDir := withTempRegistry(t)

	tmplDir := filepath.Join(registryDir, "remote-tpl")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := pkgtemplate.SaveMetadata(tmplDir, "remote-tpl", "https://example.com/repo", "main", "", "", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// Fresh (not stale) status, but written by a different/older version.
	seeded := &pkgtemplate.TemplateStatus{
		CheckedAt:    pkgtemplate.JSONTime{Time: time.Now()},
		IsUpToDate:   false,
		SpecsVersion: "0.0.1-old",
	}
	if err := pkgtemplate.SaveStatus(tmplDir, seeded); err != nil {
		t.Fatal(err)
	}

	var called atomic.Bool

	fake := func(_ context.Context, _, _, _ string) pkggit.RemoteCheckResult {
		called.Store(true)
		return pkggit.RemoteCheckResult{IsUpToDate: true}
	}

	out, err := executeCmdWithCheckFn(fake, "template", "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}

	if !called.Load() {
		t.Error("expected a refresh (checkRemoteFn call) when stored SpecsVersion differs")
	}

	if !strings.Contains(out, "up-to-date") {
		t.Errorf("expected refreshed 'up-to-date' status, got: %q", out)
	}
}

package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

// makeFakeTemplate creates a minimal template directory structure in dir.
func makeFakeTemplate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, specs.TemplateDirFile), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte("variables: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestSave_Success(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatalf("template save: %v", err)
	}

	if _, err := os.Stat(specs.TemplatePath("my-tpl")); err != nil {
		t.Errorf("expected registry entry to exist: %v", err)
	}
}

// "template %q saved" is narration about a side effect, not a product: a caller
// redirecting stdout captures nothing.
func TestSave_NarratesOnStderrOnly(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)

	stdout, stderr, err := executeCmdStreams("template", "save", src, "my-tpl")
	if err != nil {
		t.Fatalf("template save: %v", err)
	}

	if stdout != "" {
		t.Errorf("expected nothing on stdout, got: %q", stdout)
	}

	if !strings.Contains(stderr, `template "my-tpl" saved`) {
		t.Errorf("expected the confirmation on stderr, got: %q", stderr)
	}
}

func TestSave_AlreadyExists(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatal(err)
	}

	_, err := executeCmd("template", "save", src, "my-tpl")
	if err == nil {
		t.Fatal("expected error on duplicate save")
	}
}

func TestSave_AlreadyExists_IsErrTemplateAlreadyExists(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatal(err)
	}

	_, err := executeCmd("template", "save", src, "my-tpl")
	if !errors.Is(err, specs.ErrTemplateAlreadyExists) {
		t.Errorf("expected ErrTemplateAlreadyExists, got %v", err)
	}
}

func TestSave_Force(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "save", "--force", src, "my-tpl"); err != nil {
		t.Fatalf("template save --force: %v", err)
	}
}

func TestSave_InvalidName(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)

	_, err := executeCmd("template", "save", src, "bad name")
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}

// The stored value is a bare path — no "local:" marker, and $HOME written as ~.
// It must be recognised as local and resolve back to the directory saved from.
func TestSave_StoresLocalPathWithoutAMarker(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatalf("template save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(specs.TemplatePath("my-tpl"), specs.MetadataFile))
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}

	var meta pkgtemplate.Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing metadata: %v", err)
	}

	if strings.HasPrefix(meta.Repository, "local:") {
		t.Errorf("Repository still carries the dropped marker: %q", meta.Repository)
	}

	if !pkgtemplate.IsLocalRepository(meta.Repository) {
		t.Errorf("Repository %q is not recognised as a local source", meta.Repository)
	}

	// Either an absolute path or a ~-relative one, depending on where the test
	// temp directory lives; both must resolve to the directory saved from.
	if !filepath.IsAbs(meta.Repository) && !strings.HasPrefix(meta.Repository, "~") {
		t.Errorf("Repository %q is neither absolute nor home-relative", meta.Repository)
	}

	wantAbs, err := filepath.Abs(src)
	if err != nil {
		t.Fatalf("resolving %q: %v", src, err)
	}

	if got := pkgtemplate.LocalSourcePath(meta.Repository); got != wantAbs {
		t.Errorf("Repository %q resolves to %q, want %q", meta.Repository, got, wantAbs)
	}
}

// A template saved by an older version keeps working: its "local:" value is
// still recognised, and template list reports a status for it rather than
// treating it as a remote to clone.
func TestSave_LegacyLocalPrefixStillReadable(t *testing.T) {
	withTempRegistry(t)

	src := makeFakeTemplate(t)
	if _, err := executeCmd("template", "save", src, "my-tpl"); err != nil {
		t.Fatalf("template save: %v", err)
	}

	path := filepath.Join(specs.TemplatePath("my-tpl"), specs.MetadataFile)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}

	var meta pkgtemplate.Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("parsing metadata: %v", err)
	}

	// Rewrite it the way an older version would have.
	absPath, err := filepath.Abs(src)
	if err != nil {
		t.Fatalf("resolving %q: %v", src, err)
	}

	meta.Repository = "local:" + absPath

	rewritten, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("marshalling metadata: %v", err)
	}

	if err := os.WriteFile(path, rewritten, 0o644); err != nil {
		t.Fatalf("writing metadata: %v", err)
	}

	if !pkgtemplate.IsLocalRepository(meta.Repository) {
		t.Fatalf("legacy value %q is not recognised as local", meta.Repository)
	}

	if got := pkgtemplate.LocalSourcePath(meta.Repository); got != absPath {
		t.Errorf("legacy value %q resolves to %q, want %q", meta.Repository, got, absPath)
	}

	out, err := executeCmd("template", "list")
	if err != nil {
		t.Fatalf("template list against legacy metadata: %v", err)
	}

	if !strings.Contains(out, "my-tpl") {
		t.Errorf("template list did not report the legacy template: %q", out)
	}
}

package cmd

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	"github.com/specsnl/specs-cli/internal/util/exit"
)

// buildLinkedTemplate writes a template with a file link, a directory link, a
// templated link and the given extra links into dir.
func buildLinkedTemplate(t *testing.T, dir string, extra map[string]string) {
	t.Helper()

	buildMinimalTemplate(t, dir, "Name: acme\n", ".nvmrc", "26")

	tplDir := filepath.Join(dir, specs.TemplateDirFile)
	files := map[string]string{"real/f.txt": "f", "{{ .Name }}.rc": "rc"}

	for rel, content := range files {
		path := filepath.Join(tplDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	links := map[string]string{".node-version": ".nvmrc", "alias": "real", "cfg": "{{ .Name }}.rc"}
	maps.Copy(links, extra)

	for link, target := range links {
		if err := os.Symlink(target, filepath.Join(tplDir, link)); err != nil {
			t.Fatal(err)
		}
	}
}

func assertOutputLinks(t *testing.T, dir string) {
	t.Helper()

	for link, want := range map[string]string{".node-version": ".nvmrc", "alias": "real", "cfg": "acme.rc"} {
		got, err := os.Readlink(filepath.Join(dir, link))
		if err != nil {
			t.Errorf("%s is not a link: %v", link, err)
			continue
		}

		if got != want {
			t.Errorf("%s → %q, want %q", link, got, want)
		}
	}
}

func TestUse_LocalTemplateKeepsLinks(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	srcDir := t.TempDir()
	buildLinkedTemplate(t, srcDir, map[string]string{"leak.txt": outside, "broken": "nope", "up": "../project.yaml"})

	targetDir := t.TempDir()

	out, err := executeCmd("use", "--use-defaults", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v\n%s", err, out)
	}

	assertOutputLinks(t, targetDir)

	for _, skipped := range []string{"leak.txt", "broken", "up"} {
		if _, err := os.Lstat(filepath.Join(targetDir, skipped)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s should not be in the output: %v", skipped, err)
		}

		if !strings.Contains(out, "template/"+skipped+" →") {
			t.Errorf("output should warn about %s:\n%s", skipped, out)
		}
	}
}

func TestTemplateUse_RegisteredTemplateKeepsLinks(t *testing.T) {
	withTempRegistry(t)

	srcDir := t.TempDir()
	buildLinkedTemplate(t, srcDir, nil)

	if out, err := executeCmd("template", "save", srcDir, "linked"); err != nil {
		t.Fatalf("save: %v\n%s", err, out)
	}

	targetDir := t.TempDir()
	if out, err := executeCmd("template", "use", "--use-defaults", "linked", targetDir); err != nil {
		t.Fatalf("template use: %v\n%s", err, out)
	}

	assertOutputLinks(t, targetDir)
}

func TestTemplateValidate_ReportsLinks(t *testing.T) {
	dir := t.TempDir()
	buildLinkedTemplate(t, dir, map[string]string{"broken": "nope", "up": "../project.yaml"})

	out, err := executeCmd("template", "validate", dir)

	if code := validateExitCode(err); code != exit.ValidateLinks {
		t.Fatalf("exit code = %d, want %d (err=%v)\n%s", code, exit.ValidateLinks, err, out)
	}

	for _, want := range []string{"template/broken → nope", "template/up → ../project.yaml"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should mention %q:\n%s", want, out)
		}
	}
}

func TestTemplateValidate_InBoundsLinksAreValid(t *testing.T) {
	dir := t.TempDir()
	buildLinkedTemplate(t, dir, nil)

	if out, err := executeCmd("template", "validate", dir); err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
}

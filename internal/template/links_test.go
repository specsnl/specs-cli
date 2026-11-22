package template_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

// addLinks creates symlinks under root/template/; links maps a path relative to
// template/ to its target.
func addLinks(t *testing.T, root string, links map[string]string) {
	t.Helper()

	for link, target := range links {
		path := filepath.Join(root, specs.TemplateDirFile, filepath.FromSlash(link))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink(target, path); err != nil {
			t.Fatalf("symlink %s → %s: %v", link, target, err)
		}
	}
}

func executeLinks(t *testing.T, yaml string, files map[string][]byte, links map[string]string) (*pkgtemplate.Template, string) {
	t.Helper()

	root := buildTemplate(t, yaml, files)
	addLinks(t, root, links)

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	return tmpl, target
}

func assertLink(t *testing.T, dir, link, want string) {
	t.Helper()

	got, err := os.Readlink(filepath.Join(dir, filepath.FromSlash(link)))
	if err != nil {
		t.Fatalf("%s is not a link: %v", link, err)
	}

	if got != want {
		t.Errorf("%s → %q, want %q", link, got, want)
	}
}

func assertAbsent(t *testing.T, dir, path string) {
	t.Helper()

	if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(path))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s should not be in the output: %v", path, err)
	}
}

func assertLinkWarning(t *testing.T, tmpl *pkgtemplate.Template, path string, want error) {
	t.Helper()

	for _, w := range tmpl.LinkWarnings {
		if w.Path == path {
			if !errors.Is(w, want) {
				t.Errorf("%s: warning = %v, want %v", path, w, want)
			}

			return
		}
	}

	t.Errorf("no link warning for %s in %v", path, tmpl.LinkWarnings)
}

func TestExecute_FileLinkStaysALink(t *testing.T) {
	tmpl, target := executeLinks(t, "Name: x\n",
		map[string][]byte{".nvmrc": []byte("26")},
		map[string]string{".node-version": ".nvmrc"})

	assertLink(t, target, ".node-version", ".nvmrc")

	if got := readFile(t, target, ".node-version"); got != "26" {
		t.Errorf(".node-version reads %q, want 26", got)
	}

	if len(tmpl.LinkWarnings) != 0 {
		t.Errorf("LinkWarnings = %v, want none", tmpl.LinkWarnings)
	}
}

func TestExecute_DirectoryLinkStaysALink(t *testing.T) {
	_, target := executeLinks(t, "Name: x\n",
		map[string][]byte{"real/f.txt": []byte("f")},
		map[string]string{"alias": "real"})

	assertLink(t, target, "alias", "real")

	if got := readFile(t, target, "alias/f.txt"); got != "f" {
		t.Errorf("alias/f.txt reads %q, want f", got)
	}
}

func TestExecute_OutsideLinkIsSkipped(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	tmpl, target := executeLinks(t, "Name: x\n", nil, map[string]string{
		"leak.txt": outside,
		"rel.txt":  "../project.yaml",
	})

	assertAbsent(t, target, "leak.txt")
	assertAbsent(t, target, "rel.txt")
	assertLinkWarning(t, tmpl, "template/leak.txt", specs.ErrSymlinkOutside)
	assertLinkWarning(t, tmpl, "template/rel.txt", specs.ErrSymlinkOutside)
}

func TestExecute_ChainThatEscapesIsSkipped(t *testing.T) {
	tmpl, target := executeLinks(t, "Name: x\n", nil, map[string]string{
		"a": "b",
		"b": "../project.yaml",
	})

	assertAbsent(t, target, "a")
	assertLinkWarning(t, tmpl, "template/a", specs.ErrSymlinkOutside)
}

func TestExecute_DanglingLinkIsSkipped(t *testing.T) {
	tmpl, target := executeLinks(t, "Name: x\n",
		map[string][]byte{"keep.txt": []byte("kept")},
		map[string]string{"broken": "nope"})

	assertAbsent(t, target, "broken")
	assertLinkWarning(t, tmpl, "template/broken", specs.ErrSymlinkDangling)

	if got := readFile(t, target, "keep.txt"); got != "kept" {
		t.Errorf("keep.txt = %q, want kept", got)
	}
}

func TestExecute_LinkTargetIsRendered(t *testing.T) {
	_, target := executeLinks(t, "Name: acme\n",
		map[string][]byte{"{{ .Name }}.rc": []byte("rc")},
		map[string]string{"{{ .Name }}.link": "{{ .Name }}.rc"})

	assertLink(t, target, "acme.link", "acme.rc")
}

func TestExecute_VerbatimLinkTargetIsNotRendered(t *testing.T) {
	root := buildTemplate(t, "Name: acme\n", map[string][]byte{"{{ .Name }}.rc": []byte("rc")})
	addLinks(t, root, map[string]string{"raw": "{{ .Name }}.rc"})

	if err := os.WriteFile(filepath.Join(root, specs.VerbatimFile), []byte("raw\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// The target keeps its template syntax, which names no file in the output.
	assertAbsent(t, target, "raw")
	assertLinkWarning(t, tmpl, "raw", specs.ErrSymlinkDangling)
}

func TestExecute_LinkToAFileThatIsNotWrittenIsRemoved(t *testing.T) {
	tmpl, target := executeLinks(t, "UseRc: false\n",
		map[string][]byte{
			"{{ if .UseRc }}app.rc{{ end }}": []byte("rc"),
			"empty.rc":                       []byte("{{ if .UseRc }}rc{{ end }}"),
		},
		map[string]string{"cfg": "{{ if .UseRc }}app.rc{{ end }}", "empty": "empty.rc"})

	assertAbsent(t, target, "cfg")
	assertAbsent(t, target, "empty")
	assertLinkWarning(t, tmpl, "cfg", specs.ErrSymlinkDangling)
	assertLinkWarning(t, tmpl, "empty", specs.ErrSymlinkDangling)
}

// The file the target names renders with an empty segment, so it is skipped
// rather than written outside the output.
const escapingDir = "Dir: \"..//\"\n"

func TestExecute_RenderedTargetThatEscapesIsRemoved(t *testing.T) {
	tmpl, target := executeLinks(t, escapingDir,
		map[string][]byte{"{{ .Dir }}x": []byte("x")},
		map[string]string{"escape": "{{ .Dir }}x"})

	assertAbsent(t, target, "escape")
	assertLinkWarning(t, tmpl, "escape", specs.ErrSymlinkOutside)
}

func TestExecute_LinkThroughAnEscapingLinkIsRemoved(t *testing.T) {
	tmpl, target := executeLinks(t, escapingDir,
		map[string][]byte{"{{ .Dir }}x": []byte("x")},
		map[string]string{"a": "b", "b": "{{ .Dir }}x"})

	assertAbsent(t, target, "a")
	assertAbsent(t, target, "b")
	assertLinkWarning(t, tmpl, "a", specs.ErrSymlinkOutside)
	assertLinkWarning(t, tmpl, "b", specs.ErrSymlinkOutside)
}

func TestGet_LinkTargetVariablesAreReferenced(t *testing.T) {
	root := buildTemplate(t, "Name: acme\nUnused: x\n", map[string][]byte{"{{ .Name }}.rc": []byte("rc")})
	addLinks(t, root, map[string]string{"cfg": "{{ .Name }}.rc", "alias": "."})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if !tmpl.Referenced["Name"] {
		t.Error("Name, used only in a link target and a file name, should be referenced")
	}

	issues, err := tmpl.Validate()
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	for _, iss := range issues {
		if errors.Is(iss, pkgtemplate.ErrUnknownVariable) {
			t.Errorf("unexpected issue %v", iss)
		}
	}
}

func TestGet_DoesNotReadThroughLinks(t *testing.T) {
	root := buildTemplate(t, "Name: x\n", nil)
	addLinks(t, root, map[string]string{"dir": ".", "broken": "nope", "leak": "../project.yaml"})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if _, err := tmpl.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

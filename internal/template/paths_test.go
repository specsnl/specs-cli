package template_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

// executeInSandbox renders the template into sandbox/a/b/out, so a path that
// climbs two levels lands in sandbox/escaped rather than somewhere real.
func executeInSandbox(t *testing.T, yaml string, files map[string][]byte, cfg pkgtemplate.Config) (tmpl *pkgtemplate.Template, sandbox, target string, err error) {
	t.Helper()

	root := buildTemplate(t, yaml, files)

	tmpl, err = pkgtemplate.Get(root, cfg)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	sandbox = t.TempDir()
	target = filepath.Join(sandbox, "a", "b", "out")

	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}

	return tmpl, sandbox, target, tmpl.Execute(target)
}

func assertPathWarnings(t *testing.T, tmpl *pkgtemplate.Template, want ...string) {
	t.Helper()

	var got []string
	for _, w := range tmpl.PathWarnings {
		got = append(got, w.Path+" → "+w.Rendered)
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("PathWarnings = %q, want %q", got, want)
	}
}

func TestExecute_EscapingFileIsSkipped(t *testing.T) {
	tmpl, sandbox, target, err := executeInSandbox(t, "Dir: ../../escaped\n", map[string][]byte{
		"keep.txt":          []byte("keep"),
		"{{ .Dir }}/x.txt":  []byte("x"),
		"{{ .Dir }}.txt":    []byte("y"),
		"sub/{{ .Dir }}.md": []byte("z"),
	}, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := readFile(t, target, "keep.txt"); got != "keep" {
		t.Errorf("keep.txt = %q", got)
	}

	if fileExists(t, sandbox, "a/escaped.txt") || fileExists(t, sandbox, "a/b/escaped.md") {
		t.Error("an escaping file was written outside the output")
	}

	assertPathWarnings(t, tmpl,
		"template/sub/{{ .Dir }}.md → sub/../../escaped.md",
		"template/{{ .Dir }} → ../../escaped",
		"template/{{ .Dir }}.txt → ../../escaped.txt",
	)

	if !errors.Is(tmpl.PathWarnings[0], specs.ErrPathOutside) {
		t.Errorf("warning should wrap ErrPathOutside: %v", tmpl.PathWarnings[0])
	}
}

func TestExecute_EscapingDirectorySkipsItsSubtree(t *testing.T) {
	tmpl, sandbox, _, err := executeInSandbox(t, "Dir: ../../escaped\n", map[string][]byte{
		"{{ .Dir }}/x.txt":        []byte("x"),
		"{{ .Dir }}/nested/y.txt": []byte("y"),
	}, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fileExists(t, sandbox, "a/escaped") {
		t.Error("the escaping directory was created outside the output")
	}

	assertPathWarnings(t, tmpl, "template/{{ .Dir }} → ../../escaped")
}

func TestExecute_AbsoluteRenderedPathIsNotWritten(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "abs")

	tmpl, _, target, err := executeInSandbox(t, "Dir: "+abs+"\n", map[string][]byte{
		"{{ .Dir }}/x.txt": []byte("x"),
	}, pkgtemplate.Config{RejectEscapingPaths: true})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if _, err := os.Stat(abs); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an absolute rendered path was written: %v", err)
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Errorf("output should be empty, has %d entries", len(entries))
	}

	assertPathWarnings(t, tmpl)
}

func TestExecute_DotDotThatStaysInsideIsKept(t *testing.T) {
	tmpl, _, target, err := executeInSandbox(t, "Dir: a/../b\n", map[string][]byte{
		"{{ .Dir }}/x.txt": []byte("x"),
	}, pkgtemplate.Config{RejectEscapingPaths: true})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := readFile(t, target, "b/x.txt"); got != "x" {
		t.Errorf("b/x.txt = %q", got)
	}

	assertPathWarnings(t, tmpl)
}

func TestExecute_EscapingPathAbortsWhenRejected(t *testing.T) {
	tmpl, sandbox, _, err := executeInSandbox(t, "Dir: ../../escaped\n", map[string][]byte{
		"keep.txt":         []byte("keep"),
		"{{ .Dir }}/x.txt": []byte("x"),
		"{{ .Dir }}.txt":   []byte("y"),
	}, pkgtemplate.Config{RejectEscapingPaths: true})
	if !errors.Is(err, specs.ErrPathOutside) {
		t.Fatalf("Execute error = %v, want ErrPathOutside", err)
	}

	for _, want := range []string{
		"template/{{ .Dir }} → ../../escaped",
		"template/{{ .Dir }}.txt → ../../escaped.txt",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %q: %v", want, err)
		}
	}

	if fileExists(t, sandbox, "a/escaped") || fileExists(t, sandbox, "a/escaped.txt") {
		t.Error("an escaping path was written outside the output")
	}

	if len(tmpl.PathWarnings) != 2 {
		t.Errorf("PathWarnings = %v, want both escaping paths", tmpl.PathWarnings)
	}
}

func TestMetadata_IsRemote(t *testing.T) {
	tests := []struct {
		name string
		meta *pkgtemplate.Metadata
		want bool
	}{
		{name: "nil", meta: nil, want: false},
		{name: "no repository", meta: &pkgtemplate.Metadata{}, want: false},
		{name: "local path", meta: &pkgtemplate.Metadata{Repository: "/home/me/tpl"}, want: false},
		{name: "legacy local prefix", meta: &pkgtemplate.Metadata{Repository: "local:/home/me/tpl"}, want: false},
		{name: "https", meta: &pkgtemplate.Metadata{Repository: "https://github.com/acme/tpl"}, want: true},
		{name: "ssh", meta: &pkgtemplate.Metadata{Repository: "git@github.com:acme/tpl"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.meta.IsRemote(); got != tt.want {
				t.Errorf("IsRemote() = %v, want %v", got, tt.want)
			}
		})
	}
}

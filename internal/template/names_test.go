package template_test

import (
	"os"
	"strings"
	"testing"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

var nameErrorCases = []struct {
	name   string
	source string // template-relative path of the file whose name fails
	want   string // template-relative path of the name that fails to render
}{
	{name: "parse error", source: "{{ .Name | nosuchfunc }}.txt", want: "{{ .Name | nosuchfunc }}.txt"},
	{name: "undefined variable", source: "{{ .Nmae }}.txt", want: "{{ .Nmae }}.txt"},
	{name: "execution error", source: "{{ index .Name 99 }}.txt", want: "{{ index .Name 99 }}.txt"},
	{name: "action split across directories", source: "{{ if .X }}docs/images{{ end }}/badge.png", want: "{{ if .X }}docs"},
}

func TestExecute_NameError_FailFast(t *testing.T) {
	for _, tc := range nameErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, _, target, err := executeInSandbox(t, "X: true\nName: acme\n", map[string][]byte{
				"{{ .Name }}.md": []byte("md"),
				tc.source:        []byte("x"),
			}, pkgtemplate.Config{})
			if err == nil {
				t.Fatal("Execute: expected an error, got nil")
			}

			if !strings.Contains(err.Error(), "template/"+tc.want+":") {
				t.Errorf("error should name template/%s: %v", tc.want, err)
			}

			if fileExists(t, target, "<no value>.txt") {
				t.Error("an undefined variable must not render as <no value>")
			}

			if len(tmpl.Warnings) != 0 {
				t.Errorf("Warnings = %v, want none in fail-fast mode", tmpl.Warnings)
			}
		})
	}
}

func TestExecute_NameError_ContinueOnError(t *testing.T) {
	for _, tc := range nameErrorCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, _, target, err := executeInSandbox(t, "X: true\nName: acme\n", map[string][]byte{
				"{{ .Name }}.md": []byte("md"),
				tc.source:        []byte("x"),
			}, pkgtemplate.Config{ContinueOnRenderError: true})
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}

			if got := readFile(t, target, "acme.md"); got != "md" {
				t.Errorf("acme.md = %q", got)
			}

			entries, err := os.ReadDir(target)
			if err != nil {
				t.Fatal(err)
			}

			if len(entries) != 1 {
				t.Errorf("only acme.md should be written, got %v", entries)
			}

			if len(tmpl.Warnings) != 1 {
				t.Fatalf("Warnings = %v, want one", tmpl.Warnings)
			}

			w := tmpl.Warnings[0]
			if w.Path != tc.want || !w.Skipped || w.Err == nil {
				t.Errorf("warning = %+v, want a skipped %q with an error", w, tc.want)
			}
		})
	}
}

func TestExecute_NameRenderingEmptyIsSkippedSilently(t *testing.T) {
	for _, cfg := range []pkgtemplate.Config{{}, {ContinueOnRenderError: true}} {
		tmpl, _, target, err := executeInSandbox(t, "X: false\n", map[string][]byte{
			"{{ if .X }}x.txt{{ end }}":                 []byte("x"),
			"{{ if .X }}docs{{ end }}/images/badge.png": []byte("png"),
			"{{ if .X }} {{ end }}/y.txt":               []byte("y"),
		}, cfg)
		if err != nil {
			t.Fatalf("Execute: %v", err)
		}

		entries, err := os.ReadDir(target)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != 0 {
			t.Errorf("output should be empty, has %v", entries)
		}

		if len(tmpl.Warnings) != 0 {
			t.Errorf("Warnings = %v, want none", tmpl.Warnings)
		}
	}
}

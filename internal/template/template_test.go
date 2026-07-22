package template_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

// buildTemplate creates a minimal template root in a temp dir.
// yaml is the content of project.yaml.
// files is a map of relative paths (within template/) to file contents.
func buildTemplate(t *testing.T, yaml string, files map[string][]byte) string {
	t.Helper()
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, "project.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("writing project.yaml: %v", err)
	}

	templateDir := filepath.Join(root, "template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatalf("creating template dir: %v", err)
	}

	for relPath, content := range files {
		abs := filepath.Join(templateDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatalf("creating dir for %s: %v", relPath, err)
		}

		if err := os.WriteFile(abs, content, 0644); err != nil {
			t.Fatalf("writing %s: %v", relPath, err)
		}
	}

	return root
}

// readFile reads the content of a file in the target dir; fails the test if absent.
func readFile(t *testing.T, targetDir, relPath string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(targetDir, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatalf("reading %s: %v", relPath, err)
	}

	return string(data)
}

// fileExists reports whether a file or directory exists at relPath inside targetDir.
func fileExists(t *testing.T, targetDir, relPath string) bool {
	t.Helper()

	_, err := os.Stat(filepath.Join(targetDir, filepath.FromSlash(relPath)))

	return err == nil
}

func TestExecute_StaticFile(t *testing.T) {
	root := buildTemplate(t, "Name: World\n", map[string][]byte{
		"hello.txt": []byte("Hello {{.Name}}"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := readFile(t, target, "hello.txt")
	if got != "Hello World" {
		t.Errorf("hello.txt = %q, want %q", got, "Hello World")
	}
}

func TestExecute_ConditionalFilename_True(t *testing.T) {
	root := buildTemplate(t, "UseX: true\n", map[string][]byte{
		"{{if .UseX}}feature.txt{{end}}": []byte("enabled"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !fileExists(t, target, "feature.txt") {
		t.Error("feature.txt should exist when UseX is true")
	}
}

func TestExecute_ConditionalFilename_False(t *testing.T) {
	root := buildTemplate(t, "UseX: false\n", map[string][]byte{
		"{{if .UseX}}feature.txt{{end}}": []byte("enabled"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fileExists(t, target, "feature.txt") {
		t.Error("feature.txt should not exist when UseX is false")
	}
}

func TestExecute_ConditionalDir_False(t *testing.T) {
	root := buildTemplate(t, "UseX: false\n", map[string][]byte{
		"{{if .UseX}}subdir{{end}}/file.txt": []byte("inside"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fileExists(t, target, "subdir") {
		t.Error("subdir should not exist when UseX is false")
	}
}

func TestExecute_VerbatimCopy(t *testing.T) {
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"composer.lock": []byte("[[not a template]]"), // [[ is not a delimiter here; file is verbatim
	})

	// Write .specsverbatim
	if err := os.WriteFile(filepath.Join(root, ".specsverbatim"), []byte("composer.lock\n"), 0644); err != nil {
		t.Fatalf("writing .specsverbatim: %v", err)
	}

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := readFile(t, target, "composer.lock")
	if got != "[[not a template]]" { // original content must be preserved verbatim
		t.Errorf("composer.lock = %q, want verbatim copy", got)
	}
}

func TestExecute_BinaryFile(t *testing.T) {
	// File with null byte — detected as binary and copied verbatim.
	content := []byte{0x00, 0x01, 0x02, 0x03}
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"image.bin": content,
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "image.bin"))
	if err != nil {
		t.Fatalf("reading image.bin: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("image.bin content mismatch (binary file not copied verbatim)")
	}
}

func TestExecute_WhitespaceOnly(t *testing.T) {
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"empty.txt": []byte("{{if false}}x{{end}}"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fileExists(t, target, "empty.txt") {
		t.Error("empty.txt should not exist when rendered content is whitespace-only")
	}
}

func TestExecute_NestedConditionalDir(t *testing.T) {
	root := buildTemplate(t, "X: false\n", map[string][]byte{
		"{{if .X}}subdir{{end}}/nested/file.txt": []byte("deep"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if fileExists(t, target, "subdir") {
		t.Error("subdir should not exist when X is false")
	}
}

func TestExecute_ComputedValueInTemplate(t *testing.T) {
	root := buildTemplate(t,
		"Name: acme\ncomputed:\n  DbName: \"{{toSnakeCase .Name}}_production\"\n",
		map[string][]byte{
			"config.txt": []byte("DB={{.DbName}}"),
		},
	)

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Caller is responsible for applying computed values before Execute.
	ctx, err := pkgtemplate.ApplyComputed(tmpl.Context, tmpl.ComputedDefs, tmpl.FuncMap(), tmpl.Delims())
	if err != nil {
		t.Fatalf("ApplyComputed: %v", err)
	}

	tmpl.Context = ctx

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := readFile(t, target, "config.txt")
	if got != "DB=acme_production" {
		t.Errorf("config.txt = %q, want %q", got, "DB=acme_production")
	}
}

func TestExecute_ComputedNotAppliedByExecute(t *testing.T) {
	// Execute does not call ApplyComputed internally; a missing computed key causes an error.
	root := buildTemplate(t,
		"Name: acme\ncomputed:\n  DbName: \"{{toSnakeCase .Name}}_production\"\n",
		map[string][]byte{
			"config.txt": []byte("DB={{.DbName}}"),
		},
	)

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Intentionally skip ApplyComputed — Execute must NOT apply it internally.
	target := t.TempDir()
	if err := tmpl.Execute(target); err == nil {
		t.Fatal("Execute: expected error for missing computed key DbName, got nil")
	}
}

func TestExecute_PassthroughDoubleBrace(t *testing.T) {
	// When __delimiters switches to [[ ]], the default {{ }} syntax passes through
	// unchanged — important for files like GitHub Actions YAML that already use {{ }}.
	yaml := "Name: ci\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	root := buildTemplate(t, yaml, map[string][]byte{
		"ci.yml": []byte("group: ${{ github.ref }}\nname: [[.Name]]"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := readFile(t, target, "ci.yml")
	want := "group: ${{ github.ref }}\nname: ci"

	if got != want {
		t.Errorf("ci.yml = %q, want %q", got, want)
	}
}

func TestExecute_CustomDelimiters_NotInContext(t *testing.T) {
	// __delimiters must not appear as a user-visible template variable.
	yaml := "Name: test\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	root := buildTemplate(t, yaml, map[string][]byte{
		"out.txt": []byte("[[.Name]]"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if _, ok := tmpl.Context["__delimiters"]; ok {
		t.Error("__delimiters should not appear in the template context")
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := readFile(t, target, "out.txt"); got != "test" {
		t.Errorf("out.txt = %q, want %q", got, "test")
	}
}

func TestExecute_ReservedSpecsVersionAvailable(t *testing.T) {
	// A recognised reserved config key is stripped from the schema context but its value
	// is still made available to templates under its original key during rendering.
	root := buildTemplate(t,
		"Name: demo\n__specs__version: \">=0.0.1\"\n",
		map[string][]byte{"out.txt": []byte("v={{ .__specs__version }}")},
	)

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if _, ok := tmpl.Context["__specs__version"]; ok {
		t.Error("__specs__version should not appear in the schema context")
	}

	if tmpl.Reserved["__specs__version"] != ">=0.0.1" {
		t.Errorf("Reserved[__specs__version] = %v, want %q", tmpl.Reserved["__specs__version"], ">=0.0.1")
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := readFile(t, target, "out.txt"); got != "v=>=0.0.1" {
		t.Errorf("out.txt = %q, want %q", got, "v=>=0.0.1")
	}
}

func TestExecute_ReservedDelimitersAvailable(t *testing.T) {
	// The __delimiters mapping is available to templates as nested fields, rendered with
	// the very delimiters it configures.
	yaml := "Name: demo\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	root := buildTemplate(t, yaml, map[string][]byte{
		"out.txt": []byte("d=[[ .__delimiters.left ]][[ .__delimiters.right ]]"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if got := readFile(t, target, "out.txt"); got != "d=[[]]" {
		t.Errorf("out.txt = %q, want %q", got, "d=[[]]")
	}
}

func TestGet_InvalidDelimiters_ReturnsError(t *testing.T) {
	// __delimiters present but malformed — Get must return an error.
	yaml := "__delimiters: not-a-mapping\nName: x\n"
	root := buildTemplate(t, yaml, map[string][]byte{"f.txt": []byte("{{.Name}}")})

	_, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err == nil {
		t.Fatal("expected error for invalid __delimiters, got nil")
	}
}

func TestExecute_ParseError_ContinueOnError(t *testing.T) {
	// With ContinueOnRenderError=true a parse error records a warning and copies verbatim.
	yaml := "Name: test\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	original := []byte("[[ .Name | undefinedFunc ]]")
	root := buildTemplate(t, yaml, map[string][]byte{
		"compose.yml": original,
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{ContinueOnRenderError: true})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	if len(tmpl.Warnings) == 0 {
		t.Fatal("expected at least one RenderWarning, got none")
	}

	w := tmpl.Warnings[0]
	if w.Path != "compose.yml" {
		t.Errorf("warning path = %q, want %q", w.Path, "compose.yml")
	}

	if w.Preview == "" {
		t.Error("expected Preview to be set on RenderWarning")
	}

	got := readFile(t, target, "compose.yml")
	if got != string(original) {
		t.Errorf("compose.yml = %q, want verbatim copy %q", got, string(original))
	}
}

func TestExecute_ParseError_FailFast(t *testing.T) {
	// Default (fail-fast): a parse error aborts Execute with a non-nil error.
	// No destination file must be written.
	yaml := "Name: test\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	root := buildTemplate(t, yaml, map[string][]byte{
		"compose.yml": []byte("[[ .Name | undefinedFunc ]]"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err == nil {
		t.Fatal("Execute: expected error for parse failure in fail-fast mode, got nil")
	}

	if fileExists(t, target, "compose.yml") {
		t.Error("compose.yml must not be written when Execute aborts due to a parse error")
	}

	if len(tmpl.Warnings) != 0 {
		t.Errorf("expected no Warnings in fail-fast mode, got %d", len(tmpl.Warnings))
	}
}

func TestExecute_ExecuteError_FailFast(t *testing.T) {
	// Default (fail-fast): referencing a missing context key aborts Execute.
	// missingkey=error is set, so .MissingKey causes an execution error.
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"out.txt": []byte("{{ .MissingKey }}"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err == nil {
		t.Fatal("Execute: expected error for missing key in fail-fast mode, got nil")
	}

	if fileExists(t, target, "out.txt") {
		t.Error("out.txt must not be written when Execute aborts due to an execution error")
	}
}

func TestExecute_ExecuteError_ContinueOnError(t *testing.T) {
	// With ContinueOnRenderError=true an execution error records a warning and copies verbatim.
	original := []byte("{{ .MissingKey }}")
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"out.txt": original,
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{ContinueOnRenderError: true})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}

	if len(tmpl.Warnings) == 0 {
		t.Fatal("expected at least one RenderWarning, got none")
	}

	w := tmpl.Warnings[0]
	if w.Path != "out.txt" {
		t.Errorf("warning path = %q, want %q", w.Path, "out.txt")
	}

	if w.Preview == "" {
		t.Error("expected Preview to be set on RenderWarning")
	}

	got := readFile(t, target, "out.txt")
	if got != string(original) {
		t.Errorf("out.txt = %q, want verbatim copy %q", got, string(original))
	}
}

func TestExecute_PreservesPermissions_TextFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "project.yaml"), []byte("Name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	templateDir := filepath.Join(root, "template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(templateDir, "run.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho {{.Name}}\n"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(scriptPath, 0755); err != nil {
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

	info, err := os.Stat(filepath.Join(target, "run.sh"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if got := info.Mode().Perm(); got != 0755 {
		t.Errorf("run.sh permissions = %04o, want 0755", got)
	}
}

func TestExecute_PreservesPermissions_BinaryFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "project.yaml"), []byte("Name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	templateDir := filepath.Join(root, "template")
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		t.Fatal(err)
	}

	binPath := filepath.Join(templateDir, "tool")
	// null byte makes it detected as binary
	if err := os.WriteFile(binPath, []byte{0x7f, 0x45, 0x4c, 0x46, 0x00}, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(binPath, 0755); err != nil {
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

	info, err := os.Stat(filepath.Join(target, "tool"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if got := info.Mode().Perm(); got != 0755 {
		t.Errorf("tool permissions = %04o, want 0755", got)
	}
}

// Binary detection tests — each case documents the expected behaviour of isBinary
// under the two-stage check: http.DetectContentType first, null/UTF-8 fallback second.

func TestExecute_BinaryDetection_JPEG(t *testing.T) {
	// JPEG magic bytes (FF D8 FF) — detected as image/jpeg by http.DetectContentType.
	// Content includes a template marker to prove the file is NOT rendered as a template.
	content := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, []byte("{{ .Name }}")...)
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"photo.jpg": content,
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "photo.jpg"))
	if err != nil {
		t.Fatalf("reading photo.jpg: %v", err)
	}

	if string(got) != string(content) {
		t.Error("photo.jpg not copied verbatim (JPEG magic bytes should be detected as binary)")
	}
}

func TestExecute_BinaryDetection_Gzip(t *testing.T) {
	// Gzip magic bytes (1F 8B) — 0x1F is a binary data byte per the WHATWG sniff
	// algorithm, so http.DetectContentType returns application/octet-stream.
	content := append([]byte{0x1F, 0x8B, 0x08, 0x00}, []byte("{{ .Name }}")...)
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"archive.tar.gz": content,
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "archive.tar.gz"))
	if err != nil {
		t.Fatalf("reading archive.tar.gz: %v", err)
	}

	if string(got) != string(content) {
		t.Error("archive.tar.gz not copied verbatim (gzip magic bytes should be detected as binary)")
	}
}

func TestExecute_BinaryDetection_UTF16BOM(t *testing.T) {
	// UTF-16 LE BOM (FF FE) — 0xFF is in the high-byte range, so
	// http.DetectContentType returns application/octet-stream.
	// Preserved as binary; template expansion is never attempted.
	content := []byte{0xFF, 0xFE, 0x41, 0x00, 0x42, 0x00, 0x43, 0x00} // BOM + "ABC" in UTF-16 LE
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"utf16.txt": content,
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "utf16.txt"))
	if err != nil {
		t.Fatalf("reading utf16.txt: %v", err)
	}

	if string(got) != string(content) {
		t.Error("utf16.txt not copied verbatim (UTF-16 BOM should be detected as binary)")
	}
}

func TestExecute_BinaryDetection_PDFHeader(t *testing.T) {
	// PDF header (%PDF-) — all bytes are valid ASCII/UTF-8 with no null bytes, so
	// the old null/UTF-8 heuristic alone would treat this as text. http.DetectContentType
	// recognises the magic bytes and returns application/pdf, which the primary check
	// catches as non-text → the file is copied verbatim.
	// The embedded {{ .Name }} must be preserved literally, not expanded.
	content := []byte("%PDF-1.4\n{{ .Name }}")
	root := buildTemplate(t, "Name: test\n", map[string][]byte{
		"document.pdf": content,
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "document.pdf"))
	if err != nil {
		t.Fatalf("reading document.pdf: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("document.pdf = %q, want verbatim %q (PDF header should be detected as binary even though bytes are valid UTF-8)", got, string(content))
	}
}

func TestGet_SpecsVersion_Satisfied(t *testing.T) {
	root := buildTemplate(t, "Name: World\n__specs__version: ^0.1.0\n", map[string][]byte{
		"hello.txt": []byte("Hello {{.Name}}"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{Version: "0.1.5"})
	if err != nil {
		t.Fatalf("Get: expected success when version satisfies constraint, got %v", err)
	}
	// Reserved key must not leak into the context.
	if _, ok := tmpl.Context["__specs__version"]; ok {
		t.Error("__specs__version should not appear in the template context")
	}
}

func TestGet_SpecsVersion_Unsatisfied(t *testing.T) {
	root := buildTemplate(t, "Name: World\n__specs__version: ^0.2.0\n", map[string][]byte{
		"hello.txt": []byte("Hello {{.Name}}"),
	})

	_, err := pkgtemplate.Get(root, pkgtemplate.Config{Version: "0.1.0"})
	if !errors.Is(err, specs.ErrSpecsVersionUnsatisfied) {
		t.Fatalf("Get: expected ErrSpecsVersionUnsatisfied, got %v", err)
	}
}

func TestGet_SpecsVersion_SkippedWhenDev(t *testing.T) {
	root := buildTemplate(t, "Name: World\n__specs__version: ^0.2.0\n", map[string][]byte{
		"hello.txt": []byte("Hello {{.Name}}"),
	})

	if _, err := pkgtemplate.Get(root, pkgtemplate.Config{Version: "dev"}); err != nil {
		t.Fatalf("Get: expected check to be skipped for dev version, got %v", err)
	}
}

func TestGet_SpecsVersion_SkippedWhenEmpty(t *testing.T) {
	root := buildTemplate(t, "Name: World\n__specs__version: ^0.2.0\n", map[string][]byte{
		"hello.txt": []byte("Hello {{.Name}}"),
	})

	if _, err := pkgtemplate.Get(root, pkgtemplate.Config{}); err != nil {
		t.Fatalf("Get: expected check to be skipped for empty version, got %v", err)
	}
}

func TestGet_SpecsVersion_Malformed(t *testing.T) {
	root := buildTemplate(t, "Name: World\n__specs__version: \"not a version\"\n", map[string][]byte{
		"hello.txt": []byte("Hello {{.Name}}"),
	})

	_, err := pkgtemplate.Get(root, pkgtemplate.Config{Version: "0.1.0"})
	if !errors.Is(err, specs.ErrInvalidSpecsVersion) {
		t.Fatalf("Get: expected ErrInvalidSpecsVersion, got %v", err)
	}
}

func TestGet_CustomDelimiters_ConditionalFilename(t *testing.T) {
	// Custom delimiters affect filename templates too.
	yaml := "UseX: true\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	root := buildTemplate(t, yaml, map[string][]byte{
		"[[if .UseX]]feature.txt[[end]]": []byte("enabled"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !fileExists(t, target, "feature.txt") {
		t.Error("feature.txt should exist when UseX is true and custom delimiters are set")
	}
}

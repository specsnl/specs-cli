package template_test

import (
	"os"
	"path/filepath"
	"testing"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	target := t.TempDir()
	if err := tmpl.Execute(target); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := readFile(t, target, "config.txt")
	if got != "DB=acme_production" {
		t.Errorf("config.txt = %q, want %q", got, "DB=acme_production")
	}
}

func TestExecute_PassthroughDoubleBrace(t *testing.T) {
	// When __delimiters switches to [[ ]], the default {{ }} syntax passes through
	// unchanged — important for files like GitHub Actions YAML that already use {{ }}.
	yaml := "Name: ci\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	root := buildTemplate(t, yaml, map[string][]byte{
		"ci.yml": []byte("group: ${{ github.ref }}\nname: [[.Name]]"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

func TestGet_InvalidDelimiters_ReturnsError(t *testing.T) {
	// __delimiters present but malformed — Get must return an error.
	yaml := "__delimiters: not-a-mapping\nName: x\n"
	root := buildTemplate(t, yaml, map[string][]byte{"f.txt": []byte("{{.Name}}")})

	_, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
	if err == nil {
		t.Fatal("expected error for invalid __delimiters, got nil")
	}
}

func TestExecute_ParseError_CopiesVerbatimAndWarns(t *testing.T) {
	// A template file that calls an undefined function triggers a parse error.
	// Execute must succeed (verbatim copy), and the warning must be recorded on tmpl.Warnings.
	yaml := "Name: test\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	original := []byte("[[ .Name | undefinedFunc ]]")
	root := buildTemplate(t, yaml, map[string][]byte{
		"compose.yml": original,
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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
	if tmpl.Warnings[0].Path != "compose.yml" {
		t.Errorf("warning path = %q, want %q", tmpl.Warnings[0].Path, "compose.yml")
	}

	got := readFile(t, target, "compose.yml")
	if got != string(original) {
		t.Errorf("compose.yml = %q, want verbatim copy %q", got, string(original))
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

func TestGet_CustomDelimiters_ConditionalFilename(t *testing.T) {
	// Custom delimiters affect filename templates too.
	yaml := "UseX: true\n__delimiters:\n  left: \"[[\"\n  right: \"]]\"\n"
	root := buildTemplate(t, yaml, map[string][]byte{
		"[[if .UseX]]feature.txt[[end]]": []byte("enabled"),
	})

	tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{}, discardLogger())
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

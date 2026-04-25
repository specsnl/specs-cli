package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// makeTemplateWithVar creates a template directory with a single string variable
// and a rendered file that uses it.
func makeTemplateWithVar(t *testing.T, varName, defaultVal string) string {
	t.Helper()
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	project := varName + ": " + defaultVal + "\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}
	content := "hello [[." + varName + "]]"
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// saveAndUse is a helper that saves src under name and runs template use with extra args.
func saveAndUse(t *testing.T, src, name, target string, extraArgs ...string) error {
	t.Helper()
	if _, err := executeCmd("template", "save", src, name); err != nil {
		t.Fatalf("template save: %v", err)
	}
	args := append([]string{"template", "use"}, extraArgs...)
	args = append(args, name, target)
	_, err := executeCmd(args...)
	return err
}

func TestTemplateUse_UseDefaults(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "world")
	target := t.TempDir()
	if err := saveAndUse(t, src, "tpl", target, "--use-defaults"); err != nil {
		t.Fatalf("template use: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("got %q, want %q", string(got), "hello world")
	}
}

func TestTemplateUse_ArgOverride(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "default")
	target := t.TempDir()
	if err := saveAndUse(t, src, "tpl", target, "--use-defaults", "--arg", "Name=test"); err != nil {
		t.Fatalf("template use: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "hello test" {
		t.Errorf("got %q, want %q", string(got), "hello test")
	}
}

func TestTemplateUse_ValuesFile(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "default")

	vf := filepath.Join(t.TempDir(), "vals.json")
	data, _ := json.Marshal(map[string]string{"Name": "from-file"})
	if err := os.WriteFile(vf, data, 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, src, "tpl", target, "--use-defaults", "--values", vf); err != nil {
		t.Fatalf("template use: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "hello from-file" {
		t.Errorf("got %q, want %q", string(got), "hello from-file")
	}
}

func TestTemplateUse_ArgBeatsValues(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "default")

	vf := filepath.Join(t.TempDir(), "vals.json")
	data, _ := json.Marshal(map[string]string{"Name": "file-value"})
	if err := os.WriteFile(vf, data, 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, src, "tpl", target, "--use-defaults", "--values", vf, "--arg", "Name=arg-value"); err != nil {
		t.Fatalf("template use: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "hello arg-value" {
		t.Errorf("got %q, want %q", string(got), "hello arg-value")
	}
}

func TestTemplateUse_NotFound(t *testing.T) {
	withTempRegistry(t)
	_, err := executeCmd("template", "use", "--use-defaults", "no-such-name", t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
	if !errors.Is(err, specs.ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestTemplateUse_NoHooks(t *testing.T) {
	withTempRegistry(t)

	dir := t.TempDir()
	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Sentinel written by the post-use hook to confirm it ran.
	sentinel := filepath.Join(t.TempDir(), "hook-ran")
	project := "Name: x\nhooks:\n  post-use:\n    - touch " + sentinel + "\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, dir, "tpl", target, "--use-defaults", "--no-hooks"); err != nil {
		t.Fatalf("template use: %v", err)
	}
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("post-use hook ran despite --no-hooks")
	}
}

func TestTemplateUse_ComputedAvailable(t *testing.T) {
	withTempRegistry(t)

	dir := t.TempDir()
	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	project := "Name: hello\ncomputed:\n  Upper: \"[[ toUpper .Name ]]\"\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("[[.Upper]]"), 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, dir, "tpl", target, "--use-defaults"); err != nil {
		t.Fatalf("template use: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "HELLO" {
		t.Errorf("got %q, want %q", string(got), "HELLO")
	}
}

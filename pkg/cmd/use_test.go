package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// buildMinimalTemplate creates a valid template root in dir with the given project.yaml
// content and a single template file.
func buildMinimalTemplate(t *testing.T, dir, yamlContent, filename, fileContent string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	tplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tplDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, filename), []byte(fileContent), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestUse_LocalPath(t *testing.T) {
	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Name: world\n", "hello.txt", "Hello [[.Name]]")
	targetDir := t.TempDir()

	_, err := executeCmd("use", "--use-defaults", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(targetDir, "hello.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "Hello world" {
		t.Errorf("got %q, want %q", string(got), "Hello world")
	}
}

func TestUse_RelativePath(t *testing.T) {
	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Name: relative\n", "out.txt", "[[.Name]]")
	targetDir := t.TempDir()

	_, err := executeCmd("use", "--use-defaults", srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(targetDir, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "relative" {
		t.Errorf("got %q, want %q", string(got), "relative")
	}
}

func TestUse_UseDefaults(t *testing.T) {
	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Name: default-val\n", "out.txt", "[[.Name]]")
	targetDir := t.TempDir()

	_, err := executeCmd("use", "--use-defaults", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(targetDir, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "default-val" {
		t.Errorf("got %q, want %q", string(got), "default-val")
	}
}

func TestUse_ArgOverride(t *testing.T) {
	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Name: original\n", "out.txt", "[[.Name]]")
	targetDir := t.TempDir()

	_, err := executeCmd("use", "--use-defaults", "--arg", "Name=test", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(targetDir, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "test" {
		t.Errorf("got %q, want %q", string(got), "test")
	}
}

func TestUse_ValuesFile(t *testing.T) {
	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Name: original\n", "out.txt", "[[.Name]]")

	vf := filepath.Join(t.TempDir(), "vals.json")
	data, _ := json.Marshal(map[string]string{"Name": "from-file"})
	if err := os.WriteFile(vf, data, 0644); err != nil {
		t.Fatal(err)
	}

	targetDir := t.TempDir()
	_, err := executeCmd("use", "--use-defaults", "--values", vf, "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(targetDir, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if string(got) != "from-file" {
		t.Errorf("got %q, want %q", string(got), "from-file")
	}
}

func TestUse_InvalidSource(t *testing.T) {
	_, err := executeCmd("use", "--use-defaults", "not-a-valid-source", t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestUse_TempDirCleanedUp(t *testing.T) {
	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Name: x\n", "out.txt", "[[.Name]]")
	targetDir := t.TempDir()

	// Capture temp dir count before.
	before, _ := filepath.Glob(os.TempDir() + "/specs-use-src-*")

	_, err := executeCmd("use", "--use-defaults", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v", err)
	}

	// All specs-use-src-* dirs created during this run should be gone.
	after, _ := filepath.Glob(os.TempDir() + "/specs-use-src-*")
	if len(after) > len(before) {
		t.Errorf("temp source dir(s) not cleaned up: %v", after)
	}
}

func TestUse_TempDirCleanedOnError(t *testing.T) {
	srcDir := t.TempDir()
	// Invalid project.yaml — missing template/ dir causes a parse/get error.
	if err := os.WriteFile(filepath.Join(srcDir, specs.ProjectYAMLFile), []byte("Name: x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	before, _ := filepath.Glob(os.TempDir() + "/specs-use-src-*")

	_, _ = executeCmd("use", "--use-defaults", "file:"+srcDir, t.TempDir())

	after, _ := filepath.Glob(os.TempDir() + "/specs-use-src-*")
	if len(after) > len(before) {
		t.Errorf("temp source dir(s) not cleaned up after error: %v", after)
	}
}

func TestUse_NoRegistryEntry(t *testing.T) {
	withTempRegistry(t)

	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Name: world\n", "hello.txt", "Hello [[.Name]]")
	targetDir := t.TempDir()

	_, err := executeCmd("use", "--use-defaults", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v", err)
	}

	// TemplateDir may not exist at all, or may exist but be empty — either is correct.
	entries, _ := os.ReadDir(specs.TemplateDir())
	if len(entries) != 0 {
		t.Errorf("registry should be empty after specs use, got %d entries", len(entries))
	}
}

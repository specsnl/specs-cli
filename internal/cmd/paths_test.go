package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
	"github.com/specsnl/specs-cli/internal/util/exit"
)

const escapeWarning = "template/{{ .Dir }} → ../../escaped: rendered path is outside the output directory"

func buildEscapingTemplate(t *testing.T, dir string) {
	t.Helper()

	buildMinimalTemplate(t, dir, "Dir: ../../escaped\n", "keep.txt", "keep")

	nested := filepath.Join(dir, specs.TemplateDirFile, "{{ .Dir }}")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(nested, "x.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

// sandboxTempDir points the render temp dir two levels below a fresh sandbox
// and returns where a path climbing out of it would land.
func sandboxTempDir(t *testing.T) (escaped string) {
	t.Helper()

	sandbox := t.TempDir()
	tmp := filepath.Join(sandbox, "a", "b", "tmp")

	if err := os.MkdirAll(tmp, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TMPDIR", tmp)

	return filepath.Join(sandbox, "a", "escaped")
}

func assertNotEscaped(t *testing.T, escaped string) {
	t.Helper()

	if _, err := os.Stat(escaped); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a path was written outside the output: %v", err)
	}
}

func TestUse_LocalEscapingPathIsSkipped(t *testing.T) {
	escaped := sandboxTempDir(t)

	srcDir := t.TempDir()
	buildEscapingTemplate(t, srcDir)

	targetDir := t.TempDir()

	out, err := executeCmd("use", "--use-defaults", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v\n%s", err, out)
	}

	assertNotEscaped(t, escaped)

	if _, err := os.Stat(filepath.Join(targetDir, "keep.txt")); err != nil {
		t.Errorf("keep.txt should be written: %v", err)
	}

	if !strings.Contains(out, escapeWarning) {
		t.Errorf("output should warn about the skipped path:\n%s", out)
	}
}

func TestUse_ArgThatEscapesIsSkipped(t *testing.T) {
	escaped := sandboxTempDir(t)

	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Dir: inside\n", "{{ .Dir }}.txt", "x")

	targetDir := t.TempDir()

	out, err := executeCmd("use", "--use-defaults", "--arg", "Dir=../../escaped", "file:"+srcDir, targetDir)
	if err != nil {
		t.Fatalf("use: %v\n%s", err, out)
	}

	assertNotEscaped(t, escaped+".txt")

	if !strings.Contains(out, "template/{{ .Dir }}.txt → ../../escaped.txt") {
		t.Errorf("output should warn about the skipped path:\n%s", out)
	}
}

func TestUse_AbsoluteArgIsSkippedWithAWarning(t *testing.T) {
	sandboxTempDir(t)

	abs := filepath.Join(t.TempDir(), "abs")

	srcDir := t.TempDir()
	buildMinimalTemplate(t, srcDir, "Dir: src\n", "keep.txt", "keep")

	nested := filepath.Join(srcDir, specs.TemplateDirFile, "{{ .Dir }}")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(nested, "x.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := executeCmd("use", "--use-defaults", "--arg", "Dir="+abs, "file:"+srcDir, t.TempDir())
	if err != nil {
		t.Fatalf("use: %v\n%s", err, out)
	}

	assertNotEscaped(t, abs)

	if !strings.Contains(out, "template/{{ .Dir }} → "+abs+": rendered path is outside the output directory") {
		t.Errorf("output should warn about the skipped path:\n%s", out)
	}
}

func TestTemplateUse_SavedEscapingPathIsSkipped(t *testing.T) {
	withTempRegistry(t)

	escaped := sandboxTempDir(t)

	srcDir := t.TempDir()
	buildEscapingTemplate(t, srcDir)

	if out, err := executeCmd("template", "save", srcDir, "escaping"); err != nil {
		t.Fatalf("save: %v\n%s", err, out)
	}

	out, err := executeCmd("template", "use", "--use-defaults", "escaping", t.TempDir())
	if err != nil {
		t.Fatalf("template use: %v\n%s", err, out)
	}

	assertNotEscaped(t, escaped)

	if !strings.Contains(out, escapeWarning) {
		t.Errorf("output should warn about the skipped path:\n%s", out)
	}
}

func TestUse_RemoteEscapingPathAborts(t *testing.T) {
	escaped := sandboxTempDir(t)

	url := serveGitRepo(t, func(dir string) { buildEscapingTemplate(t, dir) })

	targetDir := filepath.Join(t.TempDir(), "out")

	_, err := executeCmd("use", "--use-defaults", url, targetDir)
	if !errors.Is(err, specs.ErrPathOutside) {
		t.Fatalf("use error = %v, want ErrPathOutside", err)
	}

	if !strings.Contains(err.Error(), escapeWarning) {
		t.Errorf("error should name the path: %v", err)
	}

	assertNotEscaped(t, escaped)

	if _, err := os.Stat(targetDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("nothing should be written to the target: %v", err)
	}
}

func TestTemplateUse_DownloadedEscapingPathAborts(t *testing.T) {
	withTempRegistry(t)

	escaped := sandboxTempDir(t)

	url := serveGitRepo(t, func(dir string) { buildEscapingTemplate(t, dir) })

	if out, err := executeCmd("template", "download", url, "escaping"); err != nil {
		t.Fatalf("download: %v\n%s", err, out)
	}

	targetDir := filepath.Join(t.TempDir(), "out")

	_, err := executeCmd("template", "use", "--use-defaults", "escaping", targetDir)
	if !errors.Is(err, specs.ErrPathOutside) {
		t.Fatalf("template use error = %v, want ErrPathOutside", err)
	}

	assertNotEscaped(t, escaped)

	if _, err := os.Stat(targetDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("nothing should be written to the target: %v", err)
	}
}

func TestTemplateValidate_ReportsEscapingPaths(t *testing.T) {
	escaped := sandboxTempDir(t)

	dir := t.TempDir()
	buildEscapingTemplate(t, dir)

	out, err := executeCmd("template", "validate", dir)

	if code := validateExitCode(err); code != exit.ValidatePaths {
		t.Fatalf("exit code = %d, want %d (err=%v)\n%s", code, exit.ValidatePaths, err, out)
	}

	assertNotEscaped(t, escaped)

	if !strings.Contains(out, escapeWarning) {
		t.Errorf("output should report the escaping path:\n%s", out)
	}
}

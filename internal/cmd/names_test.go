package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/util/exit"
)

const brokenName = "{{ .Name | nosuchfunc }}.txt"

func buildBrokenNameTemplate(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	buildMinimalTemplate(t, dir, "Name: acme\n", brokenName, "x")

	return dir
}

func TestUse_NameError_FailsFast(t *testing.T) {
	targetDir := filepath.Join(t.TempDir(), "out")

	_, err := executeCmd("use", "--use-defaults", "file:"+buildBrokenNameTemplate(t), targetDir)
	if err == nil {
		t.Fatal("expected an error for a name that fails to render, got nil")
	}

	if !strings.Contains(err.Error(), "template/"+brokenName) {
		t.Errorf("error should name the source path: %v", err)
	}

	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("nothing should be written to the target: %v", err)
	}
}

func TestUse_NameError_ContinueOnErrorSkipsWithAWarning(t *testing.T) {
	targetDir := t.TempDir()

	out, err := executeCmd("use", "--use-defaults", "--continue-on-error", "file:"+buildBrokenNameTemplate(t), targetDir)
	if err != nil {
		t.Fatalf("use --continue-on-error: %v\n%s", err, out)
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Errorf("the broken name should not be written, got %v", entries)
	}

	if !strings.Contains(out, "template/"+brokenName+": skipped due to render error in its name") {
		t.Errorf("output should warn about the skipped path:\n%s", out)
	}
}

func TestTemplateValidate_ReportsNameErrors(t *testing.T) {
	out, err := executeCmd("template", "validate", buildBrokenNameTemplate(t))

	if code := validateExitCode(err); code != exit.ValidateRender {
		t.Fatalf("exit code = %d, want %d (err=%v)\n%s", code, exit.ValidateRender, err, out)
	}

	if !strings.Contains(out, "render error in the name of template/"+brokenName) {
		t.Errorf("output should report the name error:\n%s", out)
	}
}

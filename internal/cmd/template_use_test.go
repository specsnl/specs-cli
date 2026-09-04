package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"

	"github.com/specsnl/specs-cli/internal/specs"
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

	content := "hello {{." + varName + "}}"
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

func TestTemplateUse_ReservedValueRenderedIntoOutput(t *testing.T) {
	// Reserved config values are stripped from the schema but still available to templates.
	withTempRegistry(t)

	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	project := "Name: demo\n__specs__version: \">=0.0.1\"\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("v={{ .__specs__version }}"), 0644); err != nil {
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

	if string(got) != "v=>=0.0.1" {
		t.Errorf("got %q, want %q", string(got), "v=>=0.0.1")
	}
}

func TestTemplateUse_ReservedArgRejected(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "default")
	target := t.TempDir()

	err := saveAndUse(t, src, "tpl", target, "--use-defaults", "--arg", "__foo=bar")
	if !errors.Is(err, specs.ErrReservedVariableName) {
		t.Fatalf("expected ErrReservedVariableName, got %v", err)
	}
}

func TestTemplateUse_ReservedValuesFileRejected(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "default")

	vf := filepath.Join(t.TempDir(), "vals.json")
	data, _ := json.Marshal(map[string]string{"__foo": "bar"})

	if err := os.WriteFile(vf, data, 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()

	err := saveAndUse(t, src, "tpl", target, "--use-defaults", "--values", vf)
	if !errors.Is(err, specs.ErrReservedVariableName) {
		t.Fatalf("expected ErrReservedVariableName, got %v", err)
	}
}

func TestTemplateUse_DelimitersArgAllowed(t *testing.T) {
	// __delimiters is a recognised configuration key, so it is not rejected as a
	// reserved variable when passed via --arg (though as an arg it is inert).
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "default")
	target := t.TempDir()

	if err := saveAndUse(t, src, "tpl", target, "--use-defaults", "--arg", "__delimiters=x"); err != nil {
		t.Fatalf("template use: %v", err)
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

func TestTemplateUse_ConditionalSkipped(t *testing.T) {
	// UseDB=false → DbName should be skipped; output file must have no DB line.
	withTempRegistry(t)

	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	project := "UseDB: false\nDbName: mydb\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("{{if .UseDB}}DB={{.DbName}}{{end}}"), 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, dir, "tpl", target, "--use-defaults"); err != nil {
		t.Fatalf("template use: %v", err)
	}
	// The output file should be empty/absent because UseDB=false renders whitespace-only.
	if _, err := os.Stat(filepath.Join(target, "out.txt")); err == nil {
		content, _ := os.ReadFile(filepath.Join(target, "out.txt"))
		if string(content) != "" {
			t.Errorf("out.txt should be absent or empty, got: %q", content)
		}
	}
}

func TestTemplateUse_ConditionalIncluded(t *testing.T) {
	// UseDB=true → DbName should be included via --arg override.
	withTempRegistry(t)

	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	project := "UseDB: false\nDbName: mydb\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("{{if .UseDB}}DB={{.DbName}}{{end}}"), 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, dir, "tpl", target, "--use-defaults", "--arg", "UseDB=true"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("out.txt missing: %v", err)
	}

	if string(got) != "DB=mydb" {
		t.Errorf("got %q, want %q", string(got), "DB=mydb")
	}
}

func TestTemplateUse_ConditionalArgOverride(t *testing.T) {
	// --arg UseDB=true with --use-defaults: output contains default DbName.
	withTempRegistry(t)

	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	project := "UseDB: false\nDbName: defaultdb\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("{{if .UseDB}}DB={{.DbName}}{{end}}"), 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, dir, "tpl", target, "--use-defaults", "--arg", "UseDB=true"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("out.txt missing: %v", err)
	}

	if string(got) != "DB=defaultdb" {
		t.Errorf("got %q, want %q", string(got), "DB=defaultdb")
	}
}

// makeConditionalTemplate builds a template with a boolean gate and a nested eq gate.
// Schema: UseDB bool, DbType string (default "pg"), PgPort string, MyPort string.
// Template: {{if .UseDB}}{{if eq .DbType "pg"}}pg={{.PgPort}}{{else}}my={{.MyPort}}{{end}}{{end}}
func makeConditionalTemplate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	project := "UseDB: false\nDbType: \"pg\"\nPgPort: \"5432\"\nMyPort: \"3306\"\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	content := `{{if .UseDB}}{{if eq .DbType "pg"}}pg={{.PgPort}}{{else}}my={{.MyPort}}{{end}}{{end}}`
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestTemplateUse_NestedEq_InnerSkippedWhenOuterGateChanges(t *testing.T) {
	// UseDB=true, DbType=mysql → PgPort condition is false (skipped); MyPort is used.
	// All values provided via --arg so promptContext runs but huh never fires.
	// This exercises the iterative conditional loop: PgPort is only evaluated after
	// DbType is resolved, so its false condition is correctly detected.
	withTempRegistry(t)
	dir := makeConditionalTemplate(t)
	target := t.TempDir()

	if err := saveAndUse(t, dir, "tpl", target,
		"--arg", "UseDB=true", "--arg", "DbType=mysql", "--arg", "MyPort=3306",
	); err != nil {
		t.Fatalf("template use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("out.txt missing: %v", err)
	}

	if string(got) != "my=3306" {
		t.Errorf("got %q, want %q", string(got), "my=3306")
	}
}

func TestTemplateUse_NestedEq_InnerIncludedWhenConditionMet(t *testing.T) {
	// UseDB=true, DbType=pg → PgPort condition is true (included); MyPort is skipped.
	// All values provided via --arg so promptContext runs but huh never fires.
	withTempRegistry(t)
	dir := makeConditionalTemplate(t)
	target := t.TempDir()

	if err := saveAndUse(t, dir, "tpl", target,
		"--arg", "UseDB=true", "--arg", "DbType=pg", "--arg", "PgPort=5432",
	); err != nil {
		t.Fatalf("template use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("out.txt missing: %v", err)
	}

	if string(got) != "pg=5432" {
		t.Errorf("got %q, want %q", string(got), "pg=5432")
	}
}

func TestTemplateUse_UnreferencedVarNotRequired(t *testing.T) {
	// Schema has "Unused" but the template never references it.
	// Name is provided via --arg so promptContext runs but huh never fires —
	// this exercises the referenced filter: Unused is stripped before runPromptPass.
	withTempRegistry(t)
	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	project := "Name: world\nUnused: \"\"\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("hello {{.Name}}"), 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, dir, "tpl", target, "--arg", "Name=world"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("out.txt missing: %v", err)
	}

	if string(got) != "hello world" {
		t.Errorf("got %q, want %q", string(got), "hello world")
	}
}

func TestTemplateUse_ComputedOnlyVar_IsUsed(t *testing.T) {
	// Name is only referenced inside a computed expression, not directly in template files.
	// Name is provided via --arg so promptContext runs — this exercises the path where
	// a variable lands in Referenced via computed-expression scanning, not template scanning.
	withTempRegistry(t)
	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	project := "Name: acme\ncomputed:\n  DbName: \"{{.Name}}_db\"\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("db={{.DbName}}"), 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, dir, "tpl", target, "--arg", "Name=acme"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("out.txt missing: %v", err)
	}

	if string(got) != "db=acme_db" {
		t.Errorf("got %q, want %q", string(got), "db=acme_db")
	}
}

func makeTemplateWithSelectVar(t *testing.T, varName string, options []string) string {
	t.Helper()
	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	var project strings.Builder

	project.WriteString(varName + ":\n")

	for _, opt := range options {
		project.WriteString("  - " + opt + "\n")
	}

	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project.String()), 0644); err != nil {
		t.Fatal(err)
	}

	content := "selected {{." + varName + "}}"
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestTemplateUse_UseDefaults_SelectFirstItem(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithSelectVar(t, "foobar", []string{"one", "two", "three"})
	target := t.TempDir()

	if err := saveAndUse(t, src, "tpl", target, "--use-defaults"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	if string(got) != "selected one" {
		t.Errorf("got %q, want %q", string(got), "selected one")
	}
}

func TestTemplateUse_UseDefaults_SelectArgOverride(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithSelectVar(t, "foobar", []string{"one", "two", "three"})
	target := t.TempDir()

	if err := saveAndUse(t, src, "tpl", target, "--use-defaults", "--arg", "foobar=two"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	if string(got) != "selected two" {
		t.Errorf("got %q, want %q", string(got), "selected two")
	}
}

func TestTemplateUse_ComputedAvailable(t *testing.T) {
	withTempRegistry(t)

	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	project := "Name: hello\ncomputed:\n  Upper: \"{{ toUpper .Name }}\"\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("{{.Upper}}"), 0644); err != nil {
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

func TestTemplateUse_ProjectYMLFile(t *testing.T) {
	withTempRegistry(t)
	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYMLFile), []byte("Name: from-yml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("{{.Name}}"), 0644); err != nil {
		t.Fatal(err)
	}

	target := t.TempDir()
	if err := saveAndUse(t, dir, "tpl", target, "--use-defaults"); err != nil {
		t.Fatalf("template use with project.yml: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}

	if string(got) != "from-yml" {
		t.Errorf("got %q, want %q", string(got), "from-yml")
	}
}

func TestTemplateUse_SafeMode_SkipsHooks(t *testing.T) {
	withTempRegistry(t)

	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	sentinel := filepath.Join(t.TempDir(), "hook-ran")

	project := "Name: x\nhooks:\n  post-use:\n    - touch " + sentinel + "\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmplDir, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "save", dir, "tpl"); err != nil {
		t.Fatalf("template save: %v", err)
	}

	if _, err := executeCmd("--safe-mode", "template", "use", "--use-defaults", "tpl", t.TempDir()); err != nil {
		t.Fatalf("template use --safe-mode: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("post-use hook ran despite --safe-mode")
	}
}

func TestExecuteTemplate_RemoteHooks_RunsWithYes(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(t.TempDir(), "hook-ran")
	project := "Name: x\nhooks:\n  post-use:\n    - touch " + sentinel + "\n"
	buildMinimalTemplate(t, dir, project, "f.txt", "x")

	app := NewApp()
	target := t.TempDir()

	err := app.executeTemplate(dir, target, executeOpts{
		useDefaults: true,
		remote:      true,
		yes:         true,
	})
	if err != nil {
		t.Fatalf("executeTemplate: %v", err)
	}

	if _, err := os.Stat(sentinel); err != nil {
		t.Error("post-use hook did not run with remote=true, yes=true")
	}
}

// Safe mode skips hooks before the remote confirmation is ever reached.
func TestExecuteTemplate_RemoteHooks_SafeMode(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(t.TempDir(), "hook-ran")
	project := "Name: x\nhooks:\n  post-use:\n    - touch " + sentinel + "\n"
	buildMinimalTemplate(t, dir, project, "f.txt", "x")

	app := NewApp()
	app.SafeMode = true

	err := app.executeTemplate(dir, t.TempDir(), executeOpts{
		useDefaults: true,
		remote:      true,
	})
	if err != nil {
		t.Fatalf("executeTemplate: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("hook ran despite safe-mode on remote source")
	}
}

func TestExecuteTemplate_SafeMode_AllowHooks(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(t.TempDir(), "hook-ran")
	project := "Name: x\nhooks:\n  post-use:\n    - touch " + sentinel + "\n"
	buildMinimalTemplate(t, dir, project, "f.txt", "x")

	app := NewApp()
	app.SafeMode = true

	err := app.executeTemplate(dir, t.TempDir(), executeOpts{
		useDefaults: true,
		allowHooks:  true,
	})
	if err != nil {
		t.Fatalf("executeTemplate: %v", err)
	}

	if _, err := os.Stat(sentinel); err != nil {
		t.Error("hook did not run despite --allow-hooks overriding --safe-mode")
	}
}

func TestTemplateUse_AmbiguousProjectFiles(t *testing.T) {
	withTempRegistry(t)
	dir := t.TempDir()

	tmplDir := filepath.Join(dir, specs.TemplateDirFile)
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte("Name: yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYMLFile), []byte("Name: yml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeCmd("template", "save", dir, "tpl"); err != nil {
		t.Fatalf("template save: %v", err)
	}

	_, err := executeCmd("template", "use", "--use-defaults", "tpl", t.TempDir())
	if err == nil {
		t.Fatal("expected error when both project.yaml and project.yml exist, got nil")
	}
}

// A run that still needs a value fails immediately, naming the variable, rather
// than blocking on a form nobody can answer.
func TestTemplateUse_NoTerminal_NamesTheMissingVariables(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "world")

	err := saveAndUse(t, src, "tpl", t.TempDir())
	if err == nil {
		t.Fatal("expected an error when a value is missing and nothing can be prompted")
	}

	if !errors.Is(err, specs.ErrCannotPrompt) {
		t.Fatalf("error = %v, want it to wrap ErrCannotPrompt", err)
	}

	for _, want := range []string{"stdin is not a terminal", "missing values for: Name", "--use-defaults"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// --non-interactive says the same thing for a different reason, so a developer
// at a terminal can see what CI will do.
func TestTemplateUse_NonInteractiveFlag_RefusesToPrompt(t *testing.T) {
	withTempRegistry(t)
	src := makeTemplateWithVar(t, "Name", "world")

	err := saveAndUse(t, src, "tpl", t.TempDir(), "--non-interactive")
	if err == nil {
		t.Fatal("expected an error with --non-interactive and a missing value")
	}

	if !strings.Contains(err.Error(), "--non-interactive is set") {
		t.Errorf("error = %q, want it to name the flag as the reason", err)
	}
}

// Supplying every value never reaches a prompt, so the guard above cannot fire.
func TestTemplateUse_ValuesSupplied_NeverPrompts(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"--arg", []string{"--arg", "Name=supplied"}, "hello supplied"},
		{"--use-defaults", []string{"--use-defaults"}, "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTempRegistry(t)

			src := makeTemplateWithVar(t, "Name", "world")
			target := t.TempDir()

			if err := saveAndUse(t, src, "tpl", target, tt.args...); err != nil {
				t.Fatalf("template use: %v", err)
			}

			got, err := os.ReadFile(filepath.Join(target, "out.txt"))
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != tt.want {
				t.Errorf("rendered %q, want %q", got, tt.want)
			}
		})
	}
}

// A remote template's hook confirmation cannot be asked for either, but the
// answer is to decline rather than to fail: the template still applies, only
// its hooks are skipped.
func TestExecuteTemplate_RemoteHooks_NoTerminal_SkipsHooks(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(t.TempDir(), "hook-ran")
	project := "Name: x\nhooks:\n  post-use:\n    - touch " + sentinel + "\n"
	buildMinimalTemplate(t, dir, project, "f.txt", "x")

	app := NewApp()

	if err := app.executeTemplate(dir, t.TempDir(), executeOpts{useDefaults: true, remote: true}); err != nil {
		t.Fatalf("executeTemplate: %v", err)
	}

	if _, err := os.Stat(sentinel); err == nil {
		t.Error("post-use hook ran without a confirmation anyone could give")
	}
}

// The form is drawn on stderr, never on stdout: stdout carries the product, and
// a prompt's redraws would corrupt `-o json` and end up inside the file a
// caller redirected to.
func TestPrompter_DrawsOnStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := prompter{
		// A ctrl-c ends the form immediately, which is all this needs: the
		// question is where the drawing went, not what was answered.
		stdin:   strings.NewReader("\x03"),
		stderr:  &stderr,
		allowed: true,
	}

	var proceed bool

	done := make(chan error, 1)
	go func() {
		done <- p.run(huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Allow hook execution?").Value(&proceed),
		)))
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the form did not return; it is reading from something other than the given stdin")
	}

	if stderr.Len() == 0 {
		t.Error("nothing was drawn on stderr")
	}

	if stdout.Len() != 0 {
		t.Errorf("drew %q on stdout", stdout.String())
	}
}

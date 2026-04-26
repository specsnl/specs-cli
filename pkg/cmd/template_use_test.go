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
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("[[if .UseDB]]DB=[[.DbName]][[end]]"), 0644); err != nil {
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
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("[[if .UseDB]]DB=[[.DbName]][[end]]"), 0644); err != nil {
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
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("[[if .UseDB]]DB=[[.DbName]][[end]]"), 0644); err != nil {
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
// Template: [[if .UseDB]][[if eq .DbType "pg"]]pg=[[.PgPort]][[else]]my=[[.MyPort]][[end]][[end]]
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
	content := `[[if .UseDB]][[if eq .DbType "pg"]]pg=[[.PgPort]][[else]]my=[[.MyPort]][[end]][[end]]`
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
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("hello [[.Name]]"), 0644); err != nil {
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
	project := "Name: acme\ncomputed:\n  DbName: \"[[.Name]]_db\"\n"
	if err := os.WriteFile(filepath.Join(dir, specs.ProjectYAMLFile), []byte(project), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "out.txt"), []byte("db=[[.DbName]]"), 0644); err != nil {
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

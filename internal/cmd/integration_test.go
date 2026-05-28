//go:build integration

// Package cmd integration tests drive the cobra command tree against static
// fixtures in internal/testdata/. No network access is required; all sources
// are local paths. Run with: go test -tags=integration ./internal/cmd/...
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
)

// fixtureDir returns the absolute path to a named fixture inside internal/testdata/.
// Go sets the test working directory to the package source dir (internal/cmd/), so
// "../testdata/<name>" resolves to "internal/testdata/<name>".
func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatalf("fixtureDir: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture %q not found at %s: %v", name, p, err)
	}
	return p
}

// --- specs use (one-shot, no registry) ---

func TestIntegration_Use_Minimal(t *testing.T) {
	src := fixtureDir(t, "minimal")
	target := t.TempDir()

	if _, err := executeCmd("use", "--use-defaults", "file:"+src, target); err != nil {
		t.Fatalf("specs use: %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatalf("README.md missing: %v", err)
	}
	if !strings.Contains(string(readme), "my-project") {
		t.Errorf("README.md: want 'my-project', got %q", string(readme))
	}
	if !strings.Contains(string(readme), "John Doe") {
		t.Errorf("README.md: want 'John Doe', got %q", string(readme))
	}

	config, err := os.ReadFile(filepath.Join(target, "config.txt"))
	if err != nil {
		t.Fatalf("config.txt missing: %v", err)
	}
	if !strings.Contains(string(config), "my-project") {
		t.Errorf("config.txt: want 'my-project', got %q", string(config))
	}
}

func TestIntegration_Use_Conditional_DatabaseFalse(t *testing.T) {
	src := fixtureDir(t, "conditional")
	target := t.TempDir()

	// Default: UseDatabase=false → conditional file and directory are skipped.
	if _, err := executeCmd("use", "--use-defaults", "file:"+src, target); err != nil {
		t.Fatalf("specs use: %v", err)
	}

	if _, err := os.ReadFile(filepath.Join(target, "README.md")); err != nil {
		t.Fatalf("README.md should always be present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "database.env")); err == nil {
		t.Error("database.env should not exist when UseDatabase=false")
	}
	if _, err := os.Stat(filepath.Join(target, "db", "schema.sql")); err == nil {
		t.Error("db/schema.sql should not exist when UseDatabase=false")
	}
}

func TestIntegration_Use_Conditional_DatabaseTrue(t *testing.T) {
	src := fixtureDir(t, "conditional")
	target := t.TempDir()

	// Override: UseDatabase=true → conditional file and directory are created.
	if _, err := executeCmd("use", "--use-defaults", "--arg", "UseDatabase=true", "file:"+src, target); err != nil {
		t.Fatalf("specs use: %v", err)
	}

	dbEnv, err := os.ReadFile(filepath.Join(target, "database.env"))
	if err != nil {
		t.Fatalf("database.env missing when UseDatabase=true: %v", err)
	}
	if !strings.Contains(string(dbEnv), "mydb") {
		t.Errorf("database.env: want 'mydb', got %q", string(dbEnv))
	}
	if _, err := os.Stat(filepath.Join(target, "db", "schema.sql")); err != nil {
		t.Errorf("db/schema.sql should exist when UseDatabase=true: %v", err)
	}
}

func TestIntegration_Use_Computed(t *testing.T) {
	src := fixtureDir(t, "computed")
	target := t.TempDir()

	// ProjectShortName=acme → ProjectSlug=acme → DbName=acme_production → DbTestName=acme_test
	if _, err := executeCmd("use", "--use-defaults", "file:"+src, target); err != nil {
		t.Fatalf("specs use: %v", err)
	}

	dbEnv, err := os.ReadFile(filepath.Join(target, "db.env"))
	if err != nil {
		t.Fatalf("db.env missing: %v", err)
	}
	content := string(dbEnv)
	for _, want := range []string{"acme_production", "acme_test", "acme", "my-project"} {
		if !strings.Contains(content, want) {
			t.Errorf("db.env: want %q in output, got %q", want, content)
		}
	}
}

func TestIntegration_Use_Hooks_Inline(t *testing.T) {
	src := fixtureDir(t, "hooks")
	target := t.TempDir()

	// post-use hook: echo "{{ .ProjectName }}" > hook-output.txt (runs in target dir)
	if _, err := executeCmd("use", "--use-defaults", "file:"+src, target); err != nil {
		t.Fatalf("specs use: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(target, "hook-output.txt"))
	if err != nil {
		t.Fatalf("hook-output.txt missing — post-use hook did not run: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "my-project" {
		t.Errorf("hook-output.txt = %q, want %q", got, "my-project")
	}
}

func TestIntegration_Use_HooksFile(t *testing.T) {
	src := fixtureDir(t, "hooks-file")
	target := t.TempDir()

	// post-use.sh: echo "$SPECS_PROJECTNAME" > hook-output.txt (runs in target dir)
	if _, err := executeCmd("use", "--use-defaults", "file:"+src, target); err != nil {
		t.Fatalf("specs use: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(target, "hook-output.txt"))
	if err != nil {
		t.Fatalf("hook-output.txt missing — file-based post-use hook did not run: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "my-project" {
		t.Errorf("hook-output.txt = %q, want %q", got, "my-project")
	}
}

func TestIntegration_Use_Verbatim(t *testing.T) {
	src := fixtureDir(t, "verbatim")
	target := t.TempDir()

	if _, err := executeCmd("use", "--use-defaults", "file:"+src, target); err != nil {
		t.Fatalf("specs use: %v", err)
	}

	// normal.txt is rendered: {{ .ProjectName }} → my-project, no delimiters remain.
	normal, err := os.ReadFile(filepath.Join(target, "normal.txt"))
	if err != nil {
		t.Fatalf("normal.txt missing: %v", err)
	}
	if strings.Contains(string(normal), "{{") {
		t.Errorf("normal.txt should be rendered, still contains delimiters: %q", string(normal))
	}
	if !strings.Contains(string(normal), "my-project") {
		t.Errorf("normal.txt: want 'my-project', got %q", string(normal))
	}

	// literal.raw matches *.raw in .specsverbatim — copied verbatim, delimiters intact.
	raw, err := os.ReadFile(filepath.Join(target, "literal.raw"))
	if err != nil {
		t.Fatalf("literal.raw missing: %v", err)
	}
	if !strings.Contains(string(raw), "{{") {
		t.Errorf("literal.raw should be verbatim (contain delimiters), got %q", string(raw))
	}

	// vendor/config.js matches vendor/** in .specsverbatim — copied verbatim.
	vendor, err := os.ReadFile(filepath.Join(target, "vendor", "config.js"))
	if err != nil {
		t.Fatalf("vendor/config.js missing: %v", err)
	}
	if !strings.Contains(string(vendor), "{{") {
		t.Errorf("vendor/config.js should be verbatim (contain delimiters), got %q", string(vendor))
	}

	// icon.bin is a binary file — copied verbatim regardless of .specsverbatim.
	if _, err := os.Stat(filepath.Join(target, "icon.bin")); err != nil {
		t.Errorf("icon.bin missing — binary file should be copied verbatim: %v", err)
	}
}

func TestIntegration_Use_Delimiters(t *testing.T) {
	src := fixtureDir(t, "delimiters")
	target := t.TempDir()

	// Custom delimiters [[ ]]: [[ .ProjectName ]] is rendered, {{ }} passes through.
	if _, err := executeCmd("use", "--use-defaults", "file:"+src, target); err != nil {
		t.Fatalf("specs use: %v", err)
	}

	config, err := os.ReadFile(filepath.Join(target, "config.yaml"))
	if err != nil {
		t.Fatalf("config.yaml missing: %v", err)
	}
	content := string(config)

	if strings.Contains(content, "[[") {
		t.Errorf("config.yaml: [[ ]] delimiters should have been rendered, still present: %q", content)
	}
	if !strings.Contains(content, "my-project") {
		t.Errorf("config.yaml: want 'my-project', got %q", content)
	}
	// ${{ github.repository }} is not valid [[ ]] syntax, so it passes through unchanged.
	if !strings.Contains(content, "${{ github.repository }}") {
		t.Errorf("config.yaml: literal ${{ github.repository }} should be preserved, got %q", content)
	}
}

// --- Registry-level: template save + template use ---

func TestIntegration_TemplateUse_Minimal(t *testing.T) {
	withTempRegistry(t)
	src := fixtureDir(t, "minimal")
	target := t.TempDir()

	if err := saveAndUse(t, src, "minimal", target, "--use-defaults"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	readme, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatalf("README.md missing: %v", err)
	}
	if !strings.Contains(string(readme), "my-project") {
		t.Errorf("README.md: want 'my-project', got %q", string(readme))
	}
}

func TestIntegration_TemplateUse_Computed(t *testing.T) {
	withTempRegistry(t)
	src := fixtureDir(t, "computed")
	target := t.TempDir()

	if err := saveAndUse(t, src, "computed", target, "--use-defaults"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	dbEnv, err := os.ReadFile(filepath.Join(target, "db.env"))
	if err != nil {
		t.Fatalf("db.env missing: %v", err)
	}
	if !strings.Contains(string(dbEnv), "acme_production") {
		t.Errorf("db.env: want 'acme_production', got %q", string(dbEnv))
	}
	if !strings.Contains(string(dbEnv), "acme_test") {
		t.Errorf("db.env: want 'acme_test', got %q", string(dbEnv))
	}
}

func TestIntegration_TemplateUse_Conditional_ArgOverride(t *testing.T) {
	withTempRegistry(t)
	src := fixtureDir(t, "conditional")
	target := t.TempDir()

	if err := saveAndUse(t, src, "conditional", target, "--use-defaults", "--arg", "UseDatabase=true"); err != nil {
		t.Fatalf("template use: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, "database.env")); err != nil {
		t.Errorf("database.env should exist when UseDatabase=true: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "db", "schema.sql")); err != nil {
		t.Errorf("db/schema.sql should exist when UseDatabase=true: %v", err)
	}
}

// --- template list ---

func TestIntegration_TemplateList_WithFixtures(t *testing.T) {
	withTempRegistry(t)

	names := []string{"minimal", "computed", "delimiters"}
	for _, name := range names {
		src := fixtureDir(t, name)
		if _, err := executeCmd("template", "save", src, name); err != nil {
			t.Fatalf("template save %s: %v", name, err)
		}
	}

	out, err := executeCmd("template", "list")
	if err != nil {
		t.Fatalf("template list: %v", err)
	}
	for _, name := range names {
		if !strings.Contains(out, name) {
			t.Errorf("template list: want %q in output, got %q", name, out)
		}
	}
}

// --- template validate ---

func TestIntegration_TemplateValidate_Minimal(t *testing.T) {
	src := fixtureDir(t, "minimal")
	out, err := executeCmd("template", "validate", src)
	if err != nil {
		t.Fatalf("template validate minimal: %v", err)
	}
	if !strings.Contains(out, "template is valid") {
		t.Errorf("expected 'template is valid', got %q", out)
	}
}

func TestIntegration_TemplateValidate_Computed(t *testing.T) {
	src := fixtureDir(t, "computed")
	_, err := executeCmd("template", "validate", src)
	if err != nil {
		t.Fatalf("template validate computed: %v", err)
	}
}

func TestIntegration_TemplateValidate_Conditional(t *testing.T) {
	src := fixtureDir(t, "conditional")
	_, err := executeCmd("template", "validate", src)
	if err != nil {
		t.Fatalf("template validate conditional: %v", err)
	}
}

func TestIntegration_TemplateValidate_Verbatim(t *testing.T) {
	src := fixtureDir(t, "verbatim")
	_, err := executeCmd("template", "validate", src)
	if err != nil {
		t.Fatalf("template validate verbatim: %v", err)
	}
}

func TestIntegration_TemplateValidate_Delimiters(t *testing.T) {
	src := fixtureDir(t, "delimiters")
	_, err := executeCmd("template", "validate", src)
	if err != nil {
		t.Fatalf("template validate delimiters: %v", err)
	}
}

// --- template delete ---

func TestIntegration_TemplateDelete(t *testing.T) {
	withTempRegistry(t)
	src := fixtureDir(t, "minimal")

	if _, err := executeCmd("template", "save", src, "to-delete"); err != nil {
		t.Fatalf("template save: %v", err)
	}
	if _, err := executeCmd("template", "delete", "to-delete"); err != nil {
		t.Fatalf("template delete: %v", err)
	}
	if _, err := os.Stat(specs.TemplatePath("to-delete")); !os.IsNotExist(err) {
		t.Error("template should be absent after delete")
	}
}

// --- template rename ---

func TestIntegration_TemplateRename(t *testing.T) {
	withTempRegistry(t)
	src := fixtureDir(t, "minimal")

	if _, err := executeCmd("template", "save", src, "old-name"); err != nil {
		t.Fatalf("template save: %v", err)
	}
	if _, err := executeCmd("template", "rename", "old-name", "new-name"); err != nil {
		t.Fatalf("template rename: %v", err)
	}

	target := t.TempDir()
	if _, err := executeCmd("template", "use", "--use-defaults", "new-name", target); err != nil {
		t.Fatalf("template use after rename: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(target, "README.md")); err != nil {
		t.Fatalf("README.md missing after rename: %v", err)
	}
}

// --- template upgrade (local — skipped without network) ---

func TestIntegration_TemplateUpgrade_Local(t *testing.T) {
	withTempRegistry(t)
	src := fixtureDir(t, "minimal")

	if _, err := executeCmd("template", "save", src, "local-minimal"); err != nil {
		t.Fatalf("template save: %v", err)
	}
	// Local templates are skipped on upgrade — must succeed without network access.
	if _, err := executeCmd("template", "upgrade", "local-minimal"); err != nil {
		t.Fatalf("template upgrade local: %v", err)
	}
}

// --- template download (local path rejected) ---

func TestIntegration_TemplateDownload_RejectsLocalPath(t *testing.T) {
	withTempRegistry(t)
	src := fixtureDir(t, "minimal")

	_, err := executeCmd("template", "download", src, "should-fail")
	if err == nil {
		t.Fatal("expected error when downloading a local path, got nil")
	}
}

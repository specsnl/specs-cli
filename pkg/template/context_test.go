package template_test

import (
	"os"
	"path/filepath"
	"testing"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
)

func writeProjectYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "project.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("writeProjectYAML: %v", err)
	}
}

func writeProjectJSON(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "project.json"), []byte(content), 0644); err != nil {
		t.Fatalf("writeProjectJSON: %v", err)
	}
}

func TestLoadUserContext_String(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, "Name: my-project\n")

	ctx, _, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx["Name"] != "my-project" {
		t.Errorf("ctx[Name] = %q, want %q", ctx["Name"], "my-project")
	}
}

func TestLoadUserContext_Bool(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, "UseSonar: false\n")

	ctx, _, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx["UseSonar"] != false {
		t.Errorf("ctx[UseSonar] = %v, want false", ctx["UseSonar"])
	}
}

func TestLoadUserContext_Select(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, `
License:
  - MIT
  - GPL
`)

	ctx, _, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	list, ok := ctx["License"].([]any)
	if !ok {
		t.Fatalf("ctx[License] is %T, want []any", ctx["License"])
	}
	if len(list) != 2 || list[0] != "MIT" || list[1] != "GPL" {
		t.Errorf("ctx[License] = %v, want [MIT GPL]", list)
	}
}

func TestLoadUserContext_JSONFallback(t *testing.T) {
	dir := t.TempDir()
	writeProjectJSON(t, dir, `{"Name": "from-json"}`)

	ctx, _, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx["Name"] != "from-json" {
		t.Errorf("ctx[Name] = %q, want %q", ctx["Name"], "from-json")
	}
}

func TestLoadUserContext_ReferencedDefault(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, `
Name: My App
Slug: "[[toKebabCase .Name]]"
`)

	ctx, _, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx["Slug"] != "my-app" {
		t.Errorf("ctx[Slug] = %q, want %q", ctx["Slug"], "my-app")
	}
}

func TestLoadUserContext_CyclicReference(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, `
A: "[[.B]]"
B: "[[.A]]"
`)

	_, _, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err == nil {
		t.Fatal("expected error for cyclic reference, got nil")
	}
}

func TestLoadUserContext_HooksStripped(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, `
Name: test
hooks:
  post-use:
    - echo hi
`)

	ctx, _, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ctx["hooks"]; ok {
		t.Error("hooks key should be stripped from user context")
	}
}

func TestLoadUserContext_ComputedStripped(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, `
Name: test
computed:
  Env: prod
`)

	ctx, computedDefs, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := ctx["computed"]; ok {
		t.Error("computed key should be stripped from user context")
	}
	if _, ok := ctx["Env"]; ok {
		t.Error("Env computed key should not be in user context")
	}
	if computedDefs["Env"] != "prod" {
		t.Errorf("computedDefs[Env] = %q, want %q", computedDefs["Env"], "prod")
	}
}

func TestLoadUserContext_ComputedConflict(t *testing.T) {
	dir := t.TempDir()
	// writeProjectYAML(t, dir, "Name: foo\ncomputed:\n  Name: bar\n")
	writeProjectYAML(t, dir, `
Name: foo
computed:
  Name: bar
`)

	_, _, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err == nil {
		t.Fatal("expected error for computed key conflict, got nil")
	}
}

func TestApplyComputed_Simple(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, `Name: acme
computed:
  Env: "[[toUpper .Name]]"
`)

	ctx, defs, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("LoadUserContext: %v", err)
	}

	result, err := pkgtemplate.ApplyComputed(ctx, defs, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("ApplyComputed: %v", err)
	}
	if result["Env"] != "ACME" {
		t.Errorf("result[Env] = %q, want %q", result["Env"], "ACME")
	}
	// Original user input still present.
	if result["Name"] != "acme" {
		t.Errorf("result[Name] = %q, want %q", result["Name"], "acme")
	}
}

func TestApplyComputed_Chain(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, `Name: acme
computed:
  Slug: "[[toKebabCase .Name]]"
  DbName: "[[.Slug]]_production"
`)

	ctx, defs, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("LoadUserContext: %v", err)
	}

	result, err := pkgtemplate.ApplyComputed(ctx, defs, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("ApplyComputed: %v", err)
	}
	if result["Slug"] != "acme" {
		t.Errorf("result[Slug] = %q, want %q", result["Slug"], "acme")
	}
	if result["DbName"] != "acme_production" {
		t.Errorf("result[DbName] = %q, want %q", result["DbName"], "acme_production")
	}
}

func TestApplyComputed_Cycle(t *testing.T) {
	dir := t.TempDir()
	writeProjectYAML(t, dir, `
Name: x
computed:
  A: "[[.B]]"
  B: "[[.A]]"
`)

	ctx, defs, err := pkgtemplate.LoadUserContext(dir, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("LoadUserContext: %v", err)
	}

	_, err = pkgtemplate.ApplyComputed(ctx, defs, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err == nil {
		t.Fatal("expected error for computed cycle, got nil")
	}
}

func TestApplyComputed_NoDefs(t *testing.T) {
	ctx := map[string]any{"Name": "test"}
	result, err := pkgtemplate.ApplyComputed(ctx, nil, pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["Name"] != "test" {
		t.Errorf("result[Name] = %q, want %q", result["Name"], "test")
	}
}

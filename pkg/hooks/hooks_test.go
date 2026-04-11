package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// loadNoPrefix calls Load with an empty env prefix, for tests that don't care about prefixing.
func loadNoPrefix(templateRoot string, projectConfig map[string]any) (*Hooks, error) {
	return Load(templateRoot, projectConfig, "")
}

// --- Load: inline hooks ---

func TestLoad_InlinePreUse(t *testing.T) {
	h, err := loadNoPrefix(t.TempDir(), map[string]any{
		"hooks": map[string]any{
			"pre-use": []any{"echo hi"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PreUse) != 1 || h.PreUse[0] != "echo hi" {
		t.Errorf("PreUse = %v, want [echo hi]", h.PreUse)
	}
}

func TestLoad_InlinePostUse(t *testing.T) {
	h, err := loadNoPrefix(t.TempDir(), map[string]any{
		"hooks": map[string]any{
			"post-use": []any{"npm install"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PostUse) != 1 || h.PostUse[0] != "npm install" {
		t.Errorf("PostUse = %v, want [npm install]", h.PostUse)
	}
}

func TestLoad_InlineBothTriggers(t *testing.T) {
	h, err := loadNoPrefix(t.TempDir(), map[string]any{
		"hooks": map[string]any{
			"pre-use":  []any{"echo pre"},
			"post-use": []any{"echo post1", "echo post2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PreUse) != 1 || h.PreUse[0] != "echo pre" {
		t.Errorf("PreUse = %v", h.PreUse)
	}
	if len(h.PostUse) != 2 {
		t.Errorf("PostUse = %v, want 2 items", h.PostUse)
	}
}

func TestLoad_NoHooks(t *testing.T) {
	h, err := loadNoPrefix(t.TempDir(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if h == nil {
		t.Fatal("expected non-nil *Hooks")
	}
	if len(h.PreUse) != 0 || len(h.PostUse) != 0 {
		t.Errorf("expected empty hooks, got %+v", h)
	}
}

func TestLoad_EmptyHooks(t *testing.T) {
	h, err := loadNoPrefix(t.TempDir(), map[string]any{
		"hooks": map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PreUse) != 0 || len(h.PostUse) != 0 {
		t.Errorf("expected empty hooks, got %+v", h)
	}
}

// --- Load: directory hooks ---

func TestLoad_DirPreUse(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	if err := os.Mkdir(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-use.sh"), []byte("echo from file"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := loadNoPrefix(dir, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PreUse) != 1 || h.PreUse[0] != "echo from file" {
		t.Errorf("PreUse = %v", h.PreUse)
	}
}

func TestLoad_DirPostUse(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, "hooks")
	if err := os.Mkdir(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "post-use.sh"), []byte("npm install"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := loadNoPrefix(dir, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.PostUse) != 1 || h.PostUse[0] != "npm install" {
		t.Errorf("PostUse = %v", h.PostUse)
	}
}

func TestLoad_DirMissingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}

	h, err := loadNoPrefix(dir, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if h.PreUse != nil {
		t.Errorf("expected nil PreUse, got %v", h.PreUse)
	}
}

// --- Load: conflict ---

func TestLoad_BothSources(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := loadNoPrefix(dir, map[string]any{
		"hooks": map[string]any{
			"pre-use": []any{"echo hi"},
		},
	})
	if !errors.Is(err, specs.ErrBothHookSources) {
		t.Errorf("expected ErrBothHookSources, got %v", err)
	}
}

// --- Run ---

var emptyFuncMap = template.FuncMap{}

func TestRun_ExecutesCommand(t *testing.T) {
	h := &Hooks{PostUse: []string{"echo ok"}}
	if err := h.Run("post-use", t.TempDir(), map[string]any{}, emptyFuncMap); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRun_NonZeroExitReturnsError(t *testing.T) {
	h := &Hooks{PostUse: []string{"exit 1"}}
	if err := h.Run("post-use", t.TempDir(), map[string]any{}, emptyFuncMap); err == nil {
		t.Error("expected error for exit 1, got nil")
	}
}

func TestRun_StopsOnFirstFailure(t *testing.T) {
	sentinel := t.TempDir() + "/sentinel"
	h := &Hooks{PostUse: []string{"exit 1", "touch " + sentinel}}
	_ = h.Run("post-use", t.TempDir(), map[string]any{}, emptyFuncMap)
	if _, err := os.Stat(sentinel); err == nil {
		t.Error("second command ran after first failure")
	}
}

func TestRun_InjectsEnvVarsWithPrefix(t *testing.T) {
	h := &Hooks{PostUse: []string{`test "$SPECS_PROJECTNAME" = acme`}, EnvPrefix: specs.HookEnvPrefix}
	ctx := map[string]any{"ProjectName": "acme"}
	if err := h.Run("post-use", t.TempDir(), ctx, emptyFuncMap); err != nil {
		t.Errorf("env var not injected: %v", err)
	}
}

func TestRun_InjectsEnvVarsNoPrefix(t *testing.T) {
	h := &Hooks{PostUse: []string{`test "$PROJECTNAME" = acme`}, EnvPrefix: ""}
	ctx := map[string]any{"ProjectName": "acme"}
	if err := h.Run("post-use", t.TempDir(), ctx, emptyFuncMap); err != nil {
		t.Errorf("env var not injected without prefix: %v", err)
	}
}

func TestRun_RendersTemplateInCommand(t *testing.T) {
	h := &Hooks{PostUse: []string{`test "[[.Name]]" = world`}}
	ctx := map[string]any{"Name": "world"}
	if err := h.Run("post-use", t.TempDir(), ctx, emptyFuncMap); err != nil {
		t.Errorf("template not rendered: %v", err)
	}
}

func TestRun_UnknownTrigger(t *testing.T) {
	h := &Hooks{}
	if err := h.Run("invalid", t.TempDir(), map[string]any{}, emptyFuncMap); err == nil {
		t.Error("expected error for unknown trigger, got nil")
	}
}

func TestRun_EmptyHooks(t *testing.T) {
	h := &Hooks{}
	if err := h.Run("post-use", t.TempDir(), map[string]any{}, emptyFuncMap); err != nil {
		t.Errorf("unexpected error on empty hooks: %v", err)
	}
}

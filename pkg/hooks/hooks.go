package hooks

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// Hooks holds the pre-use and post-use command lists.
type Hooks struct {
	PreUse    []string // each entry: a single command or multiline bash script
	PostUse   []string
	EnvPrefix string // prefix for context keys injected as env vars (e.g. "SPECS_")
}

// Load reads hook definitions from templateRoot.
// Sources:
//   - Inline: the "hooks" key in projectConfig (parsed from project.yaml)
//   - Directory: hooks/pre-use.sh and hooks/post-use.sh under templateRoot
//
// envPrefix is prepended to each context key when injecting env vars (e.g. "SPECS_").
// Returns an error if both sources are present.
// Returns an empty *Hooks (not nil) if no hooks are defined at all.
func Load(templateRoot string, projectConfig map[string]any, envPrefix string) (*Hooks, error) {
	hasInline := false
	h := Hooks{EnvPrefix: envPrefix}

	if raw, ok := projectConfig["hooks"]; ok {
		if err := h.parseInline(raw); err != nil {
			return nil, fmt.Errorf("parsing inline hooks: %w", err)
		}
		hasInline = true
	}

	hooksDir := filepath.Join(templateRoot, "hooks")
	hasDir := dirExists(hooksDir)

	if hasInline && hasDir {
		return nil, specs.ErrBothHookSources
	}

	if hasDir {
		if err := h.loadFromDir(hooksDir); err != nil {
			return nil, err
		}
	}

	return &h, nil
}

// Run executes all commands for the given trigger ("pre-use" or "post-use").
// cwd is the working directory for the subprocess.
// ctx values are injected as UPPER_SNAKE_CASE env vars.
// [[ ]] template expressions in each command are rendered against ctx before execution.
// Returns immediately on the first non-zero exit.
func (h *Hooks) Run(trigger, cwd string, ctx map[string]any, funcMap template.FuncMap) error {
	var commands []string
	switch trigger {
	case "pre-use":
		commands = h.PreUse
	case "post-use":
		commands = h.PostUse
	default:
		return fmt.Errorf("unknown hook trigger: %q", trigger)
	}

	env := buildEnv(ctx, h.EnvPrefix)

	for _, cmdTpl := range commands {
		rendered, err := renderCommand(cmdTpl, ctx, funcMap)
		if err != nil {
			return fmt.Errorf("rendering hook command: %w", err)
		}

		slog.Default().Debug("running hook", "trigger", trigger, "command", firstLine(rendered))

		cmd := exec.Command("bash", "-c", rendered)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(), env...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s hook failed: %w", trigger, err)
		}
	}
	return nil
}

// HasPreUse reports whether any pre-use hooks are defined.
func (h *Hooks) HasPreUse() bool { return len(h.PreUse) > 0 }

// HasPostUse reports whether any post-use hooks are defined.
func (h *Hooks) HasPostUse() bool { return len(h.PostUse) > 0 }

// parseInline decodes the raw "hooks" value from project.yaml into h.PreUse / h.PostUse.
func (h *Hooks) parseInline(raw any) error {
	m, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("hooks must be a mapping, got %T", raw)
	}
	if pre, ok := m["pre-use"]; ok {
		cmds, err := toStringSlice(pre)
		if err != nil {
			return fmt.Errorf("pre-use: %w", err)
		}
		h.PreUse = cmds
	}
	if post, ok := m["post-use"]; ok {
		cmds, err := toStringSlice(post)
		if err != nil {
			return fmt.Errorf("post-use: %w", err)
		}
		h.PostUse = cmds
	}
	return nil
}

// loadFromDir reads hooks/pre-use.sh and hooks/post-use.sh if they exist.
// Each file's entire content becomes a single command entry.
func (h *Hooks) loadFromDir(hooksDir string) error {
	for _, name := range []struct {
		file   string
		target *[]string
	}{
		{"pre-use.sh", &h.PreUse},
		{"post-use.sh", &h.PostUse},
	} {
		path := filepath.Join(hooksDir, name.file)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		*name.target = []string{string(data)}
	}
	return nil
}

// buildEnv converts a context map to a slice of "PREFIX+KEY=value" strings.
// Keys are uppercased and prefixed with prefix; non-string values are formatted with fmt.Sprintf.
func buildEnv(ctx map[string]any, prefix string) []string {
	env := make([]string, 0, len(ctx))
	for k, v := range ctx {
		env = append(env, fmt.Sprintf("%s%s=%v", prefix, strings.ToUpper(k), v))
	}
	return env
}

// renderCommand renders [[ ]] template expressions in a hook command string.
func renderCommand(cmdTpl string, ctx map[string]any, funcMap template.FuncMap) (string, error) {
	if !strings.Contains(cmdTpl, "[[") {
		return cmdTpl, nil // fast path: no template expressions
	}
	tmpl, err := template.New("").Delims("[[", "]]").Funcs(funcMap).Parse(cmdTpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// toStringSlice coerces a []any (from YAML or JSON unmarshalling) into []string.
func toStringSlice(v any) ([]string, error) {
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected a list, got %T", v)
	}
	result := make([]string, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("item %d is not a string: %T", i, item)
		}
		result[i] = s
	}
	return result, nil
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// firstLine returns the first line of a string, for log messages.
func firstLine(s string) string {
	if firstLine, _, found := strings.Cut(s, "\n"); found {
		return firstLine + " …"
	}
	return s
}

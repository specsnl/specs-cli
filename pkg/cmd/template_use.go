package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/specsnl/specs-cli/pkg/hooks"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	"github.com/specsnl/specs-cli/pkg/util/osutil"
	"github.com/specsnl/specs-cli/pkg/util/output"
	"github.com/specsnl/specs-cli/pkg/util/validate"
	"github.com/specsnl/specs-cli/pkg/util/values"
)

type executeOpts struct {
	valuesFile  string
	argPairs    []string
	useDefaults bool
	noHooks     bool
}

func newTemplateUseCmd(app *App) *cobra.Command {
	var opts executeOpts

	cmd := &cobra.Command{
		Use:   "use <name> <target-dir>",
		Short: "Execute a registered template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, targetDir := args[0], args[1]

			if err := validate.Name(name); err != nil {
				return err
			}

			templateRoot := specs.TemplatePath(name)
			if _, err := os.Stat(templateRoot); os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, name)
			}

			return app.executeTemplate(templateRoot, targetDir, opts)
		},
	}

	cmd.Flags().StringVar(&opts.valuesFile, "values", "", "JSON file of pre-filled values")
	cmd.Flags().StringArrayVar(&opts.argPairs, "arg", nil, "Key=Value pair (repeatable)")
	cmd.Flags().BoolVar(&opts.useDefaults, "use-defaults", false, "Skip prompts; use schema defaults")
	cmd.Flags().BoolVar(&opts.noHooks, "no-hooks", false, "Skip pre/post-use hooks")

	return cmd
}

// executeTemplate is the shared execution logic reused by specs template use (Phase 7)
// and specs use (Phase 8).
func (a *App) executeTemplate(templateRoot, targetDir string, opts executeOpts) error {
	tmpl, err := a.templateGet(templateRoot)
	if err != nil {
		return err
	}

	rawConfig, err := loadRawConfig(templateRoot)
	if err != nil {
		return err
	}
	h, err := hooks.Load(templateRoot, rawConfig, a.HookEnvPrefix)
	if err != nil {
		return err
	}

	ctx := tmpl.Context
	provided := make(map[string]bool)

	if opts.valuesFile != "" {
		fileVals, err := values.LoadFile(opts.valuesFile)
		if err != nil {
			return err
		}
		for k := range fileVals {
			provided[k] = true
		}
		ctx = values.Merge(ctx, fileVals)
	}

	for _, pair := range opts.argPairs {
		k, v, err := values.ParseArg(pair)
		if err != nil {
			return err
		}
		ctx[k] = v
		provided[k] = true
	}

	if !opts.useDefaults {
		if err := promptContext(ctx, tmpl.Context, provided); err != nil {
			return err
		}
	}

	ctx, err = pkgtemplate.ApplyComputed(ctx, tmpl.ComputedDefs, tmpl.FuncMap())
	if err != nil {
		return err
	}

	if !opts.noHooks && h.HasPreUse() {
		output.Info("running pre-use hook…")
		if err := h.Run("pre-use", templateRoot, ctx, tmpl.FuncMap()); err != nil {
			return err
		}
	}

	tmp, err := os.MkdirTemp("", "specs-use-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	tmpl.Context = ctx
	tmpl.ComputedDefs = nil // already applied above
	if err := tmpl.Execute(tmp); err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}
	if err := osutil.CopyDir(tmp, targetDir); err != nil {
		return err
	}

	if !opts.noHooks && h.HasPostUse() {
		output.Info("running post-use hook…")
		if err := h.Run("post-use", targetDir, ctx, tmpl.FuncMap()); err != nil {
			return err
		}
	}

	output.Info("done — files written to %s", targetDir)
	return nil
}

// promptContext builds and runs a huh form for all schema keys not already in provided.
// Results are written directly into ctx.
func promptContext(ctx map[string]any, schema map[string]any, provided map[string]bool) error {
	keys := sortedKeys(schema)

	var fields []huh.Field
	stringResults := make(map[string]*string)
	boolResults := make(map[string]*bool)

	for _, key := range keys {
		if provided[key] {
			continue
		}
		defaultVal := schema[key]

		switch v := defaultVal.(type) {
		case string:
			current := v
			if s, ok := ctx[key].(string); ok {
				current = s
			}
			ptr := new(string)
			*ptr = current
			stringResults[key] = ptr
			fields = append(fields, huh.NewInput().
				Title(key).
				Value(ptr).
				Description("default: "+v),
			)

		case bool:
			current := v
			if b, ok := ctx[key].(bool); ok {
				current = b
			}
			ptr := new(bool)
			*ptr = current
			boolResults[key] = ptr
			fields = append(fields, huh.NewConfirm().
				Title(key).
				Value(ptr),
			)

		case []any:
			opts := toStringOptions(v)
			if len(opts) == 0 {
				continue
			}
			selected := opts[0]
			if s, ok := ctx[key].(string); ok {
				selected = s
			}
			ptr := new(string)
			*ptr = selected
			stringResults[key] = ptr
			fields = append(fields, huh.NewSelect[string]().
				Title(key).
				Options(huh.NewOptions(opts...)...).
				Value(ptr),
			)
		}
	}

	if len(fields) == 0 {
		return nil
	}

	if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
		return err
	}

	for k, p := range stringResults {
		ctx[k] = *p
	}
	for k, p := range boolResults {
		ctx[k] = *p
	}
	return nil
}

// sortedKeys returns map keys in alphabetical order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// toStringOptions coerces a []any (from YAML) to a []string, skipping non-strings.
func toStringOptions(v []any) []string {
	opts := make([]string, 0, len(v))
	for _, item := range v {
		if s, ok := item.(string); ok {
			opts = append(opts, s)
		}
	}
	return opts
}

// loadRawConfig reads project.yaml (or project.json as fallback) without stripping any keys.
// Used to pass the raw "hooks" value to hooks.Load.
func loadRawConfig(templateRoot string) (map[string]any, error) {
	yamlPath := filepath.Join(templateRoot, specs.ProjectYAMLFile)
	if data, err := os.ReadFile(yamlPath); err == nil {
		var m map[string]any
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", specs.ProjectYAMLFile, err)
		}
		return m, nil
	}
	jsonPath := filepath.Join(templateRoot, specs.ProjectJSONFile)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("no %s or %s found in %s", specs.ProjectYAMLFile, specs.ProjectJSONFile, templateRoot)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", specs.ProjectJSONFile, err)
	}
	return m, nil
}

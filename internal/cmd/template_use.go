package cmd

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/specsnl/specs-cli/internal/hooks"
	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	"github.com/specsnl/specs-cli/internal/util/osutil"
	"github.com/specsnl/specs-cli/internal/util/validate"
	"github.com/specsnl/specs-cli/internal/util/values"
)

type executeOpts struct {
	valuesFile      string
	argPairs        []string
	useDefaults     bool
	noHooks         bool
	continueOnError bool
	allowHooks      bool // override safe-mode hook suppression
	yes             bool // skip interactive confirmation for remote hook execution
	remote          bool // set programmatically when source is a remote repository
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
	cmd.Flags().BoolVar(&opts.continueOnError, "continue-on-error", false, "Warn and copy files verbatim on render errors instead of aborting")
	cmd.Flags().BoolVar(&opts.allowHooks, "allow-hooks", false, "Allow hooks even when --safe-mode is set")
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "Skip interactive confirmation for remote hook execution")

	return cmd
}

// executeTemplate is the shared execution logic reused by specs template use (Phase 7)
// and specs use (Phase 8).
func (a *App) executeTemplate(templateRoot, targetDir string, opts executeOpts) error {
	cfg := a.templateConfig()
	cfg.ContinueOnRenderError = opts.continueOnError
	// Inject the running CLI version so Get() can enforce a template's __specs__version
	// constraint. Covers both `specs use` and `specs template use`, which share this path.
	cfg.Version = Version

	tmpl, err := pkgtemplate.Get(templateRoot, cfg)
	if err != nil {
		return err
	}

	rawConfig, err := pkgtemplate.LoadProjectFile(templateRoot)
	if err != nil {
		return err
	}

	h, err := hooks.Load(templateRoot, rawConfig, a.HookEnvPrefix)
	if err != nil {
		return err
	}

	ctx := tmpl.Context
	provided := make(map[string]bool)
	// finalSource tracks the winning source for each context key, overwritten as values flow
	// through values_file → arg_flag → prompt → default. Logged in a single batch before
	// ApplyComputed so each key appears only once with its final source.
	finalSource := make(map[string]string)

	if opts.valuesFile != "" {
		fileVals, err := values.LoadFile(opts.valuesFile)
		if err != nil {
			return err
		}

		for k := range fileVals {
			if specs.IsReservedName(k) {
				return fmt.Errorf("%w: %q (from --values)", specs.ErrReservedVariableName, k)
			}

			provided[k] = true
			finalSource[k] = "values_file"
		}

		ctx = values.Merge(ctx, fileVals)
	}

	for _, pair := range opts.argPairs {
		k, v, err := values.ParseArg(pair)
		if err != nil {
			return err
		}

		if specs.IsReservedName(k) {
			return fmt.Errorf("%w: %q (from --arg)", specs.ErrReservedVariableName, k)
		}

		ctx[k] = v
		provided[k] = true
		finalSource[k] = "arg_flag"
	}

	if !opts.useDefaults {
		if err := promptContext(a.prompter(), ctx, tmpl.Context, tmpl.Conditionals, tmpl.Referenced, provided, finalSource); err != nil {
			return err
		}
	} else {
		resolveSelectDefaults(ctx)

		for k := range tmpl.Referenced {
			if !provided[k] {
				finalSource[k] = "default"
			}
		}
	}

	// Emit one log line per key in alphabetical order with its final resolved source.
	for _, k := range sortedKeys(finalSource) {
		slog.Debug("context key resolved", "key", k, "source", finalSource[k])
	}

	ctx, err = pkgtemplate.ApplyComputed(ctx, tmpl.ComputedDefs, tmpl.FuncMap(), tmpl.Delims())
	if err != nil {
		return err
	}

	skipHooks := (a.SafeMode && !opts.allowHooks) || opts.noHooks

	if !skipHooks && opts.remote && (h.HasPreUse() || h.HasPostUse()) {
		confirmed, err := a.confirmRemoteHooks(h, ctx, tmpl, opts.yes)
		if err != nil {
			return err
		}

		if !confirmed {
			skipHooks = true
		}
	}

	if !skipHooks && h.HasPreUse() {
		a.Output.Info("running pre-use hook…")

		if err := h.Run("pre-use", templateRoot, ctx, tmpl.FuncMap(), tmpl.Delims()); err != nil {
			return err
		}
	}

	tmp, err := os.MkdirTemp("", "specs-use-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			a.Output.Warn("failed to remove temp dir %s: %v", tmp, err)
		}
	}()

	tmpl.Context = ctx
	if err := tmpl.Execute(tmp); err != nil {
		return err
	}

	if len(tmpl.Warnings) > 0 {
		for _, w := range tmpl.Warnings {
			a.Output.Warn("%s: copied verbatim due to render error: %v", w.Path, w.Err)

			if w.Preview != "" {
				a.Output.Warn("  %s: first 80 chars: %s", filepath.Join(targetDir, w.Path), w.Preview)
			}
		}

		a.Output.Warn("run 'specs template validate %s' to see all issues", filepath.Base(templateRoot))
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	if err := osutil.CopyDir(tmp, targetDir); err != nil {
		return err
	}

	if !skipHooks && h.HasPostUse() {
		a.Output.Info("running post-use hook…")

		if err := h.Run("post-use", targetDir, ctx, tmpl.FuncMap(), tmpl.Delims()); err != nil {
			return err
		}
	}

	a.Output.Info("done — files written to %s", targetDir)

	return nil
}

// confirmRemoteHooks displays the rendered hook commands from a remote template and
// asks for interactive confirmation. Returns (true, nil) when hooks should run,
// (false, nil) when the user declined, or (false, err) on a prompt error.
// When autoYes is true the prompt is skipped and hooks are always confirmed.
func (a *App) confirmRemoteHooks(h *hooks.Hooks, ctx map[string]any, tmpl *pkgtemplate.Template, autoYes bool) (bool, error) {
	a.Output.Warn("remote template defines hooks — review before executing:")

	for _, trigger := range []string{"pre-use", "post-use"} {
		rendered, err := h.Rendered(trigger, ctx, tmpl.FuncMap(), tmpl.Delims())
		if err != nil {
			return false, fmt.Errorf("rendering %s hooks for preview: %w", trigger, err)
		}

		for _, cmd := range rendered {
			a.Output.Warn("  %s: %s", trigger, strings.TrimSpace(cmd))
		}
	}

	if autoYes {
		return true, nil
	}

	p := a.prompter()

	// Declining is the safe answer to a question that cannot be asked: the
	// template is still applied, only its hooks are skipped. Failing here would
	// break every CI job using a remote template that happens to define one.
	if !p.canPrompt() {
		a.Output.Warn("cannot ask for confirmation: %s — skipping hooks (pass --yes to run them)", p.refusal)

		return false, nil
	}

	var proceed bool
	if err := p.run(huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Allow hook execution?").Value(&proceed),
	))); err != nil {
		return false, fmt.Errorf("hook confirmation: %w", err)
	}

	return proceed, nil
}

// prompter carries what an interactive form needs: the streams it is drawn
// through, and whether it may be drawn at all.
//
// It is a value rather than a reference to the App because promptContext and
// runPromptPass are pure functions over a template's schema — the streams are
// the only thing they need from the command.
type prompter struct {
	stdin  io.Reader
	stderr io.Writer
	// allowed is false when there is nothing that could answer a form; refusal
	// says why, for the error.
	allowed bool
	refusal string
}

func (a *App) prompter() prompter {
	return prompter{
		stdin:   a.Stdin,
		stderr:  a.Stderr,
		allowed: a.canPrompt(),
		refusal: a.promptRefusal(),
	}
}

func (p prompter) canPrompt() bool { return p.allowed }

// run draws form reading from stdin and writing to stderr — never stdout, which
// carries the product. A form is narration by any reading: nobody wants its
// cursor movement in `specs template use … > out.txt`, and it would corrupt
// -o json.
func (p prompter) run(form *huh.Form) error {
	return form.WithInput(p.stdin).WithOutput(p.stderr).Run()
}

// cannotPrompt reports that keys could not be asked for, naming them and how to
// supply them instead. This is what turns a silent hang in CI into an immediate
// failure whose log says what to do.
func (p prompter) cannotPrompt(keys []string) error {
	return fmt.Errorf("%w: %s\nmissing values for: %s\nprovide them with --arg Key=Value, with --values, or take the schema defaults with --use-defaults",
		specs.ErrCannotPrompt, p.refusal, strings.Join(keys, ", "))
}

// promptContext prompts the user for schema variables not already in provided.
//
// Only variables present in referenced are considered — schema variables not
// referenced anywhere in the template files or computed expressions are skipped.
//
// Pass 1 prompts always-needed variables (those absent from conditionals).
// Pass 2+ iterates in dependency order: each round finds conditional variables
// whose gate variables are all resolved, evaluates their condition against the
// now-final ctx, and prompts those that are needed. This correctly handles
// nested eq/ne gates where the gate variable is itself conditional.
func promptContext(
	p prompter,
	ctx map[string]any,
	schema map[string]any,
	conds pkgtemplate.Conditionals,
	referenced map[string]bool,
	provided map[string]bool,
	finalSource map[string]string,
) error {
	schemaKeys := make(map[string]bool, len(schema))
	for k := range schema {
		schemaKeys[k] = true
	}

	var alwaysKeys []string

	remaining := make(map[string]bool) // conditional keys not yet resolved

	for _, k := range sortedKeys(schema) {
		if !referenced[k] {
			continue // never used in templates or computed expressions
		}

		if _, conditional := conds[k]; conditional {
			remaining[k] = true
		} else {
			alwaysKeys = append(alwaysKeys, k)
		}
	}

	// Pass 1: always-needed variables.
	if err := runPromptPass(p, ctx, schema, alwaysKeys, provided, finalSource); err != nil {
		return err
	}

	// resolved tracks which schema keys have a final value in ctx.
	resolved := make(map[string]bool, len(alwaysKeys)+len(provided))
	for _, k := range alwaysKeys {
		resolved[k] = true
	}

	for k := range provided {
		resolved[k] = true
	}

	// Iterative conditional passes: each round handles one dependency layer.
	for len(remaining) > 0 {
		// Find keys whose gate variables are all resolved (or not in schema).
		var ready []string

		for k := range remaining {
			allResolved := true

			for _, gk := range conds[k].Keys() {
				if schemaKeys[gk] && !resolved[gk] {
					allResolved = false
					break
				}
			}

			if allResolved {
				ready = append(ready, k)
			}
		}

		if len(ready) == 0 {
			break // no progress: remaining keys have unresolvable dependencies
		}

		sort.Strings(ready)

		var toPrompt []string

		for _, k := range ready {
			if conds[k].Eval(ctx) {
				toPrompt = append(toPrompt, k)
			}
		}

		if err := runPromptPass(p, ctx, schema, toPrompt, provided, finalSource); err != nil {
			return err
		}

		for _, k := range ready {
			resolved[k] = true

			delete(remaining, k)
		}
	}

	return nil
}

// runPromptPass builds a huh form for the given keys and runs it.
// Results are written back into ctx and finalSource is updated with source="prompt".
//
// When there is nothing that could answer the form, it returns an error naming
// every key it would have asked for rather than blocking on a read nobody will
// answer — the CI hang this guard exists to prevent.
func runPromptPass(
	p prompter,
	ctx map[string]any,
	schema map[string]any,
	keys []string,
	provided map[string]bool,
	finalSource map[string]string,
) error {
	var (
		fields []huh.Field
		// asked names the keys fields were built for, in the order keys arrives —
		// already alphabetical — so the refusal below can list them.
		asked []string
	)

	stringResults := make(map[string]*string)
	boolResults := make(map[string]*bool)

	for _, key := range keys {
		if provided[key] {
			continue
		}

		before := len(fields)
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

		if len(fields) > before {
			asked = append(asked, key)
		}
	}

	if len(fields) == 0 {
		return nil
	}

	if !p.canPrompt() {
		return p.cannotPrompt(asked)
	}

	if err := p.run(huh.NewForm(huh.NewGroup(fields...))); err != nil {
		return err
	}

	for k, ptr := range stringResults {
		ctx[k] = *ptr
		finalSource[k] = "prompt"
	}

	for k, ptr := range boolResults {
		ctx[k] = *ptr
		finalSource[k] = "prompt"
	}

	return nil
}

// sortedKeys returns map keys in alphabetical order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

// resolveSelectDefaults replaces any []any value in ctx with its first string element.
// This mirrors what runPromptPass does for select fields when --use-defaults skips prompting.
func resolveSelectDefaults(ctx map[string]any) {
	for k, v := range ctx {
		if arr, ok := v.([]any); ok {
			if opts := toStringOptions(arr); len(opts) > 0 {
				ctx[k] = opts[0]
			}
		}
	}
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

package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
	"text/template/parse"

	"gopkg.in/yaml.v3"

	"github.com/specsnl/specs-cli/internal/specs"
)

// LoadUserContext loads project.yaml (or project.json as fallback) from templateRoot.
// It strips the reserved "computed" and "hooks" top-level keys, then resolves any
// referenced defaults (string values containing the left delimiter) in topological order.
// Returns the user input map and the raw computed definitions separately.
func LoadUserContext(templateRoot string, funcMap texttemplate.FuncMap, delims specs.Delimiters) (userCtx map[string]any, computedDefs map[string]string, err error) {
	raw, err := LoadProjectFile(templateRoot)
	if err != nil {
		return nil, nil, err
	}

	computedDefs, err = extractComputed(raw)
	if err != nil {
		return nil, nil, err
	}

	delete(raw, "hooks")                      // consumed by the hook runner, not a template variable
	delete(raw, specs.ProjectDelimitersKey)   // consumed by Get(); must not appear as a user variable

	userCtx, err = resolveReferencedDefaults(raw, funcMap, delims)
	return userCtx, computedDefs, err
}

// ExtractProjectDelimiters reads project.yaml and returns the delimiters configured
// under the __delimiters key, falling back to fallback when the key is absent.
// Returns an error if __delimiters is present but malformed.
func ExtractProjectDelimiters(templateRoot string, fallback specs.Delimiters) (specs.Delimiters, error) {
	raw, err := LoadProjectFile(templateRoot)
	if err != nil {
		return fallback, err
	}
	v, ok := raw[specs.ProjectDelimitersKey]
	if !ok {
		return fallback, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fallback, specs.ErrInvalidDelimiters
	}
	left, leftOK := m["left"].(string)
	right, rightOK := m["right"].(string)
	if !leftOK || !rightOK || left == "" || right == "" {
		return fallback, specs.ErrInvalidDelimiters
	}
	return specs.Delimiters{Left: left, Right: right}, nil
}

// ApplyComputed resolves computed definitions against the finalised context (post-prompt)
// and returns a new map containing both user inputs and computed values.
// Called after prompting and --values/--arg overrides are complete.
func ApplyComputed(ctx map[string]any, defs map[string]string, funcMap texttemplate.FuncMap, delims specs.Delimiters) (map[string]any, error) {
	if len(defs) == 0 {
		return ctx, nil
	}

	// Build dependency graph among computed keys.
	keys := make([]string, 0, len(defs))
	for k := range defs {
		keys = append(keys, k)
	}

	deps := make(map[string][]string, len(keys))
	for k, expr := range defs {
		deps[k] = extractRefs(expr, funcMap, delims)
	}

	sorted, err := topoSort(keys, deps)
	if err != nil {
		return nil, fmt.Errorf("computed values: %w", err)
	}

	// Copy the context so we don't mutate the caller's map.
	result := make(map[string]any, len(ctx)+len(defs))
	maps.Copy(result, ctx)

	for _, k := range sorted {
		expr := defs[k]
		val, err := renderExpr(expr, result, funcMap, delims)
		if err != nil {
			return nil, fmt.Errorf("computed %q: %w", k, err)
		}
		result[k] = val
		slog.Debug("context key resolved", "key", k, "source", "computed")
	}

	return result, nil
}

// LoadProjectFile reads project.yaml or project.yml (falls back to project.json) and
// returns the raw, unmodified map. Reserved keys (hooks, computed, __delimiters) are
// preserved so that callers such as hooks.Load can access them directly.
// Returns ErrAmbiguousProjectFile if both YAML variants are present.
func LoadProjectFile(templateRoot string) (map[string]any, error) {
	yamlPath := filepath.Join(templateRoot, specs.ProjectYAMLFile)
	ymlPath := filepath.Join(templateRoot, specs.ProjectYMLFile)
	_, yamlErr := os.Stat(yamlPath)
	_, ymlErr := os.Stat(ymlPath)
	hasYAML := yamlErr == nil
	hasYML := ymlErr == nil

	if hasYAML && hasYML {
		return nil, specs.ErrAmbiguousProjectFile
	}

	if hasYAML || hasYML {
		chosen := yamlPath
		chosenName := specs.ProjectYAMLFile
		if hasYML {
			chosen = ymlPath
			chosenName = specs.ProjectYMLFile
		}
		data, err := os.ReadFile(chosen)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", chosenName, err)
		}
		var ctx map[string]any
		if err := yaml.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", chosenName, err)
		}
		return ctx, nil
	}

	jsonPath := filepath.Join(templateRoot, specs.ProjectJSONFile)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("%w in %s", specs.ErrProjectFileMissing, templateRoot)
	}
	var ctx map[string]any
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", specs.ProjectJSONFile, err)
	}
	return ctx, nil
}

// extractComputed removes the "computed" key from raw and returns its string entries.
// Returns an error if any computed key conflicts with a user input key.
func extractComputed(raw map[string]any) (map[string]string, error) {
	v, ok := raw["computed"]
	if !ok {
		return nil, nil
	}
	delete(raw, "computed")

	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: \"computed\" must be a mapping, got %T", specs.ErrInvalidComputedDef, v)
	}

	defs := make(map[string]string, len(m))
	for k, val := range m {
		if _, conflict := raw[k]; conflict {
			return nil, fmt.Errorf("%w: key %q conflicts with a user input key", specs.ErrInvalidComputedDef, k)
		}
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("%w: value for %q must be a string, got %T", specs.ErrInvalidComputedDef, k, val)
		}
		defs[k] = s
	}
	return defs, nil
}

// resolveReferencedDefaults renders string values containing the left delimiter in
// topological order so that each key's pre-fill value is correct before the user is prompted.
// It returns a new map; the caller's input is never modified.
func resolveReferencedDefaults(ctx map[string]any, funcMap texttemplate.FuncMap, delims specs.Delimiters) (map[string]any, error) {
	// Find keys whose string value is a template expression.
	var refKeys []string
	for k, v := range ctx {
		if s, ok := v.(string); ok && strings.Contains(s, delims.Left) {
			refKeys = append(refKeys, k)
		}
	}
	if len(refKeys) == 0 {
		return ctx, nil
	}

	deps := make(map[string][]string, len(refKeys))
	for _, k := range refKeys {
		deps[k] = extractRefs(ctx[k].(string), funcMap, delims)
	}

	sorted, err := topoSort(refKeys, deps)
	if err != nil {
		return nil, fmt.Errorf("referenced defaults: %w", err)
	}

	// Copy the input map so the caller's map is not mutated.
	result := make(map[string]any, len(ctx))
	maps.Copy(result, ctx)

	for _, k := range sorted {
		val, err := renderExpr(result[k].(string), result, funcMap, delims)
		if err != nil {
			return nil, fmt.Errorf("referenced default %q: %w", k, err)
		}
		result[k] = val
	}

	return result, nil
}

// topoSort returns keys in an order where each key comes after all of its
// dependencies that are also in keys. Keys may depend on external items not in
// keys; those are treated as already-resolved leaves.
// Returns an error if a cycle exists among keys.
func topoSort(keys []string, deps map[string][]string) ([]string, error) {
	inSet := make(map[string]bool, len(keys))
	for _, k := range keys {
		inSet[k] = true
	}

	inDegree := make(map[string]int, len(keys))
	dependents := make(map[string][]string, len(keys))
	for _, k := range keys {
		inDegree[k] = 0
	}
	for _, k := range keys {
		for _, dep := range deps[k] {
			if inSet[dep] {
				inDegree[k]++
				dependents[dep] = append(dependents[dep], k)
			}
		}
	}

	queue := make([]string, 0, len(keys))
	for _, k := range keys {
		if inDegree[k] == 0 {
			queue = append(queue, k)
		}
	}

	sorted := make([]string, 0, len(keys))
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		sorted = append(sorted, n)
		for _, dep := range dependents[n] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(sorted) != len(keys) {
		var cycle []string
		for _, k := range keys {
			if inDegree[k] > 0 {
				cycle = append(cycle, k)
			}
		}
		return nil, fmt.Errorf("%w: %s", specs.ErrCyclicDependency, strings.Join(cycle, ", "))
	}

	return sorted, nil
}

// extractRefs parses a delimited template expression and returns all
// top-level .Key references found in it.
// It uses texttemplate.New (not parse.New) so that Go's builtin functions
// such as eq, ne, and, or, not are registered and the parser accepts them.
func extractRefs(expr string, funcMap texttemplate.FuncMap, delims specs.Delimiters) []string {
	if !strings.Contains(expr, delims.Left) {
		return nil
	}
	tmpl, err := texttemplate.New("").
		Delims(delims.Left, delims.Right).
		Funcs(funcMap).
		Parse(expr)
	if err != nil || tmpl == nil || tmpl.Tree == nil || tmpl.Tree.Root == nil {
		slog.Debug("extractRefs: failed to parse expression; dependency detection skipped", "err", err)
		return nil // parse errors surface during actual rendering
	}
	seen := make(map[string]bool)
	var refs []string
	walkForRefs(tmpl.Tree.Root, seen, &refs)
	return refs
}

// walkForRefs recursively walks a template AST node collecting FieldNode identifiers.
func walkForRefs(node parse.Node, seen map[string]bool, refs *[]string) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		for _, child := range n.Nodes {
			walkForRefs(child, seen, refs)
		}
	case *parse.ActionNode:
		walkForRefs(n.Pipe, seen, refs)
	case *parse.PipeNode:
		for _, cmd := range n.Cmds {
			walkForRefs(cmd, seen, refs)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			walkForRefs(arg, seen, refs)
		}
	case *parse.FieldNode:
		if len(n.Ident) > 0 {
			key := n.Ident[0]
			if !seen[key] {
				seen[key] = true
				*refs = append(*refs, key)
			}
		}
	case *parse.IfNode:
		walkForRefs(n.Pipe, seen, refs)
		walkForRefs(n.List, seen, refs)
		if n.ElseList != nil {
			walkForRefs(n.ElseList, seen, refs)
		}
	case *parse.RangeNode:
		walkForRefs(n.Pipe, seen, refs)
		walkForRefs(n.List, seen, refs)
		if n.ElseList != nil {
			walkForRefs(n.ElseList, seen, refs)
		}
	case *parse.WithNode:
		walkForRefs(n.Pipe, seen, refs)
		walkForRefs(n.List, seen, refs)
		if n.ElseList != nil {
			walkForRefs(n.ElseList, seen, refs)
		}
	}
}

// renderExpr renders a single delimited template expression against ctx.
func renderExpr(expr string, ctx map[string]any, funcMap texttemplate.FuncMap, delims specs.Delimiters) (string, error) {
	tmpl, err := texttemplate.New("").
		Delims(delims.Left, delims.Right).
		Funcs(funcMap).
		Option("missingkey=error").
		Parse(expr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

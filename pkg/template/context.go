package template

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
	"text/template/parse"

	"gopkg.in/yaml.v3"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// LoadUserContext loads project.yaml (or project.json as fallback) from templateRoot.
// It strips the reserved "computed" and "hooks" top-level keys, then resolves any
// referenced defaults (string values containing "[[") in topological order.
// Returns the user input map and the raw computed definitions separately.
func LoadUserContext(templateRoot string, funcMap texttemplate.FuncMap) (userCtx map[string]any, computedDefs map[string]string, err error) {
	raw, err := loadContextFile(templateRoot)
	if err != nil {
		return nil, nil, err
	}

	computedDefs, err = extractComputed(raw)
	if err != nil {
		return nil, nil, err
	}

	delete(raw, "hooks") // consumed by the hook runner, not a template variable

	userCtx, err = resolveReferencedDefaults(raw, funcMap)
	return userCtx, computedDefs, err
}

// ApplyComputed resolves computed definitions against the finalised context (post-prompt)
// and returns a new map containing both user inputs and computed values.
// Called after prompting and --values/--arg overrides are complete.
func ApplyComputed(ctx map[string]any, defs map[string]string, funcMap texttemplate.FuncMap) (map[string]any, error) {
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
		deps[k] = extractRefs(expr, funcMap)
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
		val, err := renderExpr(expr, result, funcMap)
		if err != nil {
			return nil, fmt.Errorf("computed %q: %w", k, err)
		}
		result[k] = val
	}

	return result, nil
}

// loadContextFile reads project.yaml; falls back to project.json.
func loadContextFile(templateRoot string) (map[string]any, error) {
	yamlPath := filepath.Join(templateRoot, specs.ProjectYAMLFile)
	if data, err := os.ReadFile(yamlPath); err == nil {
		var ctx map[string]any
		if err := yaml.Unmarshal(data, &ctx); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", specs.ProjectYAMLFile, err)
		}
		return ctx, nil
	}

	jsonPath := filepath.Join(templateRoot, specs.ProjectJSONFile)
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("no %s or %s found in %s", specs.ProjectYAMLFile, specs.ProjectJSONFile, templateRoot)
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
		return nil, fmt.Errorf("\"computed\" must be a mapping, got %T", v)
	}

	defs := make(map[string]string, len(m))
	for k, val := range m {
		if _, conflict := raw[k]; conflict {
			return nil, fmt.Errorf("computed key %q conflicts with a user input key", k)
		}
		s, ok := val.(string)
		if !ok {
			return nil, fmt.Errorf("computed value for %q must be a string, got %T", k, val)
		}
		defs[k] = s
	}
	return defs, nil
}

// resolveReferencedDefaults renders string values containing "[[" in topological order
// so that each key's pre-fill value is correct before the user is prompted.
func resolveReferencedDefaults(ctx map[string]any, funcMap texttemplate.FuncMap) (map[string]any, error) {
	// Find keys whose string value is a template expression.
	var refKeys []string
	for k, v := range ctx {
		if s, ok := v.(string); ok && strings.Contains(s, "[[") {
			refKeys = append(refKeys, k)
		}
	}
	if len(refKeys) == 0 {
		return ctx, nil
	}

	deps := make(map[string][]string, len(refKeys))
	for _, k := range refKeys {
		deps[k] = extractRefs(ctx[k].(string), funcMap)
	}

	sorted, err := topoSort(refKeys, deps)
	if err != nil {
		return nil, fmt.Errorf("referenced defaults: %w", err)
	}

	for _, k := range sorted {
		val, err := renderExpr(ctx[k].(string), ctx, funcMap)
		if err != nil {
			return nil, fmt.Errorf("referenced default %q: %w", k, err)
		}
		ctx[k] = val
	}

	return ctx, nil
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
		return nil, fmt.Errorf("cycle detected among keys: %s", strings.Join(cycle, ", "))
	}

	return sorted, nil
}

// extractRefs parses a "[[ ]]"-delimited template expression and returns all
// top-level .Key references found in it.
func extractRefs(expr string, funcMap texttemplate.FuncMap) []string {
	if !strings.Contains(expr, "[[") {
		return nil
	}
	funcs := map[string]any(funcMap)
	tree, err := parse.New("t").Parse(expr, "[[", "]]", map[string]*parse.Tree{}, funcs)
	if err != nil || tree == nil || tree.Root == nil {
		return nil // parse errors surface during actual rendering
	}
	seen := make(map[string]bool)
	var refs []string
	walkForRefs(tree.Root, seen, &refs)
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

// renderExpr renders a single "[[ ]]"-delimited template expression against ctx.
func renderExpr(expr string, ctx map[string]any, funcMap texttemplate.FuncMap) (string, error) {
	tmpl, err := texttemplate.New("").
		Delims("[[", "]]").
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

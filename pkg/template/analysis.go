package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
	"text/template/parse"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// Conditionals maps a schema variable name to the condition that must be true for
// the variable to be needed. A variable absent from the map is always needed.
type Conditionals map[string]Cond

// AnalyzeConditionals walks every file and filename template under
// templateRoot/template/ and returns a conditionals map and a referenced set.
// A variable appears in the conditionals map only when it is accessed exclusively
// inside recognisable conditional blocks; otherwise it is absent (always prompt).
// The referenced set contains every variable name that appears anywhere in the
// template file tree, conditional or not.
func AnalyzeConditionals(
	templateRoot string,
	userCtx map[string]any,
	funcMap texttemplate.FuncMap,
) (Conditionals, map[string]bool, error) {
	srcRoot := filepath.Join(templateRoot, specs.TemplateDirFile)

	always := make(map[string]bool)
	conds := make(Conditionals)

	err := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcRoot, path)
		if rel == "." {
			return nil
		}

		// Analyse the filename/path template itself.
		analyseExpr(rel, nil, funcMap, conds, always)

		// For files, analyse the content.
		if !d.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			analyseExpr(string(data), nil, funcMap, conds, always)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Build the referenced set before cleanup (union of always and all cond keys).
	referenced := make(map[string]bool, len(always)+len(conds))
	for k := range always {
		referenced[k] = true
	}
	for k := range conds {
		referenced[k] = true
	}

	// Remove any key that turned out to be unconditionally accessed.
	for k := range always {
		delete(conds, k)
	}
	return conds, referenced, nil
}

// analyseExpr parses one template expression and walks its AST.
// Uses texttemplate.New so that built-in functions (not, eq, and, or, …) are
// recognised by the parser without having to enumerate them manually.
func analyseExpr(
	src string,
	outerGate Cond,
	funcMap texttemplate.FuncMap,
	conds Conditionals,
	always map[string]bool,
) {
	if !strings.Contains(src, "[[") {
		return
	}
	tmpl, err := texttemplate.New("").Delims("[[", "]]").Funcs(funcMap).Parse(src)
	if err != nil || tmpl == nil || tmpl.Tree == nil || tmpl.Tree.Root == nil {
		return
	}
	walkNode(tmpl.Tree.Root, outerGate, funcMap, conds, always)
}

// walkNode recursively walks a template AST node, tracking conditional gates.
func walkNode(
	node parse.Node,
	gate Cond,
	funcMap texttemplate.FuncMap,
	conds Conditionals,
	always map[string]bool,
) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parse.ListNode:
		for _, child := range n.Nodes {
			walkNode(child, gate, funcMap, conds, always)
		}

	case *parse.IfNode:
		// Walk the condition pipe at the current gate level so gate variables
		// are classified as always-needed rather than self-conditionally-needed.
		walkNode(n.Pipe, gate, funcMap, conds, always)

		innerCond, ok := parsePipeCond(n.Pipe)
		if ok {
			thenGate := andGates(gate, innerCond)
			elseGate := andGates(gate, condNot{innerCond})
			walkNode(n.List, thenGate, funcMap, conds, always)
			if n.ElseList != nil {
				walkNode(n.ElseList, elseGate, funcMap, conds, always)
			}
		} else {
			// Unrecognised condition — walk bodies under the current gate unchanged
			// (conservative fallback: treat as unconditional).
			walkNode(n.List, gate, funcMap, conds, always)
			if n.ElseList != nil {
				walkNode(n.ElseList, gate, funcMap, conds, always)
			}
		}

	case *parse.ActionNode:
		walkNode(n.Pipe, gate, funcMap, conds, always)

	case *parse.PipeNode:
		for _, cmd := range n.Cmds {
			walkNode(cmd, gate, funcMap, conds, always)
		}

	case *parse.CommandNode:
		for _, arg := range n.Args {
			walkNode(arg, gate, funcMap, conds, always)
		}

	case *parse.FieldNode:
		if len(n.Ident) == 0 {
			return
		}
		key := n.Ident[0]
		if gate == nil {
			always[key] = true
		} else if !always[key] {
			if existing, exists := conds[key]; exists {
				// Seen under a different condition — treat as always needed.
				if fmt.Sprint(existing) != fmt.Sprint(gate) {
					always[key] = true
					delete(conds, key)
				}
			} else {
				conds[key] = gate
			}
		}

	case *parse.RangeNode:
		walkNode(n.Pipe, gate, funcMap, conds, always)
		walkNode(n.List, gate, funcMap, conds, always)
		if n.ElseList != nil {
			walkNode(n.ElseList, gate, funcMap, conds, always)
		}

	case *parse.WithNode:
		walkNode(n.Pipe, gate, funcMap, conds, always)
		walkNode(n.List, gate, funcMap, conds, always)
		if n.ElseList != nil {
			walkNode(n.ElseList, gate, funcMap, conds, always)
		}
	}
}

// andGates combines an outer gate with an inner condition into a conjunction.
// If outer is nil, inner is returned as-is.
func andGates(outer Cond, inner Cond) Cond {
	if outer == nil {
		return inner
	}
	return condAnd{subs: []Cond{outer, inner}}
}

// parsePipeCond attempts to parse a template pipe node into a typed Cond.
// Returns (cond, true) on success, (nil, false) for any unrecognised form.
func parsePipeCond(pipe *parse.PipeNode) (Cond, bool) {
	if pipe == nil || len(pipe.Cmds) != 1 {
		return nil, false
	}
	return parseCmdCond(pipe.Cmds[0])
}

func parseCmdCond(cmd *parse.CommandNode) (Cond, bool) {
	args := cmd.Args
	if len(args) == 0 {
		return nil, false
	}

	// [[if .Var]] — single field reference
	if len(args) == 1 {
		if f, ok := args[0].(*parse.FieldNode); ok && len(f.Ident) == 1 {
			return condField{f.Ident[0]}, true
		}
		return nil, false
	}

	// Function form: first arg must be an identifier naming the function.
	fn, ok := args[0].(*parse.IdentifierNode)
	if !ok {
		return nil, false
	}

	switch fn.Ident {
	case "not":
		if len(args) != 2 {
			return nil, false
		}
		sub, ok := parseArgCond(args[1])
		if !ok {
			return nil, false
		}
		return condNot{sub}, true

	case "eq", "ne":
		if len(args) != 3 {
			return nil, false
		}
		field, ok := args[1].(*parse.FieldNode)
		if !ok || len(field.Ident) != 1 {
			return nil, false
		}
		lit, ok := parseLiteral(args[2])
		if !ok {
			return nil, false
		}
		if fn.Ident == "eq" {
			return condEq{field.Ident[0], lit}, true
		}
		return condNe{field.Ident[0], lit}, true

	case "and", "or":
		if len(args) < 3 {
			return nil, false
		}
		subs := make([]Cond, 0, len(args)-1)
		for _, arg := range args[1:] {
			sub, ok := parseArgCond(arg)
			if !ok {
				return nil, false
			}
			subs = append(subs, sub)
		}
		if fn.Ident == "and" {
			return condAnd{subs}, true
		}
		return condOr{subs}, true
	}

	return nil, false
}

// parseArgCond handles an argument that is either a bare field or a parenthesised
// sub-expression (PipeNode), enabling recursion for nested conditions.
func parseArgCond(arg parse.Node) (Cond, bool) {
	switch n := arg.(type) {
	case *parse.FieldNode:
		if len(n.Ident) == 1 {
			return condField{n.Ident[0]}, true
		}
	case *parse.PipeNode:
		return parsePipeCond(n)
	}
	return nil, false
}

// parseLiteral extracts a Go value from a string, bool, or number node.
func parseLiteral(node parse.Node) (any, bool) {
	switch n := node.(type) {
	case *parse.StringNode:
		return n.Text, true
	case *parse.BoolNode:
		return n.True, true
	case *parse.NumberNode:
		if n.IsInt {
			return n.Int64, true
		}
		if n.IsFloat {
			return n.Float64, true
		}
	}
	return nil, false
}

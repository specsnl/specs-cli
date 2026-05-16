package template

import "fmt"

// Cond is a boolean condition derived from a [[if …]] expression in a template file.
// All concrete types are unexported; callers interact only through Eval and Keys.
type Cond interface {
	// Eval returns true when this condition is satisfied by the given context.
	Eval(ctx map[string]any) bool
	// Keys returns all schema variable names referenced by this condition.
	Keys() []string
}

// condField is satisfied when the named key is truthy (bool true or non-empty string).
type condField struct{ key string }

// condNot negates a sub-condition.
type condNot struct{ sub Cond }

// condEq is satisfied when ctx[key] == val (string comparison via fmt.Sprint).
type condEq struct {
	key string
	val any
}

// condNe is satisfied when ctx[key] != val.
type condNe struct {
	key string
	val any
}

// condAnd is satisfied when ALL sub-conditions are satisfied.
type condAnd struct{ subs []Cond }

// condOr is satisfied when ANY sub-condition is satisfied.
type condOr struct{ subs []Cond }

func (c condField) Eval(ctx map[string]any) bool { return isTruthy(ctx[c.key]) }
func (c condNot) Eval(ctx map[string]any) bool   { return !c.sub.Eval(ctx) }
func (c condEq) Eval(ctx map[string]any) bool {
	return fmt.Sprint(ctx[c.key]) == fmt.Sprint(c.val)
}
func (c condNe) Eval(ctx map[string]any) bool {
	return fmt.Sprint(ctx[c.key]) != fmt.Sprint(c.val)
}
func (c condAnd) Eval(ctx map[string]any) bool {
	for _, s := range c.subs {
		if !s.Eval(ctx) {
			return false
		}
	}
	return true
}
func (c condOr) Eval(ctx map[string]any) bool {
	for _, s := range c.subs {
		if s.Eval(ctx) {
			return true
		}
	}
	return false
}

func (c condField) Keys() []string { return []string{c.key} }
func (c condNot) Keys() []string   { return c.sub.Keys() }
func (c condEq) Keys() []string    { return []string{c.key} }
func (c condNe) Keys() []string    { return []string{c.key} }
func (c condAnd) Keys() []string   { return collectKeys(c.subs) }
func (c condOr) Keys() []string    { return collectKeys(c.subs) }

func collectKeys(subs []Cond) []string {
	var keys []string
	for _, s := range subs {
		keys = append(keys, s.Keys()...)
	}
	return keys
}

// isTruthy returns true for bool true or a non-empty string; false for everything else.
func isTruthy(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	default:
		return false
	}
}

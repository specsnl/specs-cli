package template

import "testing"

func TestCondField_True(t *testing.T) {
	c := condField{"A"}
	if !c.Eval(map[string]any{"A": true}) {
		t.Error("expected true for bool true")
	}
}

func TestCondField_False(t *testing.T) {
	c := condField{"A"}
	if c.Eval(map[string]any{"A": false}) {
		t.Error("expected false for bool false")
	}
}

func TestCondField_NonEmptyString(t *testing.T) {
	c := condField{"A"}
	if !c.Eval(map[string]any{"A": "hello"}) {
		t.Error("expected true for non-empty string")
	}
}

func TestCondField_EmptyString(t *testing.T) {
	c := condField{"A"}
	if c.Eval(map[string]any{"A": ""}) {
		t.Error("expected false for empty string")
	}
}

func TestCondField_Missing(t *testing.T) {
	c := condField{"A"}
	if c.Eval(map[string]any{}) {
		t.Error("expected false for missing key")
	}
}

func TestCondNot_True(t *testing.T) {
	c := condNot{condField{"A"}}
	if c.Eval(map[string]any{"A": true}) {
		t.Error("expected false when negating true")
	}
}

func TestCondNot_False(t *testing.T) {
	c := condNot{condField{"A"}}
	if !c.Eval(map[string]any{"A": false}) {
		t.Error("expected true when negating false")
	}
}

func TestCondEq_Match(t *testing.T) {
	c := condEq{"T", "pg"}
	if !c.Eval(map[string]any{"T": "pg"}) {
		t.Error("expected true for matching value")
	}
}

func TestCondEq_NoMatch(t *testing.T) {
	c := condEq{"T", "pg"}
	if c.Eval(map[string]any{"T": "mysql"}) {
		t.Error("expected false for non-matching value")
	}
}

func TestCondNe_Match(t *testing.T) {
	c := condNe{"T", "pg"}
	if !c.Eval(map[string]any{"T": "mysql"}) {
		t.Error("expected true when value differs")
	}
}

func TestCondNe_NoMatch(t *testing.T) {
	c := condNe{"T", "pg"}
	if c.Eval(map[string]any{"T": "pg"}) {
		t.Error("expected false when value equals")
	}
}

func TestCondAnd_AllTrue(t *testing.T) {
	c := condAnd{[]Cond{condField{"A"}, condField{"B"}}}
	if !c.Eval(map[string]any{"A": true, "B": true}) {
		t.Error("expected true when all sub-conditions are true")
	}
}

func TestCondAnd_OneFalse(t *testing.T) {
	c := condAnd{[]Cond{condField{"A"}, condField{"B"}}}
	if c.Eval(map[string]any{"A": true, "B": false}) {
		t.Error("expected false when one sub-condition is false")
	}
}

func TestCondOr_OneTrue(t *testing.T) {
	c := condOr{[]Cond{condField{"A"}, condField{"B"}}}
	if !c.Eval(map[string]any{"A": false, "B": true}) {
		t.Error("expected true when one sub-condition is true")
	}
}

func TestCondOr_AllFalse(t *testing.T) {
	c := condOr{[]Cond{condField{"A"}, condField{"B"}}}
	if c.Eval(map[string]any{"A": false, "B": false}) {
		t.Error("expected false when all sub-conditions are false")
	}
}

func TestCondField_Keys(t *testing.T) {
	c := condField{"X"}
	keys := c.Keys()
	if len(keys) != 1 || keys[0] != "X" {
		t.Errorf("Keys() = %v, want [X]", keys)
	}
}

func TestCondNot_Keys(t *testing.T) {
	c := condNot{condField{"X"}}
	keys := c.Keys()
	if len(keys) != 1 || keys[0] != "X" {
		t.Errorf("Keys() = %v, want [X]", keys)
	}
}

func TestCondAnd_Keys(t *testing.T) {
	c := condAnd{[]Cond{condField{"A"}, condField{"B"}}}
	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() = %v, want 2 keys", keys)
	}
}

func TestCondOr_Keys(t *testing.T) {
	c := condOr{[]Cond{condField{"A"}, condField{"B"}}}
	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() = %v, want 2 keys", keys)
	}
}

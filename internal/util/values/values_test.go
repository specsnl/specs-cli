package values_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/internal/util/values"
)

func TestLoadFile_Valid(t *testing.T) {
	f := filepath.Join(t.TempDir(), "vals.json")
	if err := os.WriteFile(f, []byte(`{"Name":"acme"}`), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := values.LoadFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m["Name"] != "acme" {
		t.Errorf("Name = %v, want %q", m["Name"], "acme")
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := values.LoadFile(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFile_InvalidJSON(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(f, []byte(`{not valid`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := values.LoadFile(f)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadFile_ValidYAML(t *testing.T) {
	for _, ext := range []string{".yaml", ".yml"} {
		t.Run(ext, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "vals"+ext)
			if err := os.WriteFile(f, []byte("Name: acme\n"), 0644); err != nil {
				t.Fatal(err)
			}

			m, err := values.LoadFile(f)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if m["Name"] != "acme" {
				t.Errorf("Name = %v, want %q", m["Name"], "acme")
			}
		})
	}
}

func TestLoadFile_InvalidYAML(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(f, []byte(":\tinvalid: yaml: [\n"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := values.LoadFile(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseArg_Valid(t *testing.T) {
	k, v, err := values.ParseArg("Name=acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if k != "Name" || v != "acme" {
		t.Errorf("got key=%q value=%q, want key=%q value=%q", k, v, "Name", "acme")
	}
}

func TestParseArg_WithEquals(t *testing.T) {
	k, v, err := values.ParseArg("Url=http://x.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if k != "Url" || v != "http://x.com" {
		t.Errorf("got key=%q value=%q", k, v)
	}
}

func TestParseArg_NoEquals(t *testing.T) {
	_, _, err := values.ParseArg("NoEquals")
	if err == nil {
		t.Fatal("expected error for missing =")
	}
}

func TestMerge_OverridesBase(t *testing.T) {
	base := map[string]any{"A": json.Number("1")}
	overrides := map[string]any{"A": json.Number("2"), "B": json.Number("3")}

	result := values.Merge(base, overrides)
	if result["A"] != json.Number("2") {
		t.Errorf("A = %v, want 2", result["A"])
	}

	if result["B"] != json.Number("3") {
		t.Errorf("B = %v, want 3", result["B"])
	}
}

func TestMerge_DoesNotMutateBase(t *testing.T) {
	base := map[string]any{"A": json.Number("1")}
	overrides := map[string]any{"A": json.Number("2")}
	values.Merge(base, overrides)

	if base["A"] != json.Number("1") {
		t.Errorf("base mutated: A = %v, want 1", base["A"])
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name  string
		value string
		def   any
		want  any
	}{
		{"bool false", "false", true, false},
		{"bool true", "true", false, true},
		{"bool numeric", "0", true, false},
		{"int", "5432", 3306, 5432},
		{"float", "1.5", 2.0, 1.5},
		{"select option", "MIT", []any{"Apache-2.0", "MIT"}, "MIT"},
		{"string", "false", "default", "false"},
		{"not in schema", "42", nil, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := values.Convert("Key", tt.value, tt.def)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestConvert_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		value string
		def   any
	}{
		{"bool", "nope", false},
		{"int", "abc", 3306},
		{"int given a float", "1.5", 3306},
		{"float", "abc", 2.0},
		{"select", "GPL", []any{"Apache-2.0", "MIT"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := values.Convert("Key", tt.value, tt.def)
			if err == nil {
				t.Fatal("expected error")
			}

			if !strings.Contains(err.Error(), "Key="+tt.value) {
				t.Errorf("error %q does not name the argument", err)
			}
		})
	}
}

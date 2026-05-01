package values_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/values"
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

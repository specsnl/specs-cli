package cmd

import (
	"strings"
	"testing"
)

func TestVersion_PrintsVersion(t *testing.T) {
	out, err := executeCmd("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "specs version") {
		t.Errorf("expected output to contain 'specs version', got: %q", out)
	}
}

func TestVersion_DontPrettify(t *testing.T) {
	out, err := executeCmd("version", "--dont-prettify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "specs version") {
		t.Errorf("expected plain output, got: %q", out)
	}
	if !strings.Contains(out, Version) {
		t.Errorf("expected output to contain version %q, got: %q", Version, out)
	}
}

func TestVersionFlag_LongForm(t *testing.T) {
	out, err := executeCmd("--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, Version) {
		t.Errorf("expected output to contain %q, got: %q", Version, out)
	}
	if strings.Contains(out, "specs version") {
		t.Errorf("expected plain output without 'specs version' prefix, got: %q", out)
	}
}

func TestVersionFlag_ShortForm(t *testing.T) {
	out, err := executeCmd("-v")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, Version) {
		t.Errorf("expected output to contain %q, got: %q", Version, out)
	}
	if strings.Contains(out, "specs version") {
		t.Errorf("expected plain output without 'specs version' prefix, got: %q", out)
	}
}

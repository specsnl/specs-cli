package cmd

import (
	"strings"
	"testing"
)

// The version is the product of the command: it stays on stdout so that
// $(specs version) keeps working.
func TestVersion_PrintsVersionToStdout(t *testing.T) {
	stdout, stderr, err := executeCmdStreams("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stdout, "specs version") {
		t.Errorf("expected stdout to contain 'specs version', got: %q", stdout)
	}

	if stderr != "" {
		t.Errorf("expected nothing on stderr, got: %q", stderr)
	}
}

func TestVersion_JSONOutput(t *testing.T) {
	stdout, stderr, err := executeCmdStreams("version", "--output=json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{"version":"` + Version + `"}`
	if strings.TrimSpace(stdout) != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}

	if stderr != "" {
		t.Errorf("expected nothing on stderr, got: %q", stderr)
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

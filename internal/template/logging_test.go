package template_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

// TestExecute_SummaryLogIsDebugLevel guards against the "template execution complete"
// summary leaking onto normal runs. It is a diagnostic log and must only surface at
// Debug level (--debug); at the default Info level it must stay silent so it does not
// print an unstyled line to the terminal.
func TestExecute_SummaryLogIsDebugLevel(t *testing.T) {
	root := buildTemplate(t, "Name: World\n", map[string][]byte{
		"hello.txt": []byte("Hello {{ .Name }}"),
	})

	runExecute := func(t *testing.T, level slog.Level) string {
		t.Helper()

		var buf bytes.Buffer

		prev := slog.Default()

		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level})))
		t.Cleanup(func() { slog.SetDefault(prev) })

		tmpl, err := pkgtemplate.Get(root, pkgtemplate.Config{})
		if err != nil {
			t.Fatalf("Get: %v", err)
		}

		if err := tmpl.Execute(t.TempDir()); err != nil {
			t.Fatalf("Execute: %v", err)
		}

		return buf.String()
	}

	// At Info level (the default, no --debug) the summary must NOT surface.
	if out := runExecute(t, slog.LevelInfo); strings.Contains(out, "template execution complete") {
		t.Errorf("summary log leaked at Info level:\n%s", out)
	}

	// At Debug level (--debug) it must still be available, with its attributes.
	out := runExecute(t, slog.LevelDebug)
	if !strings.Contains(out, "template execution complete") {
		t.Fatalf("summary log missing at Debug level:\n%s", out)
	}

	for _, attr := range []string{"rendered=", "verbatim=", "skipped="} {
		if !strings.Contains(out, attr) {
			t.Errorf("summary log missing attribute %q at Debug level:\n%s", attr, out)
		}
	}
}

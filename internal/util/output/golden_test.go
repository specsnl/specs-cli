package output_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the .golden files from the current output")

// Rendering depends on the environment lipgloss is given, so every golden case
// names one of these two rather than inheriting the runner's. That is what makes
// the files reproducible under a terminal and in CI alike.
var (
	// The CI-log rendering: nothing in the environment for colour detection to
	// enable, so every sequence is stripped.
	goldenPlain = []string{}

	// Colour forced on regardless of where the test runs. COLORTERM=truecolor
	// short-circuits colorprofile.Detect before its terminfo lookup, which reads
	// the system terminfo database and would answer differently on a developer's
	// machine than in the go-builder container.
	goldenColour = []string{"CLICOLOR_FORCE=1", "TERM=xterm", "COLORTERM=truecolor"}
)

// assertGolden compares got against testdata/<name>.golden, rewriting the file
// instead when -update is passed. Regenerate them all with `task test:update`
// and review the diff.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata: %v", err)
		}

		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v (run `task test:update` to create it)", path, err)
	}

	if got == string(want) {
		return
	}

	t.Errorf("output does not match %s\n%s\nrun `task test:update` to accept the new output",
		path, diff(string(want), got))
}

// diff renders the first differing line of want and got quoted, so escape
// sequences and trailing whitespace are visible.
func diff(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")

	for i := range max(len(wantLines), len(gotLines)) {
		w, g := lineAt(wantLines, i), lineAt(gotLines, i)
		if w == g {
			continue
		}

		return fmt.Sprintf("line %d:\n  want: %q\n  got:  %q", i+1, w, g)
	}

	return fmt.Sprintf("  want: %q\n  got:  %q", want, got)
}

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}

	return "<missing>"
}

package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/output"
)

func TestHumanWriter_Info_NonEmpty(t *testing.T) {
	var buf bytes.Buffer
	w := output.NewHumanWriter(&buf, &bytes.Buffer{})
	w.Info("hello %s", "world")
	if buf.Len() == 0 {
		t.Error("HumanWriter.Info produced no output")
	}
}

func TestHumanWriter_Warn_NonEmpty(t *testing.T) {
	var errBuf bytes.Buffer
	w := output.NewHumanWriter(&bytes.Buffer{}, &errBuf)
	w.Warn("something wrong")
	if errBuf.Len() == 0 {
		t.Error("HumanWriter.Warn produced no output")
	}
}

func TestHumanWriter_Error_NonEmpty(t *testing.T) {
	var errBuf bytes.Buffer
	w := output.NewHumanWriter(&bytes.Buffer{}, &errBuf)
	w.Error("fatal error")
	if errBuf.Len() == 0 {
		t.Error("HumanWriter.Error produced no output")
	}
}

func TestJSONWriter_Info(t *testing.T) {
	var buf bytes.Buffer
	w := output.NewJSONWriter(&buf, &bytes.Buffer{})
	w.Info("hello %s", "world")
	out := buf.String()
	if !strings.Contains(out, `"level":"info"`) {
		t.Errorf("JSONWriter.Info missing level field, got: %q", out)
	}
	if !strings.Contains(out, `"message":"hello world"`) {
		t.Errorf("JSONWriter.Info missing message field, got: %q", out)
	}
}

func TestJSONWriter_Warn(t *testing.T) {
	var errBuf bytes.Buffer
	w := output.NewJSONWriter(&bytes.Buffer{}, &errBuf)
	w.Warn("something wrong")
	out := errBuf.String()
	if !strings.Contains(out, `"level":"warn"`) {
		t.Errorf("JSONWriter.Warn missing level field, got: %q", out)
	}
}

func TestJSONWriter_Error(t *testing.T) {
	var errBuf bytes.Buffer
	w := output.NewJSONWriter(&bytes.Buffer{}, &errBuf)
	w.Error("fatal error")
	out := errBuf.String()
	if !strings.Contains(out, `"level":"error"`) {
		t.Errorf("JSONWriter.Error missing level field, got: %q", out)
	}
}

func TestJSONWriter_Table(t *testing.T) {
	var buf bytes.Buffer
	w := output.NewJSONWriter(&buf, &bytes.Buffer{})
	w.Table(
		[]string{"Name", "Version"},
		[][]string{{"my-tpl", "1.0.0"}, {"other", "2.0.0"}},
	)
	out := buf.String()
	if !strings.Contains(out, `"Name":"my-tpl"`) {
		t.Errorf("JSONWriter.Table missing Name field, got: %q", out)
	}
	if !strings.Contains(out, `"Version":"1.0.0"`) {
		t.Errorf("JSONWriter.Table missing Version field, got: %q", out)
	}
}

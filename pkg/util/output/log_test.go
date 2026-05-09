package output_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/pkg/specs"
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

func TestJSONWriter_WriteErr_WithKnownSentinel(t *testing.T) {
	var errBuf bytes.Buffer
	w := output.NewJSONWriter(&bytes.Buffer{}, &errBuf)
	w.WriteErr(specs.ErrTemplateNotFound)
	out := errBuf.String()
	if !strings.Contains(out, `"level":"error"`) {
		t.Errorf("JSONWriter.WriteErr missing level field, got: %q", out)
	}
	if !strings.Contains(out, `"error_kind":"template_not_found"`) {
		t.Errorf("JSONWriter.WriteErr missing error_kind field, got: %q", out)
	}
	if !strings.Contains(out, `"message"`) {
		t.Errorf("JSONWriter.WriteErr missing message field, got: %q", out)
	}
}

func TestJSONWriter_WriteErr_UnknownError(t *testing.T) {
	var errBuf bytes.Buffer
	w := output.NewJSONWriter(&bytes.Buffer{}, &errBuf)
	w.WriteErr(errors.New("something unexpected"))
	out := errBuf.String()
	if strings.Contains(out, `"error_kind"`) {
		t.Errorf("JSONWriter.WriteErr should not include error_kind for unknown error, got: %q", out)
	}
}

func TestHumanWriter_WriteErr_NonEmpty(t *testing.T) {
	var errBuf bytes.Buffer
	w := output.NewHumanWriter(&bytes.Buffer{}, &errBuf)
	w.WriteErr(specs.ErrTemplateNotFound)
	if errBuf.Len() == 0 {
		t.Error("HumanWriter.WriteErr produced no output")
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

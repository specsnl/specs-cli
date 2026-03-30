package output

import (
	"fmt"
	"io"
	"os"

	"charm.land/lipgloss/v2"
)

var (
	styleInfo  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(12)) // bright blue
	styleWarn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(11)) // bright yellow
	styleError = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(9))  // bright red
	styleDebug = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))             // dark grey
)

var debugEnabled bool

func SetDebug(enabled bool) { debugEnabled = enabled }

func Info(format string, a ...any) {
	logTo(os.Stdout, styleInfo.Render("info")+" ", format, a...)
}

func Warn(format string, a ...any) {
	logTo(os.Stderr, styleWarn.Render("warn")+" ", format, a...)
}

func Error(format string, a ...any) {
	logTo(os.Stderr, styleError.Render("error")+" ", format, a...)
}

func Debug(format string, a ...any) {
	if !debugEnabled {
		return
	}
	logTo(os.Stdout, styleDebug.Render("debug")+" ", format, a...)
}

func logTo(w io.Writer, prefix, format string, a ...any) {
	lipgloss.Fprintln(w, fmt.Sprintf(prefix+format, a...))
}

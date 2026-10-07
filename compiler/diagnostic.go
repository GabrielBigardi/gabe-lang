package compiler

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Span is a half-open range of byte offsets in a UTF-8 source file.
type Span struct{ Start, End int }

func (s Span) join(other Span) Span { return Span{min(s.Start, other.Start), max(s.End, other.End)} }

// Diagnostic reports a source error without panicking.
type Diagnostic struct {
	Message string
	Span    Span
}

func diagnostic(span Span, message string) *Diagnostic {
	return &Diagnostic{Message: message, Span: span}
}
func (d *Diagnostic) Error() string { return d.Message }

// Render formats a diagnostic with its source line and Unicode character column.
func (d *Diagnostic) Render(path, source string) string {
	start := max(0, min(d.Span.Start, len(source)))
	prefix := source[:start]
	lineNumber := strings.Count(prefix, "\n") + 1
	lineStart := strings.LastIndex(prefix, "\n") + 1
	column := utf8.RuneCountInString(source[lineStart:start]) + 1
	line := strings.SplitN(source[lineStart:], "\n", 2)[0]
	line = strings.TrimSuffix(line, "\r")
	end := min(d.Span.End, lineStart+len(line))
	width := 1
	if end > start {
		width = max(1, utf8.RuneCountInString(source[start:end]))
	}
	return fmt.Sprintf("%s:%d:%d: error: %s\n  %s\n  %s%s", path, lineNumber, column, d.Message, line, strings.Repeat(" ", column-1), strings.Repeat("^", width))
}

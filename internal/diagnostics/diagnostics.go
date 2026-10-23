package diagnostics

import (
	"fmt"
	"strings"
)

// Position identifies a byte offset and a 1-based line/column in source text.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span identifies a half-open range [Start, End) in source text.
type Span struct {
	Start Position
	End   Position
}

// Severity describes the impact of a diagnostic.
type Severity uint8

const (
	Error Severity = iota
	Warning
)

func (s Severity) String() string {
	if s == Warning {
		return "warning"
	}
	return "error"
}

// Diagnostic is a source-located compiler message.
type Diagnostic struct {
	Severity Severity
	Span     Span
	Message  string
}

func (d Diagnostic) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", d.Span.Start.Line, d.Span.Start.Column, d.Severity, d.Message)
}

// Diagnostics is a collection of compiler messages.
type Diagnostics []Diagnostic

func (d Diagnostics) Error() string {
	if len(d) == 0 {
		return ""
	}
	parts := make([]string, len(d))
	for i, diagnostic := range d {
		parts[i] = diagnostic.Error()
	}
	return strings.Join(parts, "\n")
}

func (d Diagnostics) HasErrors() bool {
	for _, diagnostic := range d {
		if diagnostic.Severity == Error {
			return true
		}
	}
	return false
}

func (d *Diagnostics) Add(severity Severity, span Span, message string) {
	*d = append(*d, Diagnostic{Severity: severity, Span: span, Message: message})
}

func (d *Diagnostics) Errorf(span Span, format string, args ...any) {
	d.Add(Error, span, fmt.Sprintf(format, args...))
}

func (d *Diagnostics) Warningf(span Span, format string, args ...any) {
	d.Add(Warning, span, fmt.Sprintf(format, args...))
}

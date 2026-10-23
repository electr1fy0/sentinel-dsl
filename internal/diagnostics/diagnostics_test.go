package diagnostics

import (
	"testing"
)

func TestSeverityString(t *testing.T) {
	if Warning.String() != "warning" {
		t.Fatalf("expected 'warning', got %q", Warning.String())
	}
	if Error.String() != "error" {
		t.Fatalf("expected 'error', got %q", Error.String())
	}
}

func TestDiagnostics(t *testing.T) {
	var d Diagnostics
	if d.Error() != "" {
		t.Fatalf("expected empty error string, got %q", d.Error())
	}
	if d.HasErrors() {
		t.Fatalf("expected no errors on empty diagnostics")
	}

	span := Span{
		Start: Position{Offset: 0, Line: 1, Column: 2},
		End:   Position{Offset: 5, Line: 1, Column: 7},
	}

	d.Warningf(span, "check %s", "warning")
	if d.HasErrors() {
		t.Fatalf("expected HasErrors to be false with only warnings")
	}
	if d.Error() != "1:2: warning: check warning" {
		t.Fatalf("unexpected warning format: %q", d.Error())
	}

	d.Errorf(span, "invalid %d", 42)
	if !d.HasErrors() {
		t.Fatalf("expected HasErrors to be true after adding error")
	}

	diag := Diagnostic{
		Severity: Error,
		Span:     span,
		Message:  "manual",
	}
	if diag.Error() != "1:2: error: manual" {
		t.Fatalf("unexpected Diagnostic.Error format: %q", diag.Error())
	}

	d.Add(Error, span, "added")
	expected := "1:2: warning: check warning\n1:2: error: invalid 42\n1:2: error: added"
	if d.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, d.Error())
	}
}

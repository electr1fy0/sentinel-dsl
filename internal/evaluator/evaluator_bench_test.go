package evaluator

import (
	"testing"

	"github.com/ayush/sentinel-dsl/internal/parser"
)

func BenchmarkEvaluate(b *testing.B) {
	policy, diagnostics := parser.Parse(`policy api { rule read when method == "GET" and path matches "^/api/[0-9]+$" then allow; default deny; }`)
	if diagnostics.HasErrors() {
		b.Fatal(diagnostics)
	}
	request := Request{Method: "GET", Path: "/api/42"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Evaluate(policy, request)
	}
}

package middleware

import (
	"fmt"
	"net/http"

	"github.com/ayush/sentinel-dsl/internal/ast"
	"github.com/ayush/sentinel-dsl/internal/evaluator"
)

type DeniedHandler func(http.ResponseWriter, *http.Request, evaluator.Result)

type Option func(*Middleware)

func WithDeniedHandler(handler DeniedHandler) Option { return func(m *Middleware) { m.denied = handler } }

// Middleware evaluates a policy before dispatching to the wrapped handler.
type Middleware struct {
	Policy *ast.Policy
	denied DeniedHandler
}

func New(policy *ast.Policy, options ...Option) *Middleware {
	m := &Middleware{Policy: policy}
	for _, option := range options { option(m) }
	return m
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	if next == nil { next = http.NotFoundHandler() }
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := evaluator.Evaluate(m.Policy, evaluator.RequestFromHTTP(r))
		if result.Allowed { next.ServeHTTP(w, r); return }
		if m.denied != nil { m.denied(w, r, result); return }
		status := result.Action.Status
		if status == 0 { status = http.StatusForbidden }
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		message := result.Action.Message
		if message == "" { message = "request denied" }
		_, _ = fmt.Fprintln(w, message)
	})
}

// Wrap is convenient for the common middleware(policy, next) form.
func Wrap(next http.Handler, policy *ast.Policy, options ...Option) http.Handler { return New(policy, options...).Handler(next) }

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ayush/sentinel-dsl/internal/evaluator"
	"github.com/ayush/sentinel-dsl/internal/parser"
)

func TestWrapAllowsAndDenies(t *testing.T) {
	policy, diagnostics := parser.Parse(`policy api { rule get when method == "GET" then allow default deny 401 "unauthorized" }`)
	if diagnostics.HasErrors() {
		t.Fatalf("parse diagnostics: %v", diagnostics)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := Wrap(next, policy)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("allow status = %d", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || response.Body.String() != "unauthorized\n" {
		t.Fatalf("deny = %d %q", response.Code, response.Body.String())
	}
}

func TestMiddlewareDefaultsAndCustomDenied(t *testing.T) {
	// Policy with default deny without status (status == 0) and without message (message == "")
	policyDefault, diagnostics := parser.Parse(`policy api { default deny }`)
	if diagnostics.HasErrors() {
		t.Fatalf("parse diagnostics: %v", diagnostics)
	}

	// 1. Test nil next handler falls back to http.NotFoundHandler() when allowed
	policyAllow, _ := parser.Parse(`policy api { default allow }`)
	mAllow := New(policyAllow)
	hNilNext := mAllow.Handler(nil)
	req := httptest.NewRequest(http.MethodGet, "/foo", nil)
	rec := httptest.NewRecorder()
	hNilNext.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from nil next handler, got %d", rec.Code)
	}

	// 2. Test status 0 -> 403 and empty message -> "request denied\n"
	mDefault := New(policyDefault)
	rec2 := httptest.NewRecorder()
	mDefault.Handler(nil).ServeHTTP(rec2, req)
	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec2.Code)
	}
	if strings.TrimSpace(rec2.Body.String()) != "request denied" {
		t.Fatalf("expected 'request denied', got %q", rec2.Body.String())
	}

	// 3. Test WithDeniedHandler custom option
	customDeniedCalled := false
	customOpt := WithDeniedHandler(func(w http.ResponseWriter, r *http.Request, res evaluator.Result) {
		customDeniedCalled = true
		w.WriteHeader(http.StatusTeapot)
	})
	mCustom := New(policyDefault, customOpt)
	rec3 := httptest.NewRecorder()
	mCustom.Handler(nil).ServeHTTP(rec3, req)
	if !customDeniedCalled || rec3.Code != http.StatusTeapot {
		t.Fatalf("custom denied handler not called or wrong status: %d", rec3.Code)
	}
}

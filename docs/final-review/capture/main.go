// Capture real API responses without opening a network port. This helper uses
// the unchanged production handler through net/http/httptest.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ayush/sentinel-dsl/internal/api"
)

type sample struct {
	ID, Title, Endpoint, Source, Note string
	Payload                           map[string]any
	ExpectedValid                     bool
	ExpectedDecision                  string
}

func main() {
	root := "docs/final-review"
	out := filepath.Join(root, "evidence")
	must(os.MkdirAll(out, 0755))
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(root, "fixtures", name))
		must(err)
		return string(b)
	}
	var samples []sample
	for _, kind := range []string{"syntax", "semantic"} {
		for _, state := range []string{"error", "recovered"} {
			src := read(kind + "-" + state + ".sntl")
			samples = append(samples, sample{
				ID: kind + "-" + state, Title: strings.ToUpper(kind[:1]) + kind[1:] + " / " + state,
				Endpoint: "/api/validate", Source: src, Payload: map[string]any{"source": src}, ExpectedValid: state == "recovered",
				Note: map[string]string{"error": "Invalid source is diagnosed. It is not executed.", "recovered": "The author corrects the source and submits it again."}[state],
			})
		}
	}
	for _, state := range []string{"error", "recovered"} {
		pattern := "(a|b"
		if state == "recovered" {
			pattern = "(a|b)*"
		}
		samples = append(samples, sample{
			ID: "automata-" + state, Title: "Regular expression / " + state, Endpoint: "/api/automata",
			Source: "pattern: " + pattern + "\ninput:   abba", Payload: map[string]any{"pattern": pattern, "input": "abba"}, ExpectedValid: state == "recovered",
			Note: "The educational regex compiler supports literals, grouping, alternation, *, + and ?.",
		})
	}
	policy := read("review-policy.sntl")
	for _, c := range []struct{ file, decision string }{{"allowed", "ALLOW"}, {"denied", "DENY"}, {"default-denied", "DENY"}} {
		var request map[string]any
		must(json.Unmarshal([]byte(read(c.file+".json")), &request))
		samples = append(samples, sample{ID: c.file, Title: "Policy decision / " + c.file, Endpoint: "/api/explain", Source: policy,
			Payload: map[string]any{"source": policy, "request": request}, ExpectedValid: true, ExpectedDecision: c.decision,
			Note: "The decision and trace below come from the same handler used by Sentinel Studio."})
	}
	page := template.Must(template.New("evidence").Parse(htmlPage))
	var all []map[string]any
	for _, s := range samples {
		payload, err := json.Marshal(s.Payload)
		must(err)
		req := httptest.NewRequest("POST", s.Endpoint, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		api.Handler().ServeHTTP(response, req)
		if response.Code != 200 {
			panic(fmt.Sprintf("%s: HTTP %d: %s", s.ID, response.Code, response.Body.String()))
		}
		var parsed map[string]any
		must(json.Unmarshal(response.Body.Bytes(), &parsed))
		if parsed["valid"] != s.ExpectedValid {
			panic(fmt.Sprintf("%s: unexpected validity", s.ID))
		}
		if s.ExpectedDecision != "" && parsed["decision"].(map[string]any)["decision"] != s.ExpectedDecision {
			panic(s.ID + ": unexpected decision")
		}
		if s.ID == "automata-recovered" && parsed["accepted"] != true {
			panic("regex correction did not accept abba")
		}
		if s.ID == "syntax-error" {
			p := parsed["policy"].(map[string]any)
			if p["rule_count"] != float64(1) || len(parsed["diagnostics"].([]any)) != 2 {
				panic("parser synchronization evidence is incomplete")
			}
		}
		pretty, err := json.MarshalIndent(parsed, "", "  ")
		must(err)
		must(os.WriteFile(filepath.Join(out, s.ID+".json"), append(pretty, '\n'), 0644))
		must(os.WriteFile(filepath.Join(out, s.ID+"-request.json"), append(payload, '\n'), 0644))
		var lines []string
		for i, line := range strings.Split(strings.TrimSpace(s.Source), "\n") {
			lines = append(lines, fmt.Sprintf("%2d  %s", i+1, line))
		}
		var summary []string
		summary = append(summary, fmt.Sprintf("HTTP %d  |  valid: %v", response.Code, parsed["valid"]))
		if ds, ok := parsed["diagnostics"].([]any); ok {
			for _, d := range ds {
				item := d.(map[string]any)
				start := item["start"].(map[string]any)
				summary = append(summary, fmt.Sprintf("%v:%v  %v", start["line"], start["column"], item["message"]))
			}
			if len(ds) == 0 {
				summary = append(summary, "Diagnostics: none")
			}
		}
		if p, ok := parsed["policy"].(map[string]any); ok {
			summary = append(summary, fmt.Sprintf("Policy: %v  |  rule count: %v", p["name"], p["rule_count"]))
			if rules, ok := p["rules"].([]any); ok {
				for _, r := range rules {
					summary = append(summary, fmt.Sprintf("Retained rule: %v", r.(map[string]any)["name"]))
				}
			}
		}
		if v, ok := parsed["accepted"]; ok {
			summary = append(summary, fmt.Sprintf("Accepted: %v", v))
		}
		if decision, ok := parsed["decision"].(map[string]any); ok {
			j, _ := json.MarshalIndent(decision, "", "  ")
			summary = append(summary, string(j))
		}
		if s.ID == "automata-recovered" {
			for _, name := range []string{"nfa", "dfa", "minimized"} {
				g := parsed[name].(map[string]any)
				summary = append(summary, fmt.Sprintf("%s states: %d", name, len(g["nodes"].([]any))))
			}
		}
		f, err := os.Create(filepath.Join(out, s.ID+".html"))
		must(err)
		must(page.Execute(f, map[string]any{"Title": s.Title, "Source": strings.Join(lines, "\n"), "Summary": strings.Join(summary, "\n\n"), "Endpoint": s.Endpoint, "Valid": s.ExpectedValid, "Note": s.Note, "Date": time.Now().UTC().Format("02 Jan 2006"), "JSON": s.ID + ".json"}))
		must(f.Close())
		all = append(all, map[string]any{"id": s.ID, "endpoint": s.Endpoint, "status": response.Code, "valid": parsed["valid"], "assertions": "PASS"})
		fmt.Printf("PASS %-22s %s\n", s.ID, s.Endpoint)
	}
	b, _ := json.MarshalIndent(all, "", "  ")
	must(os.WriteFile(filepath.Join(out, "summary.json"), append(b, '\n'), 0644))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

const htmlPage = `<!doctype html><html lang="en"><meta charset="utf-8"><title>SentinelDSL - {{.Title}}</title><style>
*{box-sizing:border-box}body{margin:0;background:#f2f5f8;color:#162b42;font:18px -apple-system,BlinkMacSystemFont,sans-serif;padding:38px}main{max-width:1380px;margin:auto}header{display:flex;justify-content:space-between;align-items:center;border-bottom:2px solid #203d58;padding-bottom:20px}.brand{font-weight:750;letter-spacing:2px;font-size:17px}.date{font-size:15px;color:#5b6d7e}h1{font-size:36px;margin:28px 0 12px;letter-spacing:-1px}.badge{display:inline-block;border-radius:6px;padding:7px 11px;font-size:14px;font-weight:700;letter-spacing:1px;background:{{if .Valid}}#d7ede5;color:#136646{{else}}#f9e0df;color:#9c2929{{end}}}article{display:grid;grid-template-columns:1.1fr 1fr;gap:24px;margin-top:24px}section{background:white;border:1px solid #d4dee7;border-radius:10px;overflow:hidden}h2{font-size:14px;letter-spacing:1.4px;padding:17px 22px;margin:0;background:#e9eff4;border-bottom:1px solid #d4dee7}pre{white-space:pre-wrap;overflow-wrap:anywhere;margin:0;padding:24px 20px;font:16px/1.65 ui-monospace,SFMono-Regular,Menlo,monospace;min-height:330px}p{font-size:17px;line-height:1.55;color:#4c6075}footer{border-top:1px solid #cbd7e1;margin-top:26px;padding-top:18px;font-size:14px;color:#637386}code{font-family:ui-monospace,Menlo,monospace}
</style><main><header><span class="brand">SENTINEL DSL / EXECUTION EVIDENCE</span><span class="date">{{.Date}}</span></header><h1>{{.Title}}</h1><span class="badge">{{if .Valid}}VALID INPUT{{else}}ERROR DIAGNOSED{{end}}</span><p>{{.Note}}</p><article><section><h2>INPUT SEGMENT</h2><pre>{{.Source}}</pre></section><section><h2>CAPTURED HANDLER OUTPUT</h2><pre>{{.Summary}}</pre></section></article><footer>POST <code>{{.Endpoint}}</code> / actual Go handler via <code>net/http/httptest</code> / no mocked responses<br>Offline evidence view. Full unedited JSON: <code>{{.JSON}}</code></footer></main></html>`

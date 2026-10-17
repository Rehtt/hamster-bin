package tasks

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/repository"
)

type traceParser struct{ fail atomic.Bool }

func (p *traceParser) GetName() string                             { return "trace" }
func (p *traceParser) CanParse(string) bool                        { return true }
func (p *traceParser) Parse(string) (*parser.ComponentInfo, error) { panic("missing options") }
func (p *traceParser) ParseWithOptions(code string, o parser.ParseOptions) (*parser.ComponentInfo, error) {
	*o.Trace = parser.ParseTrace{Mode: o.Mode(), FinalSource: "test_source", Fallback: true, FallbackReason: "test fallback", Calls: []parser.ParseCall{{Tool: "test_source", Arguments: map[string]string{"code": code}, Endpoint: "https://example.com/product", Status: "success"}}}
	if p.fail.Load() {
		return &parser.ComponentInfo{Name: "must not be persisted"}, parser.ErrParserContent
	}
	return &parser.ComponentInfo{Name: "resistor"}, nil
}
func TestTaskModesTraceFailureAndRetryPersistence(t *testing.T) {
	for _, kind := range []string{ComponentParse, QRCodeParse} {
		t.Run(kind, func(t *testing.T) {
			s := testService(t, 1, time.Second)
			p := &traceParser{}
			p.fail.Store(true)
			s.manager = parser.NewParserManager()
			s.manager.Register(p)
			task, err := s.Submit(Request{Type: kind, Input: Input{Code: "C123", QRCodeData: "{pc:C123,qty:2}", UseLLM: true, ParseMode: "agent"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			start(t, s)
			done := waitTask(t, s, task.ID, repository.TaskFailed)
			var failed Result
			if err := json.Unmarshal(done.Result, &failed); err != nil {
				t.Fatal(err)
			}
			if failed.Component != nil || len(failed.Items) > 0 || failed.Quantity != 0 || failed.QRCodeInfo != nil || failed.ParseTrace == nil || failed.ParseTrace.Mode != "agent" || len(failed.ParseTrace.Calls) != 1 || !failed.ParseTrace.Fallback {
				t.Fatalf("failed result %s", done.Result)
			}
			p.fail.Store(false)
			retry, err := s.Retry(task.ID)
			if err != nil {
				t.Fatal(err)
			}
			var input Input
			if err := json.Unmarshal(retry.Input, &input); err != nil {
				t.Fatal(err)
			}
			if input.ParseMode != "agent" || !input.UseLLM || retry.RetryOfID == nil || *retry.RetryOfID != task.ID {
				t.Fatal(retry)
			}
			succeeded := waitTask(t, s, retry.ID, repository.TaskSucceeded)
			var result Result
			if err := json.Unmarshal(succeeded.Result, &result); err != nil {
				t.Fatal(err)
			}
			if result.Component == nil || result.ParseTrace == nil || result.ParseTrace.Mode != "agent" || result.ParseTrace.FinalSource != "test_source" {
				t.Fatal(string(succeeded.Result))
			}
			if kind == QRCodeParse && result.Quantity != 2 {
				t.Fatal("QR quantity", result.Quantity)
			}
		})
	}
}

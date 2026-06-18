package parser

import (
	"context"
	"strings"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/llm"
)

type fakeCompleter struct {
	called bool
	result string
}

func TestLCSCParserUseLLMRequiresConfiguredClient(t *testing.T) {
	parser := NewLCSCParser()
	_, err := parser.ParseWithOptions("C123", ParseOptions{UseLLM: true})
	if err != llm.ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func (f *fakeCompleter) CompleteJSON(ctx context.Context, messages []llm.Message) (string, error) {
	f.called = true
	return f.result, nil
}

func (f *fakeCompleter) Configured() bool {
	return true
}

func TestLCSCParserEnrichWithLLM(t *testing.T) {
	completer := &fakeCompleter{
		result: `{"name":"电阻","model":"RC0603FR-0710KL","value":"10kΩ","package":"0603","description":"厚膜电阻 1%","manufacturer":"YAGEO","datasheet_url":"https://example.com/ds.pdf","image_url":"https://example.com/image.jpg"}`,
	}
	parser := NewLCSCParser(completer)
	info := &ComponentInfo{
		PlatformCode: "C123",
		PlatformName: "立创商城",
		PlatformURL:  "https://example.com/C123",
	}

	if err := parser.enrichWithLLM(context.Background(), info, "10k 0603 resistor"); err != nil {
		t.Fatal(err)
	}
	if !completer.called {
		t.Fatal("expected LLM completer to be called")
	}
	if info.Name != "电阻" || info.Value != "10kΩ" || info.Package != "0603" {
		t.Fatalf("unexpected enriched info: %#v", info)
	}
	if info.PlatformCode != "C123" || info.PlatformName != "立创商城" {
		t.Fatalf("platform fields should be preserved: %#v", info)
	}
}

func TestLCSCParserEnrichWithLLMPreservesExistingOnEmptyFields(t *testing.T) {
	completer := &fakeCompleter{
		result: `{"name":"","model":"","value":"22uF","package":"","description":"","manufacturer":"","datasheet_url":"","image_url":""}`,
	}
	parser := NewLCSCParser(completer)
	info := &ComponentInfo{
		Name:    "电容",
		Model:   "CL10A226MQ8NRNC",
		Package: "0603",
	}

	if err := parser.enrichWithLLM(context.Background(), info, strings.Repeat("x", 10)); err != nil {
		t.Fatal(err)
	}
	if info.Name != "电容" || info.Model != "CL10A226MQ8NRNC" || info.Package != "0603" {
		t.Fatalf("empty LLM fields should not overwrite existing values: %#v", info)
	}
	if info.Value != "22uF" {
		t.Fatalf("expected value to be updated, got %s", info.Value)
	}
}

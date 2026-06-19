package parser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/Rehtt/hamster-bin/internal/llm"
)

type fakeCompleter struct {
	called bool
	result string
}

func TestLCSCParserUseLLMRequiresConfiguredClient(t *testing.T) {
	parser := NewLCSCParser()
	_, err := parser.ParseWithOptions("C123", ParseOptions{UseLLM: true})
	if !errors.Is(err, llm.ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestParseLCSCDetailDocumentSetsCategoryName(t *testing.T) {
	html := `
		<div class="BaseInfo_component-info__yuOgz">
			<h1 class="BaseInfo_component-name__7OSgG">10kΩ 1% 0603</h1>
			<dl>
				<div><dt>品牌名称</dt><dd>YAGEO</dd></div>
				<div><dt>厂家型号</dt><dd>RC0603FR-0710KL</dd></div>
				<div><dt>商品封装</dt><dd>0603</dd></div>
			</dl>
		</div>
		<table class="GoodsParameter_table__VYg5o">
			<tbody><tr><td>商品目录</td><td>电阻</td></tr></tbody>
		</table>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	info, err := parseLCSCDetailDocument(doc, "C123", "https://example.com/C123")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "10kΩ 1% 0603" || info.CategoryName != "电阻" {
		t.Fatalf("expected name from 商品页标题 and category_name from 商品目录, got %#v", info)
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
		result: `{"name":"电阻","category_name":"电阻","model":"RC0603FR-0710KL","value":"10kΩ","package":"0603","description":"厚膜电阻 1%","manufacturer":"YAGEO","datasheet_url":"https://example.com/ds.pdf","image_url":"https://example.com/image.jpg"}`,
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
	if info.CategoryName != "电阻" {
		t.Fatalf("expected category_name to be enriched, got %s", info.CategoryName)
	}
	if info.PlatformCode != "C123" || info.PlatformName != "立创商城" {
		t.Fatalf("platform fields should be preserved: %#v", info)
	}
}

func TestLCSCParserEnrichWithLLMPreservesExistingOnEmptyFields(t *testing.T) {
	completer := &fakeCompleter{
		result: `{"name":"","category_name":"","model":"","value":"22uF","package":"","description":"","manufacturer":"","datasheet_url":"","image_url":""}`,
	}
	parser := NewLCSCParser(completer)
	info := &ComponentInfo{
		Name:         "电容",
		CategoryName: "电容",
		Model:        "CL10A226MQ8NRNC",
		Package:      "0603",
	}

	if err := parser.enrichWithLLM(context.Background(), info, strings.Repeat("x", 10)); err != nil {
		t.Fatal(err)
	}
	if info.Name != "电容" || info.CategoryName != "电容" || info.Model != "CL10A226MQ8NRNC" || info.Package != "0603" {
		t.Fatalf("empty LLM fields should not overwrite existing values: %#v", info)
	}
	if info.Value != "22uF" {
		t.Fatalf("expected value to be updated, got %s", info.Value)
	}
}

package parser

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestParseLCSCDetailDocumentMissingContainer(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<html><body></body></html>`))
	if err != nil {
		t.Fatal(err)
	}

	_, err = parseLCSCDetailDocument(doc, "C123", "https://example.com/C123")
	if !errors.Is(err, ErrParserContent) {
		t.Fatalf("expected ErrParserContent, got %v", err)
	}
}

func TestParseLCSCDetailDocumentMissingName(t *testing.T) {
	html := `
		<div class="BaseInfo_component-info__yuOgz">
			<h1 class="BaseInfo_component-name__7OSgG"></h1>
		</div>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}

	_, err = parseLCSCDetailDocument(doc, "C123", "https://example.com/C123")
	if !errors.Is(err, ErrParserContent) {
		t.Fatalf("expected ErrParserContent, got %v", err)
	}
}

func TestLCSCParserEnrichWithLLMInvalidJSON(t *testing.T) {
	completer := &fakeCompleter{result: "not-json"}
	parser := NewLCSCParser(completer)
	info := &ComponentInfo{PlatformCode: "C123"}

	err := parser.enrichWithLLM(context.Background(), info, "page text")
	if !errors.Is(err, ErrParserContent) {
		t.Fatalf("expected ErrParserContent, got %v", err)
	}
}

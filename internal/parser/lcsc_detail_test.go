package parser

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/llm"
)

const detailFixture = `{"code":200,"result":{"productCode":"C25804","productNameEn":"RES 10kΩ","title":"Title","productModel":"0603WAF1002T5E","brandNameEn":"UNI-ROYAL","encapStandard":"0603","catalogName":"Chip Resistor","productIntroEn":"Thick Film Resistor","productImages":["", "javascript:bad", "//assets.lcsc.com/resistor.jpg"],"pdfUrl":"https://datasheet.lcsc.com/resistor.pdf","stockNumber":0,"currencyType":"USD","productPriceList":[{"ladder":1000,"productPrice":"0.0020","currencyPrice":0.002},{"ladder":100,"productPrice":"0.0024","currencyPrice":0.0024}],"paramVOList":[{"paramNameEn":"Resistance","paramValueEn":"10kΩ"},{"paramNameEn":"Tolerance","paramValueEn":"±1%"}]}}`
const fallbackHTML = `<div class="BaseInfo_component-info__yuOgz"><h1 class="BaseInfo_component-name__7OSgG">HTML 电阻</h1></div>`

func TestLCSCDetailMapping(t *testing.T) {
	htmlCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ftps/wm/product/detail" {
			htmlCalls++
			t.Error("unexpected HTML request")
		}
		if r.URL.Query().Get("productCode") != "C25804" {
			t.Error("code not normalized")
		}
		if r.Header.Get("Origin") != "https://www.lcsc.com" || r.Header.Get("Referer") != "https://www.lcsc.com/" || r.Header.Get("Accept") != "application/json" || r.Header.Get("User-Agent") == "" {
			t.Error("missing API headers")
		}
		fmt.Fprint(w, detailFixture)
	}))
	defer server.Close()
	info, err := localParser(server).Parse("  c25804 \n")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "RES 10kΩ" || info.Model != "0603WAF1002T5E" || info.Manufacturer != "UNI-ROYAL" || info.Package != "0603" || info.CategoryName != "Chip Resistor" || info.Description != "Thick Film Resistor" || info.Value != "Resistance: 10kΩ; Tolerance: ±1%" || info.ImageURL != "https://assets.lcsc.com/resistor.jpg" || info.DatasheetURL != "https://datasheet.lcsc.com/resistor.pdf" {
		t.Fatalf("mapping: %+v", info)
	}
	quote := info.SupplierQuote
	if quote == nil || quote.StockQuantity == nil || *quote.StockQuantity != 0 || quote.Currency == nil || *quote.Currency != "USD" || len(quote.PriceTiers) != 2 || quote.PriceTiers[0].MinQuantity != 100 || quote.PriceTiers[1].UnitPrice != "0.0020" || info.Price != 0 || htmlCalls != 0 {
		t.Fatalf("quote: %+v", quote)
	}
	queried, err := time.Parse(time.RFC3339, quote.QueriedAt)
	if err != nil || time.Since(queried) > 3*time.Second {
		t.Fatalf("query time: %v %v", queried, err)
	}
}

func TestLCSCDetailOptionalFields(t *testing.T) {
	for _, tc := range []struct{ name, result, wantName, wantCurrency, wantPrice string }{
		{"missing quote", `"title":"Title"`, "Title", "", ""},
		{"model fallback", `"productModel":"Model"`, "Model", "", ""},
		{"currency price", `"productNameEn":"Name","currencyType":"EUR","productPriceList":[{"ladder":1,"currencyPrice":0.000123,"productPrice":"9.9"}]`, "Name", "EUR", "0.000123"},
		{"invalid tiers", `"productNameEn":"Name","stockNumber":-1,"productPriceList":[{"ladder":0,"currencyPrice":1},{"ladder":1,"currencyPrice":"-1"},{"ladder":2,"currencyPrice":null}]`, "Name", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"code":200,"result":{"productCode":"C25804",%s}}`, tc.result)
			}))
			defer server.Close()
			info, err := localParser(server).Parse("C25804")
			if err != nil {
				t.Fatal(err)
			}
			q := info.SupplierQuote
			if info.Name != tc.wantName || q.StockQuantity != nil || q.PriceTiers == nil {
				t.Fatalf("optional values: %+v", info)
			}
			if tc.wantCurrency == "" {
				if q.Currency != nil {
					t.Fatal("unknown currency must be null")
				}
			} else if q.Currency == nil || *q.Currency != tc.wantCurrency {
				t.Fatal("wrong currency")
			}
			if tc.wantPrice == "" {
				if len(q.PriceTiers) != 0 {
					t.Fatal("unexpected prices")
				}
			} else if len(q.PriceTiers) != 1 || q.PriceTiers[0].UnitPrice != tc.wantPrice {
				t.Fatal("wrong price")
			}
		})
	}
}

func TestLCSCDetailFallback(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"no result", `{"code":200,"result":null}`, 200},
		{"business failure", `{"code":500,"result":{}}`, 200},
		{"missing success", `{"result":{"productCode":"C25804","title":"Name"}}`, 200},
		{"wrong code", strings.Replace(detailFixture, "C25804", "C123", 1), 200},
		{"invalid content", `{"code":200,"result":{"productCode":"C25804"}}`, 200},
		{"invalid JSON", `{`, 200},
		{"HTTP error", `failure`, 503},
		{"oversize", strings.Repeat("x", (2<<20)+1), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			htmlCalls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/ftps/") {
					w.WriteHeader(tc.status)
					fmt.Fprint(w, tc.body)
					return
				}
				if r.URL.Path == "/api/eda/product/search" {
					fmt.Fprint(w, `{"code":200,"result":{"productList":[]}}`)
					return
				}
				htmlCalls++
				fmt.Fprint(w, fallbackHTML)
			}))
			defer server.Close()
			info, err := localParser(server).Parse("C25804")
			if err != nil || info.Name != "HTML 电阻" || info.SupplierQuote != nil || htmlCalls != 1 {
				t.Fatalf("fallback: %+v %v", info, err)
			}
		})
	}
}

func TestLCSCDetailBothFailClassification(t *testing.T) {
	for _, status := range []int{200, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/ftps/") {
					w.WriteHeader(502)
					return
				}
				w.WriteHeader(status)
				fmt.Fprint(w, "invalid HTML")
			}))
			defer server.Close()
			_, err := localParser(server).Parse("C25804")
			want := ErrParserContent
			if status != 200 {
				want = ErrParserUpstream
			}
			if !errors.Is(err, want) || !strings.Contains(err.Error(), "详情 API 失败") || !strings.Contains(err.Error(), "HTML 解析失败") {
				t.Fatalf("error: %v", err)
			}
			if status == 200 && errors.Is(err, ErrParserUpstream) {
				t.Fatal("API category leaked into final HTML category")
			}
		})
	}
}

type detailCompleter struct {
	input string
	err   error
}

func (c *detailCompleter) Configured() bool { return true }
func (c *detailCompleter) CompleteJSON(_ context.Context, messages []llm.Message) (string, error) {
	c.input = messages[1].Content
	return `{"name":"AI 电阻","price":123,"supplier_quote":{"stock_quantity":999}}`, c.err
}

func TestLCSCDetailLLM(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, detailFixture) }))
			defer server.Close()
			completer := &detailCompleter{}
			if fail {
				completer.err = errors.New("model unavailable")
			}
			info, err := localParser(server, completer).ParseWithOptions("C25804", ParseOptions{UseLLM: true})
			if calls != 1 || !strings.Contains(completer.input, "paramNameEn") || strings.Contains(completer.input, "supplier_quote") {
				t.Fatalf("model input or fallback: %s", completer.input)
			}
			if fail {
				if !errors.Is(err, ErrParserUpstream) {
					t.Fatal(err)
				}
				return
			}
			if err != nil || info.Name != "AI 电阻" || info.Price != 0 || *info.SupplierQuote.StockQuantity != 0 || info.SupplierQuote.PriceTiers[1].UnitPrice != "0.0020" {
				t.Fatalf("LLM quote isolation: %+v %v", info, err)
			}
		})
	}
}

func TestLCSCDetailNetworkFailureAndClientTimeoutFallback(t *testing.T) {
	for _, apiErr := range []error{errors.New("network failure"), context.DeadlineExceeded} {
		p := NewLCSCParser()
		calls := 0
		p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.Host == "wmsc.lcsc.com" {
				return nil, apiErr
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fallbackHTML)), Header: make(http.Header)}, nil
		})
		info, err := p.Parse("C25804")
		if err != nil || info.Name != "HTML 电阻" || calls != 4 {
			t.Fatalf("network fallback: %+v %v", info, err)
		}
	}
}

func TestLCSCDetailCancelledDoesNotFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := NewLCSCParser()
	calls := 0
	p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) { calls++; return nil, r.Context().Err() })
	_, err := p.ParseWithOptions("C25804", ParseOptions{Context: ctx})
	if !errors.Is(err, context.Canceled) || calls > 1 {
		t.Fatalf("cancel: calls=%d err=%v", calls, err)
	}
}

package parser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const easyEDAFixture = `{"code":200,"success":true,"result":{"productList":[{"code":"C258040","name":"模糊结果"},{"code":"C25804","name":"厚膜电阻","model":"R0603","brandName":"UNI-ROYAL","standard":"0603","catalogName":"贴片电阻","desc":"10kΩ ±1%","image":"invalid<$>https://example.com/1.jpg<$>https://example.com/2.jpg","pdfFileUrl":"/relative.pdf","pdfHtmlUrl":"https://example.com/datasheet","stockNumber":"0","priceList":[{"startNumber":1000,"price":"0.0020"},{"startNumber":"100","price":0.0024}],"device_info":{"description":"device description","attributes":{"Value":"10kΩ","Tolerance":"±1%","Symbol":"drawing","Footprint":"drawing","3D Model":"drawing"},"symbol_info":{"dataStr":"ignored drawing"}}}]}}`

func TestEasyEDAMappingAndFallback(t *testing.T) {
	for _, internationalFail := range []bool{false, true} {
		t.Run(fmt.Sprint(internationalFail), func(t *testing.T) {
			var hosts []string
			p := NewLCSCParser()
			p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) {
				hosts = append(hosts, r.URL.Host)
				if r.URL.Query().Get("keyword") != "C25804" || r.URL.Query().Get("type") != "3" || r.URL.Query().Get("page") != "1" || r.URL.Query().Get("pageSize") != "10" {
					t.Fatal("wrong query", r.URL)
				}
				body := easyEDAFixture
				if internationalFail && r.URL.Host == "pro.easyeda.com" {
					body = `{"code":0,"result":{"productList":[{"code":"C258040","name":"fuzzy"}]}}`
				}
				return jsonHTTP(body), nil
			})
			trace := &ParseTrace{}
			ctx := context.WithValue(context.Background(), traceKey{}, trace)
			info, params, err := p.fetchEasyEDA(ctx, "C25804")
			if err != nil {
				t.Fatal(err)
			}
			q := info.SupplierQuote
			if info.Name != "厚膜电阻" || info.Model != "R0603" || info.Package != "0603" || info.Manufacturer != "UNI-ROYAL" || info.CategoryName != "贴片电阻" || info.Description != "10kΩ ±1%" || info.ImageURL != "https://example.com/1.jpg" || info.DatasheetURL != "https://example.com/datasheet" || q.Currency != nil || q.StockQuantity == nil || *q.StockQuantity != 0 || len(q.PriceTiers) != 2 || q.PriceTiers[0].MinQuantity != 100 || q.PriceTiers[1].UnitPrice != "0.0020" || strings.Contains(params, "drawing") || !strings.Contains(params, "10kΩ") {
				t.Fatalf("mapping: %+v %+v %s", info, q, params)
			}
			want := 1
			if internationalFail {
				want = 2
			}
			if len(hosts) != want || len(trace.Calls) != want || hosts[0] != "pro.easyeda.com" || (want == 2 && hosts[1] != "pro.lceda.cn") || q.SourceURL != trace.Calls[want-1].Endpoint {
				t.Fatalf("fallback/trace: %v %+v", hosts, trace)
			}
		})
	}
}
func TestEasyEDAInternationalMapping(t *testing.T) {
	p := NewLCSCParser()
	p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) {
		return jsonHTTP(`{"code":0,"result":{"productList":[{"number":"C25804","mpn":"R0603","manufacturer":"Brand","package":"0603","stockNumber":12,"currency":"USD","price":[[100,"0.00240"],[1000,"0.00200"]],"image":[{"900x900":"https://example.com/image.jpg"}],"device_info":{"attributes":{"LCSC Part Name":"Resistor","Datasheet":"https://example.com/file.pdf","Value":"10kΩ"},"description":"device description","tags":{"child_tag":{"name":"Resistor"}}}}]}}`), nil
	})
	info, _, err := p.fetchEasyEDA(context.Background(), "C25804")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Resistor" || info.Description != "device description" || info.CategoryName != "Resistor" || info.SupplierQuote.StockQuantity == nil || *info.SupplierQuote.StockQuantity != 12 || *info.SupplierQuote.Currency != "USD" || info.SupplierQuote.PriceTiers[0].UnitPrice != "0.00240" || info.ImageURL != "https://example.com/image.jpg" {
		t.Fatalf("international mapping: %+v", info)
	}
}
func TestEasyEDAFailures(t *testing.T) {
	for _, body := range []string{`{`, `{"code":403,"result":{"productList":[]}}`, `{"result":{"productList":[]}}`, `{"code":200,"result":{}}`, `{"code":200,"ok":false,"result":{"productList":[]}}`, `{"code":200,"result":{"productList":[{"code":"C25804"}]}}`, strings.Repeat("x", (2<<20)+1)} {
		t.Run(fmt.Sprint(len(body)), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, body) }))
			defer server.Close()
			_, _, err := localParser(server).fetchEasyEDA(context.Background(), "C25804")
			if err == nil || calls != 2 {
				t.Fatalf("error: %v calls=%d", err, calls)
			}
		})
	}
	p := NewLCSCParser()
	calls := 0
	p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		resp := jsonHTTP("")
		resp.StatusCode = 403
		return resp, nil
	})
	_, _, err := p.fetchEasyEDA(context.Background(), "C25804")
	if err == nil || calls != 2 || !strings.Contains(err.Error(), "403") {
		t.Fatal(err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	_, _, err = p.fetchEasyEDA(ctx, "C25804")
	if !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal(err, calls)
	}
}

func TestStandardAndLLMUseEasyEDABeforeHTML(t *testing.T) {
	for _, mode := range []string{"standard", "llm"} {
		t.Run(mode, func(t *testing.T) {
			model := &detailCompleter{}
			p, hosts := agentParser(model, true)
			trace := &ParseTrace{}
			info, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: mode, Trace: trace})
			if err != nil {
				t.Fatal(err)
			}
			want := "厚膜电阻"
			if mode == "llm" {
				want = "AI 电阻"
			}
			if info.Name != want || len(*hosts) != 2 || (*hosts)[1] != "pro.easyeda.com" || trace.FinalSource != "easyeda_product_search" || trace.Mode != mode || len(trace.Calls) != 2 || info.SupplierQuote.SourceName != "EasyEDA" || info.SupplierQuote.PriceTiers[1].UnitPrice != "0.0020" {
				t.Fatal(info, *hosts, trace)
			}
			if mode == "llm" && (!strings.Contains(model.input, "10kΩ") || strings.Contains(model.input, "supplier_quote")) {
				t.Fatal("LLM input", model.input)
			}
		})
	}
}

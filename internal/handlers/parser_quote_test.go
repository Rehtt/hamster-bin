package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/gin-gonic/gin"
)

type quoteParser struct{ t *testing.T }

func (p quoteParser) GetName() string           { return "local" }
func (p quoteParser) CanParse(code string) bool { return code == "C25804" }
func (p quoteParser) Parse(code string) (*parser.ComponentInfo, error) {
	return p.ParseWithOptions(code, parser.ParseOptions{})
}
func (p quoteParser) ParseWithOptions(code string, options parser.ParseOptions) (*parser.ComponentInfo, error) {
	if options.Context == nil || !options.UseLLM {
		p.t.Error("request context/options not propagated")
	}
	stock := int64(0)
	currency := "USD"
	return &parser.ComponentInfo{Name: "RES", PlatformCode: code, SupplierQuote: &parser.SupplierQuote{StockQuantity: &stock, Currency: &currency, PriceTiers: []parser.SupplierPriceTier{{MinQuantity: 100, UnitPrice: "0.0020"}}, QueriedAt: "2026-01-02T03:04:05Z"}}, nil
}

func TestSynchronousParseSupplierQuote(t *testing.T) {
	manager := parser.NewParserManager()
	manager.Register(quoteParser{t})
	handler := NewParserHandler(manager)
	for _, qr := range []bool{false, true} {
		t.Run(map[bool]string{false: "code", true: "qrcode"}[qr], func(t *testing.T) {
			body := `{"code":"C25804","use_llm":true}`
			if qr {
				body = `{"qrcode_data":"{pc:C25804,qty:20}","use_llm":true}`
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/parse", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			if qr {
				handler.ParseQRCode(c)
			} else {
				handler.ParseComponent(c)
			}
			if w.Code != http.StatusOK {
				t.Fatalf("response: %d %s", w.Code, w.Body.String())
			}
			var response struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			componentJSON := response.Data
			if qr {
				var result struct {
					Component json.RawMessage `json:"component"`
					Quantity  int             `json:"quantity"`
				}
				if err := json.Unmarshal(response.Data, &result); err != nil {
					t.Fatal(err)
				}
				if result.Quantity != 20 {
					t.Fatal("QR quantity changed")
				}
				componentJSON = result.Component
			}
			var info parser.ComponentInfo
			if err := json.Unmarshal(componentJSON, &info); err != nil {
				t.Fatal(err)
			}
			q := info.SupplierQuote
			if q == nil || *q.StockQuantity != 0 || *q.Currency != "USD" || q.PriceTiers[0].UnitPrice != "0.0020" || q.QueriedAt != "2026-01-02T03:04:05Z" || info.Price != 0 {
				t.Fatalf("quote lost: %+v", info)
			}
		})
	}
}

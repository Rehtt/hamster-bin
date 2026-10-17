package tasks

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/Rehtt/hamster-bin/internal/repository"
)

type quoteParser struct{ t *testing.T }

func (p quoteParser) GetName() string           { return "local" }
func (p quoteParser) CanParse(code string) bool { return code == "C25804" }
func (p quoteParser) Parse(code string) (*parser.ComponentInfo, error) {
	return p.ParseWithOptions(code, parser.ParseOptions{})
}
func (p quoteParser) ParseWithOptions(code string, options parser.ParseOptions) (*parser.ComponentInfo, error) {
	if options.Context == nil || !options.UseLLM {
		p.t.Error("task context/options not propagated")
	}
	stock := int64(0)
	currency := "USD"
	return &parser.ComponentInfo{Name: "RES", PlatformCode: code, SupplierQuote: &parser.SupplierQuote{StockQuantity: &stock, Currency: &currency, PriceTiers: []parser.SupplierPriceTier{{MinQuantity: 100, UnitPrice: "0.0020"}}, QueriedAt: "2026-01-02T03:04:05Z"}}, nil
}

func TestSupplierQuoteTaskPersistence(t *testing.T) {
	for _, kind := range []string{ComponentParse, QRCodeParse} {
		t.Run(kind, func(t *testing.T) {
			s := testService(t, 1, time.Second)
			s.manager = parser.NewParserManager()
			s.manager.Register(quoteParser{t})
			task, err := s.Submit(Request{Type: kind, Input: Input{Code: "C25804", QRCodeData: "{pc:C25804,qty:20}", UseLLM: true}, Source: "pre-stocks"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			start(t, s)
			done := waitTask(t, s, task.ID, repository.TaskSucceeded)
			var result Result
			if err := json.Unmarshal(done.Result, &result); err != nil {
				t.Fatal(err)
			}
			if result.Component == nil {
				t.Fatal("missing component")
			}
			q := result.Component.SupplierQuote
			if q == nil || *q.StockQuantity != 0 || *q.Currency != "USD" || q.PriceTiers[0].UnitPrice != "0.0020" || q.QueriedAt != "2026-01-02T03:04:05Z" || result.Component.Price != 0 {
				t.Fatalf("snapshot lost: %s", done.Result)
			}
			if kind == QRCodeParse && result.Quantity != 20 {
				t.Fatal("supplier stock replaced QR quantity")
			}
		})
	}
}

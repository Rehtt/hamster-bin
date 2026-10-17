package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/parser"
	"github.com/gin-gonic/gin"
)

type modeParser struct{ fail bool }

func (p modeParser) GetName() string                             { return "mode" }
func (p modeParser) CanParse(string) bool                        { return true }
func (p modeParser) Parse(string) (*parser.ComponentInfo, error) { panic("missing options") }
func (p modeParser) ParseWithOptions(code string, o parser.ParseOptions) (*parser.ComponentInfo, error) {
	if o.Context == nil || o.Trace == nil {
		panic("missing context/trace")
	}
	*o.Trace = parser.ParseTrace{Mode: o.Mode(), FinalSource: "local", Calls: []parser.ParseCall{{Tool: "local", Arguments: map[string]string{"code": code}, Status: "success"}}}
	if p.fail {
		return nil, parser.ErrParserContent
	}
	return &parser.ComponentInfo{Name: "resistor", PlatformCode: code}, nil
}
func TestSynchronousModesAndTrace(t *testing.T) {
	for _, qr := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			for _, mode := range []string{"standard", "llm", "agent", "invalid"} {
				t.Run(fmt.Sprint(qr, fail, mode), func(t *testing.T) {
					manager := parser.NewParserManager()
					manager.Register(modeParser{fail: fail})
					h := NewParserHandler(manager)
					body := map[string]any{"code": "C123", "qrcode_data": "{pc:C123,qty:2}", "parse_mode": mode, "use_llm": true}
					raw, _ := json.Marshal(body)
					w := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(w)
					c.Request = httptest.NewRequest(http.MethodPost, "/parse", strings.NewReader(string(raw)))
					c.Request.Header.Set("Content-Type", "application/json")
					if qr {
						h.ParseQRCode(c)
					} else {
						h.ParseComponent(c)
					}
					want := 200
					if fail {
						want = 422
					}
					if mode == "invalid" {
						want = 400
					}
					if w.Code != want {
						t.Fatal(w.Code, w.Body.String())
					}
					var response struct {
						ParseTrace *parser.ParseTrace `json:"parse_trace"`
					}
					if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					if mode != "invalid" && (response.ParseTrace == nil || response.ParseTrace.Mode != mode || len(response.ParseTrace.Calls) != 1) {
						t.Fatal(w.Body.String())
					}
				})
			}
		}
	}
}

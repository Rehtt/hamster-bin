package parser

import (
	"context"
	"github.com/Rehtt/hamster-bin/internal/llm"
	"github.com/Rehtt/hamster-bin/internal/normalization"
	"net/http"
	"strings"
	"testing"
)

func TestAllModesNormalizeAndPreserveSources(t *testing.T) {
	for _, mode := range []string{"standard", "llm", "agent", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			model := &fakeCompleter{result: `{"name":"中文整理名称","description":"中文整理描述","manufacturer":"Different Company","package":"SOT23"}`}
			var completer JSONCompleter = model
			if mode == "agent" {
				agent := &agentModel{}
				agent.fn = func(context.Context, []llm.Message, []llm.Tool) (llm.Message, error) {
					if agent.rounds == 1 {
						return toolCall("lcsc_product_detail", `{"code":"C25804"}`), nil
					}
					return llm.Message{Content: `{"source":"lcsc_product_detail","component":{"name":"中文整理名称","package":"SOT23","manufacturer":"Different Company"}}`}, nil
				}
				completer = agent
			}
			if mode == "fallback" {
				completer = nil
			}
			p, hosts := agentParser(completer, false)
			selectedMode := mode
			if mode == "fallback" {
				selectedMode = "agent"
			}
			info, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: selectedMode})
			if err != nil {
				t.Fatal(err)
			}
			if info.Normalization == nil || info.Normalization.Version != normalization.Version || len(info.Normalization.Sources) != 1 || len(*hosts) != 1 {
				t.Fatal(info, hosts)
			}
			src := info.Normalization.Sources[0]
			if src.Fields.Name != "RES 10kΩ" || src.Fields.Manufacturer != "UNI-ROYAL" || src.Parameters == "" || src.URL == "" || src.QueriedAt == "" {
				t.Fatal(src)
			}
			if info.Manufacturer != "UNI-ROYAL" {
				t.Fatal("model merged brand")
			}
			if mode == "standard" || mode == "fallback" {
				if model.called {
					t.Fatal("ordinary/fallback called model")
				}
			} else {
				if info.Package != "SOT-23" {
					t.Fatal("package not normalized")
				}
				semantic := false
				for _, c := range info.Normalization.Changes {
					semantic = semantic || c.Semantic
				}
				if !semantic {
					t.Fatal("AI change not marked")
				}
			}
		})
	}
}
func TestAgentConflictingSourcesUsePrimaryFacts(t *testing.T) {
	m := &agentModel{}
	m.fn = func(context.Context, []llm.Message, []llm.Tool) (llm.Message, error) {
		switch m.rounds {
		case 1:
			return toolCall("lcsc_product_detail", `{"code":"C25804"}`), nil
		case 2:
			return toolCall("easyeda_product_search", `{"code":"C25804"}`), nil
		default:
			return finalMessage("lcsc_product_detail"), nil
		}
	}
	p, _ := agentParser(m, false)
	info, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Normalization.Sources) != 2 {
		t.Fatal(info.Normalization)
	}
	for _, i := range info.Normalization.Issues {
		if i.Code == "source_conflict" && i.Field == "name" {
			if info.Name != "RES 10kΩ" {
				t.Fatal("mixed conflicting field", info.Name)
			}
			return
		}
	}
	t.Fatal("missing conflict")
}
func TestImageNormalizationUsesSameRules(t *testing.T) {
	p := NewImageParser(&fakeCompleter{result: `{"items":[{"name":"Resistor","category_name":"Resistor","package":"SOT23-5","manufacturer":"Texas Instruments Inc."}]}`})
	items, err := p.Parse(context.Background(), []byte("fake"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	item := items[0]
	if item.Name != "电阻" || item.Package != "SOT-23-5" || item.CategoryID != 0 || item.CategoryName != "" || item.Normalization.Sources[0].Fields.Name != "Resistor" {
		t.Fatal(item)
	}
}
func TestSourceParametersTruncation(t *testing.T) {
	src := snapshot(&ComponentInfo{Name: "Resistor"}, "local", "local", strings.Repeat("a", 12001))
	if !src.Truncated || len(src.Parameters) != 12000 {
		t.Fatal(src.Truncated, len(src.Parameters))
	}
}

func TestAllSourceFallbacksNormalize(t *testing.T) {
	for _, host := range []string{"wmsc.lcsc.com", "pro.easyeda.com", "pro.lceda.cn", "www.jlc-smt.com"} {
		t.Run(host, func(t *testing.T) {
			p := NewLCSCParser()
			p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != host {
					return jsonHTTP(`{}`), nil
				}
				switch host {
				case "wmsc.lcsc.com":
					return jsonHTTP(detailFixture), nil
				case "www.jlc-smt.com":
					return jsonHTTP(fallbackHTML), nil
				default:
					return jsonHTTP(easyEDAFixture), nil
				}
			})
			info, err := p.Parse("C25804")
			if err != nil || info.Normalization == nil || len(info.Normalization.Sources) != 1 || !strings.Contains(info.Normalization.Sources[0].URL, host) {
				t.Fatal(info, err)
			}
		})
	}
}

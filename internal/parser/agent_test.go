package parser

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/llm"
)

func jsonHTTP(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

type agentModel struct {
	fn     func(context.Context, []llm.Message, []llm.Tool) (llm.Message, error)
	rounds int
}

func (m *agentModel) Configured() bool { return true }
func (m *agentModel) CompleteJSON(context.Context, []llm.Message) (string, error) {
	panic("Agent fallback must not run AI enrichment")
}
func (m *agentModel) CompleteTools(ctx context.Context, messages []llm.Message, tools []llm.Tool) (llm.Message, error) {
	m.rounds++
	return m.fn(ctx, messages, tools)
}
func toolCall(name, args string) llm.Message {
	return llm.Message{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call1", Type: "function", Function: llm.FunctionCall{Name: name, Arguments: args}}}}
}
func finalMessage(source string) llm.Message {
	return llm.Message{Role: "assistant", Content: `{"source":"` + source + `","component":{"name":"AI 电阻","price":999,"supplier_quote":{"stock_quantity":999}}}`}
}
func agentParser(m JSONCompleter, failLCSC bool) (*LCSCParser, *[]string) {
	p := NewLCSCParser(m)
	hosts := []string{}
	p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) {
		hosts = append(hosts, r.URL.Host)
		if r.URL.Host == "wmsc.lcsc.com" {
			if failLCSC {
				return nil, errors.New("unavailable")
			}
			return jsonHTTP(detailFixture), nil
		}
		return jsonHTTP(easyEDAFixture), nil
	})
	return p, &hosts
}
func TestAgentSelectsToolsAndPreservesQuote(t *testing.T) {
	for _, tool := range []string{"lcsc_product_detail", "easyeda_product_search"} {
		t.Run(tool, func(t *testing.T) {
			m := &agentModel{}
			m.fn = func(ctx context.Context, messages []llm.Message, tools []llm.Tool) (llm.Message, error) {
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > 60*time.Second {
					t.Fatal("missing budget")
				}
				if len(tools) != 2 {
					t.Fatal("registry", tools)
				}
				if m.rounds == 1 {
					return toolCall(tool, `{"code":"C25804"}`), nil
				}
				last := messages[len(messages)-1]
				if last.Role != "tool" || last.ToolCallID != "call1" || !strings.Contains(last.Content, `"status":"success"`) || strings.Contains(last.Content, "supplier_quote") {
					t.Fatal("tool round trip", last)
				}
				return finalMessage(tool), nil
			}
			p, hosts := agentParser(m, false)
			trace := &ParseTrace{}
			info, err := p.ParseWithOptions(" c25804 ", ParseOptions{ParseMode: "agent", Trace: trace})
			if err != nil || info.Name != "AI 电阻" || info.Price != 0 || *info.SupplierQuote.StockQuantity != 0 || info.SupplierQuote.PriceTiers[1].UnitPrice != "0.0020" || trace.Fallback || trace.FinalSource != tool || len(*hosts) != 1 {
				t.Fatalf("result=%+v trace=%+v err=%v", info, trace, err)
			}
		})
	}
}
func TestAgentObservesFailureSwitchesAndCaches(t *testing.T) {
	m := &agentModel{}
	m.fn = func(_ context.Context, messages []llm.Message, _ []llm.Tool) (llm.Message, error) {
		switch m.rounds {
		case 1:
			return toolCall("lcsc_product_detail", `{"code":"C25804"}`), nil
		case 2:
			if !strings.Contains(messages[len(messages)-1].Content, `"status":"error"`) {
				t.Fatal("missing error")
			}
			return toolCall("easyeda_product_search", `{"code":"C25804"}`), nil
		case 3:
			return toolCall("easyeda_product_search", `{"code":"C25804"}`), nil
		default:
			return finalMessage("easyeda_product_search"), nil
		}
	}
	p, hosts := agentParser(m, true)
	trace := &ParseTrace{}
	info, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: "agent", Trace: trace})
	if err != nil || info == nil || !reflect.DeepEqual(*hosts, []string{"wmsc.lcsc.com", "pro.easyeda.com"}) || len(trace.Calls) != 3 || !trace.Calls[2].Cached || trace.Fallback {
		t.Fatal(info, err, *hosts, trace)
	}
}
func TestAgentFallbacks(t *testing.T) {
	for _, kind := range []string{"unconfigured", "incompatible", "model error", "child timeout", "ungrounded", "invalid json", "invalid component", "round budget", "tool budget", "illegal args", "unknown tool"} {
		t.Run(kind, func(t *testing.T) {
			m := &agentModel{}
			m.fn = func(_ context.Context, _ []llm.Message, _ []llm.Tool) (llm.Message, error) {
				switch kind {
				case "child timeout":
					return llm.Message{}, context.DeadlineExceeded
				case "model error":
					return llm.Message{}, errors.New("tools not supported")
				case "ungrounded":
					return finalMessage("lcsc_product_detail"), nil
				case "invalid json":
					return llm.Message{Content: "not JSON"}, nil
				case "invalid component":
					if m.rounds == 1 {
						return toolCall("lcsc_product_detail", `{"code":"C25804"}`), nil
					}
					return llm.Message{Content: `{"source":"lcsc_product_detail","component":null}`}, nil
				case "illegal args":
					return toolCall("lcsc_product_detail", `{"code":"C123","url":"https://invalid"}`), nil
				case "unknown tool":
					return toolCall("arbitrary_request", `{"code":"C25804"}`), nil
				case "tool budget":
					msg := toolCall("lcsc_product_detail", `{"code":"C25804"}`)
					msg.ToolCalls = append(msg.ToolCalls, msg.ToolCalls...)
					msg.ToolCalls[1].ID = "call2"
					return msg, nil
				default:
					return toolCall("lcsc_product_detail", `{"code":"C25804"}`), nil
				}
			}
			var completer JSONCompleter = m
			if kind == "unconfigured" {
				completer = nil
			}
			if kind == "incompatible" {
				completer = &fakeCompleter{}
			}
			p, hosts := agentParser(completer, false)
			trace := &ParseTrace{}
			info, err := p.ParseWithOptions("C25804", ParseOptions{UseLLM: true, ParseMode: "agent", Trace: trace})
			if kind == "tool budget" && !strings.Contains(trace.FallbackReason, "4次工具调用") {
				t.Fatal(trace.FallbackReason)
			}
			if kind == "round budget" && !strings.Contains(trace.FallbackReason, "4轮") {
				t.Fatal(trace.FallbackReason)
			}
			if err != nil || info.Name != "RES 10kΩ" || !trace.Fallback || trace.FallbackReason == "" || trace.FinalSource != "lcsc_product_detail" || len(*hosts) != 1 || m.rounds > 4 {
				t.Fatalf("%+v %v %+v hosts=%v rounds=%d", info, err, trace, *hosts, m.rounds)
			}
		})
	}
}
func TestAgentCancellationDoesNotFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &agentModel{fn: func(ctx context.Context, _ []llm.Message, _ []llm.Tool) (llm.Message, error) {
		cancel()
		<-ctx.Done()
		return llm.Message{}, ctx.Err()
	}}
	p, hosts := agentParser(m, false)
	trace := &ParseTrace{}
	_, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: "agent", Context: ctx, Trace: trace})
	if !errors.Is(err, context.Canceled) || len(*hosts) != 0 || trace.Fallback {
		t.Fatal(err, *hosts, trace)
	}
}
func TestParseModeCompatibility(t *testing.T) {
	for _, tc := range []struct {
		mode string
		use  bool
		want string
	}{{"", false, "standard"}, {"", true, "llm"}, {"standard", true, "standard"}, {"llm", false, "llm"}, {"agent", true, "agent"}} {
		o := ParseOptions{ParseMode: tc.mode, UseLLM: tc.use}
		if o.Mode() != tc.want || o.ValidateMode() != nil {
			t.Fatal(o)
		}
	}
	p, hosts := agentParser(nil, false)
	if _, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: "standard", UseLLM: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: "bad"}); !errors.Is(err, ErrInvalidParseMode) {
		t.Fatal(err)
	}
	if len(*hosts) != 1 {
		t.Fatal(*hosts)
	}
	if err := p.ValidateOptions(ParseOptions{ParseMode: "agent"}); err != nil {
		t.Fatal(err)
	}
}

func TestAgentParentDeadlineStopsActiveTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	m := &agentModel{fn: func(context.Context, []llm.Message, []llm.Tool) (llm.Message, error) {
		return toolCall("lcsc_product_detail", `{"code":"C25804"}`), nil
	}}
	p := NewLCSCParser(m)
	calls := 0
	p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	trace := &ParseTrace{}
	_, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: "agent", Context: ctx, Trace: trace})
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 || trace.Fallback || len(trace.Calls) != 1 {
		t.Fatal(err, calls, trace)
	}
}
func TestAgentInvalidArgumentsCanRecover(t *testing.T) {
	m := &agentModel{}
	m.fn = func(_ context.Context, messages []llm.Message, _ []llm.Tool) (llm.Message, error) {
		if m.rounds == 1 {
			return toolCall("lcsc_product_detail", `{"code":"C999"}`), nil
		}
		if m.rounds == 2 {
			if !strings.Contains(messages[len(messages)-1].Content, "invalid tool") {
				t.Fatal(messages)
			}
			return toolCall("lcsc_product_detail", `{"code":"C25804"}`), nil
		}
		return finalMessage("lcsc_product_detail"), nil
	}
	p, hosts := agentParser(m, false)
	trace := &ParseTrace{}
	_, err := p.ParseWithOptions("C25804", ParseOptions{ParseMode: "agent", Trace: trace})
	if err != nil || trace.Fallback || len(*hosts) != 1 || len(trace.Calls) != 2 || !strings.Contains(trace.Calls[0].Arguments["raw"], "C999") {
		t.Fatal(err, trace, *hosts)
	}
}

package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCompleteJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header: %s", r.Header.Get("Authorization"))
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "test-model" {
			t.Fatalf("unexpected model: %s", req.Model)
		}
		if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
			t.Fatalf("unexpected response_format: %#v", req.ResponseFormat)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"name\":\"电阻\"}"}}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/v1", "test-key", "test-model")
	got, err := client.CompleteJSON(context.Background(), []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"name":"电阻"}` {
		t.Fatalf("unexpected content: %s", got)
	}
}

func TestClientCompleteJSONNotConfigured(t *testing.T) {
	client := NewClient("", "", "")
	if _, err := client.CompleteJSON(context.Background(), nil); err != ErrNotConfigured {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestClientToolsRoundTrip(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		if len(req.Tools) != 1 || req.Tools[0].Function.Name != "lookup" || req.ResponseFormat != nil {
			t.Error("tool declaration", req)
		}
		if calls == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"code\":\"C123\"}"}}]}}]}`))
		} else {
			if len(req.Messages) != 3 || len(req.Messages[1].ToolCalls) != 1 || req.Messages[1].ToolCalls[0].ID != "call_1" || req.Messages[2].ToolCallID != "call_1" || req.Messages[2].Role != "tool" {
				t.Error("tool round trip", req.Messages)
			}
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"done"}}]}`))
		}
	}))
	defer server.Close()
	c := NewClient(server.URL, "key", "model")
	tools := []Tool{{Type: "function", Function: FunctionDefinition{Name: "lookup", Description: "lookup product", Parameters: map[string]any{"type": "object"}}}}
	messages := []Message{{Role: "user", Content: "C123"}}
	msg, err := c.CompleteTools(context.Background(), messages, tools)
	if err != nil || len(msg.ToolCalls) != 1 {
		t.Fatal(msg, err)
	}
	messages = append(messages, msg, Message{Role: "tool", ToolCallID: msg.ToolCalls[0].ID, Content: `{"name":"resistor"}`})
	msg, err = c.CompleteTools(context.Background(), messages, tools)
	if err != nil || msg.Content != "done" || calls != 2 {
		t.Fatal(msg, err, calls)
	}
}

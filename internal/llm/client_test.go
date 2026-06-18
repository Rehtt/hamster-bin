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

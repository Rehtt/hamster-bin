package parser

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rehtt/hamster-bin/internal/llm"
)

type localTransport func(*http.Request) (*http.Response, error)

func (f localTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func localParser(server *httptest.Server, completer ...JSONCompleter) *LCSCParser {
	p := NewLCSCParser(completer...)
	p.client.Transport = localTransport(func(r *http.Request) (*http.Response, error) {
		req, err := http.NewRequestWithContext(r.Context(), r.Method, server.URL+r.URL.RequestURI(), r.Body)
		if err != nil {
			return nil, err
		}
		req.Header = r.Header
		return http.DefaultTransport.RoundTrip(req)
	})
	return p
}
func TestLCSCParser_parseByAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div class="BaseInfo_component-info__yuOgz"><h1 class="BaseInfo_component-name__7OSgG">本地电阻</h1><dl><div><dt>厂家型号</dt><dd>R0603</dd></div></dl></div>`))
	}))
	defer server.Close()
	p := localParser(server)
	info, err := p.parseByAPI("C103086")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "本地电阻" || info.PlatformCode != "C103086" {
		t.Fatalf("unexpected info %+v", info)
	}
}
func TestLCSCParserHTTPContextCancellation(t *testing.T) {
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-time.After(2 * time.Second):
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := localParser(server).ParseWithOptions("C123", ParseOptions{Context: ctx})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected propagated timeout: %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("HTTP request not cancelled")
	}
}
func TestLCSCParserModelContextCancellation(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>model input</html>")) }))
	defer page.Close()
	cancelled := make(chan struct{})
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-time.After(2 * time.Second):
		}
	}))
	defer model.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err := localParser(page, llm.NewClient(model.URL, "test-key", "test-model")).ParseWithOptions("C123", ParseOptions{UseLLM: true, Context: ctx})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected model timeout: %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("model request not cancelled")
	}
}
func TestLCSCParserLocalModelResponse(t *testing.T) {
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>model input</html>")) }))
	defer page.Close()
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"name\":\"AI 电阻\",\"category_name\":\"电阻\"}"}}]}`))
	}))
	defer model.Close()
	info, err := localParser(page, llm.NewClient(model.URL, "test-key", "test-model")).ParseWithOptions("C123", ParseOptions{UseLLM: true})
	if err != nil || info.Name != "AI 电阻" || info.CategoryName != "" || info.Normalization == nil {
		t.Fatalf("model response: %+v %v", info, err)
	}
}

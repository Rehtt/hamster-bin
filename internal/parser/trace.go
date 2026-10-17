package parser

import (
	"context"
	"time"
)

// ParseTrace 仅记录本次解析的查询诊断，不保存模型提示词或认证信息。
type ParseTrace struct {
	Mode           string      `json:"mode"`
	FinalSource    string      `json:"final_source,omitempty"`
	Fallback       bool        `json:"fallback"`
	FallbackReason string      `json:"fallback_reason,omitempty"`
	Calls          []ParseCall `json:"calls"`
}
type ParseCall struct {
	Tool       string            `json:"tool"`
	Arguments  map[string]string `json:"arguments"`
	Endpoint   string            `json:"endpoint,omitempty"`
	StartedAt  string            `json:"started_at"`
	DurationMS int64             `json:"duration_ms"`
	Status     string            `json:"status"`
	Summary    string            `json:"summary"`
	Cached     bool              `json:"cached,omitempty"`
}
type traceKey struct{}

func traceFrom(ctx context.Context) *ParseTrace {
	t, _ := ctx.Value(traceKey{}).(*ParseTrace)
	return t
}
func recordQuery(ctx context.Context, tool, code, endpoint string, cached bool, fn func() (*ComponentInfo, string, error)) (*ComponentInfo, string, error) {
	start := time.Now()
	info, params, err := fn()
	if t := traceFrom(ctx); t != nil {
		call := ParseCall{Tool: tool, Arguments: map[string]string{"code": code}, Endpoint: endpoint, StartedAt: start.UTC().Format(time.RFC3339Nano), DurationMS: time.Since(start).Milliseconds(), Status: "success", Cached: cached}
		if err != nil {
			call.Status = "error"
			call.Summary = err.Error()
		} else if info != nil {
			call.Summary = info.Name
		}
		t.Calls = append(t.Calls, call)
	}
	return info, params, err
}

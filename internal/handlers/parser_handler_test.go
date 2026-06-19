package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Rehtt/hamster-bin/internal/llm"
	"github.com/Rehtt/hamster-bin/internal/parser"
)

func TestParseErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantPrefix string
	}{
		{
			name:       "no parsers available",
			err:        parser.ErrNoParsersAvailable,
			wantStatus: http.StatusServiceUnavailable,
			wantPrefix: "解析服务不可用",
		},
		{
			name:       "no parser matched",
			err:        parser.ErrNoParserMatched,
			wantStatus: http.StatusBadRequest,
			wantPrefix: "无法识别的平台编码格式",
		},
		{
			name:       "llm not configured",
			err:        llm.ErrNotConfigured,
			wantStatus: http.StatusBadRequest,
			wantPrefix: "LLM 未配置",
		},
		{
			name:       "upstream error",
			err:        fmt.Errorf("HTTP 状态码: 404: %w", parser.ErrParserUpstream),
			wantStatus: http.StatusBadGateway,
			wantPrefix: "上游平台请求失败",
		},
		{
			name:       "content error",
			err:        fmt.Errorf("未找到元件信息容器: %w", parser.ErrParserContent),
			wantStatus: http.StatusUnprocessableEntity,
			wantPrefix: "页面内容解析失败",
		},
		{
			name:       "unknown error",
			err:        errors.New("unexpected panic recovery"),
			wantStatus: http.StatusInternalServerError,
			wantPrefix: "解析失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, message := parseErrorResponse(tt.err)
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d", status, tt.wantStatus)
			}
			if !errors.Is(tt.err, parser.ErrParserUpstream) && !errors.Is(tt.err, parser.ErrParserContent) {
				if message != tt.wantPrefix && len(message) < len(tt.wantPrefix) {
					t.Fatalf("message = %q, want prefix %q", message, tt.wantPrefix)
				}
			}
			if len(message) < len(tt.wantPrefix) || message[:len(tt.wantPrefix)] != tt.wantPrefix {
				t.Fatalf("message = %q, want prefix %q", message, tt.wantPrefix)
			}
		})
	}
}

package o11y

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

type AgentEvent struct {
	TraceID   string                 `json:"trace_id"`            // 贯穿整个任务的唯一 ID
	SpanID    string                 `json:"span_id"`             // 当前动作的 ID
	ParentID  string                 `json:"parent_id,omitempty"` // 父级动作 ID
	AgentName string                 `json:"agent_name"`          // 触发者 (如 Lead, FrontendDev)
	EventType string                 `json:"event_type"`          // 事件类型：llm_call, tool_start, tool_end, error
	Timestamp time.Time              `json:"timestamp"`
	Duration  int64                  `json:"duration_ms,omitempty"` // 耗时 (毫秒)
	Payload   map[string]interface{} `json:"payload"`               // 具体数据 (Prompt, 参数, 结果, Token消耗)
}
type traceKey struct{}
type TraceContext struct {
	TraceID string
	SpanID  string
}

func GenerateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func WithTrace(ctx context.Context, traceID, parentSpanID string) context.Context {
	if traceID == "" {
		traceID = GenerateID()
	}
	return context.WithValue(ctx, traceKey{}, TraceContext{
		TraceID: traceID,
		SpanID:  parentSpanID,
	})
}
func GetTrace(ctx context.Context) TraceContext {
	if tc, ok := ctx.Value(traceKey{}).(TraceContext); ok {
		return tc
	}
	return TraceContext{TraceID: GenerateID(), SpanID: ""} // Fallback
}

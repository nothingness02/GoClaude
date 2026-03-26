package agent

import (
	"context"
	"time"

	"github.com/GoClaude/common"
	o11y "github.com/GoClaude/olly"
	"github.com/sashabaranov/go-openai"
)

func ExecuteToolWithtrace(ctx context.Context, agent *AgentInstance, toolCall openai.ToolCall) string {
	tc := o11y.GetTrace(ctx)
	spanID := o11y.GenerateID()
	startTime := time.Now()
	o11y.GlobalO11y.Record(o11y.AgentEvent{
		TraceID: tc.TraceID, SpanID: spanID, ParentID: tc.SpanID,
		AgentName: agent.Name, EventType: "tool_start", Timestamp: startTime,
		Payload: map[string]interface{}{"tool_name": toolCall.Function.Name, "args": toolCall.Function.Arguments},
	})
	out := agent.ToolProvider.HandleToolCall(toolCall)
	duration := time.Since(startTime).Milliseconds()
	o11y.GlobalO11y.Record(o11y.AgentEvent{
		TraceID: tc.TraceID, SpanID: spanID, ParentID: tc.SpanID,
		AgentName: agent.Name, EventType: "tool_success", Timestamp: time.Now(), Duration: duration,
		Payload: map[string]interface{}{"tool_name": toolCall.Function.Name, "output_preview": out},
	})
	return out
}

func CallLLMWithTrace(ctx context.Context, agent *AgentInstance, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	tc := o11y.GetTrace(ctx)
	spanID := o11y.GenerateID() // 为这次 LLM 调用生成新的 SpanID
	startTime := time.Now()
	o11y.GlobalO11y.Record(o11y.AgentEvent{
		TraceID: tc.TraceID, SpanID: spanID, ParentID: tc.SpanID,
		AgentName: agent.Name, EventType: "llm_start", Timestamp: startTime,
		Payload: map[string]interface{}{"model": req.Model, "messages_count": len(req.Messages)},
	})
	resp, err := common.Client.CreateChatCompletion(ctx, req)
	duration := time.Since(startTime).Milliseconds()
	if err != nil {
		o11y.GlobalO11y.Record(o11y.AgentEvent{
			TraceID: tc.TraceID, SpanID: spanID, ParentID: tc.SpanID,
			AgentName: agent.Name, EventType: "llm_error", Timestamp: time.Now(), Duration: duration,
			Payload: map[string]interface{}{"error": err.Error()},
		})
		return resp, err
	}

	o11y.GlobalO11y.Record(o11y.AgentEvent{
		TraceID: tc.TraceID, SpanID: spanID, ParentID: tc.SpanID,
		AgentName: agent.Name, EventType: "llm_success", Timestamp: time.Now(), Duration: duration,
		Payload: map[string]interface{}{
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
			"total_tokens":      resp.Usage.TotalTokens,
			"stop_reason":       resp.Choices[0].FinishReason,
		},
	})
	return resp, nil
}

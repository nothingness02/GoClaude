package compact

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/GoClaude/common"
	"github.com/sashabaranov/go-openai"
)

func EstimateTokens(messages *[]openai.ChatCompletionMessage) int {
	b, _ := json.Marshal(*messages)
	return len(b) / 4
}
func Micro_compact(messages *[]openai.ChatCompletionMessage) {
	var toolMsgIndices []int
	for i, msg := range *messages {
		if msg.Role == openai.ChatMessageRoleTool {
			toolMsgIndices = append(toolMsgIndices, i)
		}
	}
	if len(toolMsgIndices) < common.Keep_RECENT {
		return
	}
	toolNameMap := make(map[string]string)
	for _, msg := range *messages {
		if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				toolNameMap[tc.ID] = tc.Function.Name
			}
		}
	}
	toClear := toolMsgIndices[:len(toolMsgIndices)-common.Keep_RECENT]
	for _, idx := range toClear {
		msg := &(*messages)[idx] // 取指针以便修改

		// 如果内容过长，则进行压缩替换
		if len(msg.Content) > 100 {
			toolName := toolNameMap[msg.ToolCallID]
			if toolName == "" {
				toolName = "unknown"
			}
			msg.Content = fmt.Sprintf("[Previous: used %s]", toolName)
		}
	}
}

func Auto_compact(messages *[]openai.ChatCompletionMessage) error {
	os.MkdirAll(common.TRANSCRIPT_DIR, 0755)
	filename := filepath.Join(common.TRANSCRIPT_DIR, fmt.Sprintf("transcript_%d.jsonl", time.Now().Unix()))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create transcript: %v", err)
	}
	for _, msg := range *messages {
		b, _ := json.Marshal(msg)
		file.Write(append(b, '\n'))
	}
	file.Close()
	fmt.Printf("\n[!] Context full. Transcript saved: %s\n", filename)

	//将当前对话生成摘要（限制长度防爆）
	// todos: 替换更加智能的阶段方式机制，防止信息的简单泄露
	convBytes, _ := json.Marshal(*messages)
	convText := string(convBytes)
	if len(convText) > common.THRESHOLD {
		convText = convText[:common.THRESHOLD] // 简单截断
	}
	// 3. 请求 LLM 进行总结
	summaryReq := openai.ChatCompletionRequest{
		Model: common.ModelID,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				Content: "Summarize this conversation for continuity. Include: \n" +
					"1) What was accomplished, \n" +
					"2) Current state, \n" +
					"3) Key decisions made. \n" +
					"Be concise but preserve critical details.\n\n" + convText,
			},
		},
		MaxTokens: 2000,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	resp, err := common.Client.CreateChatCompletion(ctx, summaryReq)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %v", err)
	}
	summary := resp.Choices[0].Message.Content
	// 4. 重建消息队列
	var newMsgs []openai.ChatCompletionMessage
	if len(*messages) > 0 && (*messages)[0].Role == openai.ChatMessageRoleSystem {
		newMsgs = append(newMsgs, (*messages)[0])
	}
	newMsgs = append(newMsgs, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: fmt.Sprintf("[Conversation compressed. Transcript: %s]\n\n%s", filename, summary),
	})
	newMsgs = append(newMsgs, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: "Understood. I have the context from the summary. Continuing.",
	})
	*messages = newMsgs
	fmt.Println("[!] Context successfully compacted.")
	return nil
}

package main

//todos:
// 优化交互界面.对外提供网络接口
// 提供除文字外更多有确定性的交互接口
import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/GoClaude/common"
	"github.com/GoClaude/compact"
	"github.com/GoClaude/prompt"
	"github.com/GoClaude/skills"
	"github.com/GoClaude/tools"
	"github.com/sashabaranov/go-openai"
)

func agentLoop(messages *[]openai.ChatCompletionMessage) {
	roundsSinceTodo := 0
	for {
		compact.Micro_compact(messages)
		if compact.EstimateTokens(messages) > common.THRESHOLD {
			compact.Auto_compact(messages)
		}
		tools.Notify(messages)
		req := openai.ChatCompletionRequest{
			Model:       common.ModelID,
			Messages:    *messages,
			Tools:       tools.GetALLTools(),
			MaxTokens:   4000,
			Temperature: 0.2,
		}

		ctx := context.Background()
		resp, err := common.Client.CreateChatCompletion(ctx, req)
		if err != nil {
			fmt.Printf("API Error: %v\n", err)
			return
		}

		msg := resp.Choices[0].Message
		*messages = append(*messages, msg)

		// 如果没有调用工具，则回合结束
		if len(msg.ToolCalls) == 0 {
			break
		}
		usedTodo := false
		// 处理工具调用
		for _, toolCall := range msg.ToolCalls {
			output := tools.HandleToolCall(toolCall, &usedTodo)
			// 将工具执行结果追加到消息记录
			*messages = append(*messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    output,
				ToolCallID: toolCall.ID,
			})
		}
		if usedTodo {
			roundsSinceTodo = 0
		} else {
			roundsSinceTodo++
		}

		// 如果连续 3 个带有工具调用的回合没有更新 Todo，注入提醒
		if roundsSinceTodo >= 3 {
			*messages = append(*messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "<reminder>Update your todos.</reminder>",
			})
			// 重置计数器，避免连续轰炸
			roundsSinceTodo = 0
		}
	}
}

func main() {
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: fmt.Sprintf(prompt.SYSTEM_PROMOT, common.WorkDir,
				skills.GlobalSkillLoader.GetDescriptions()),
		},
	}
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\033[36ms03 >> \033[0m")
		if !scanner.Scan() {
			break
		}
		query := scanner.Text()

		qLower := strings.ToLower(strings.TrimSpace(query))
		if qLower == "q" || qLower == "exit" || qLower == "" {
			break
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: query,
		})

		agentLoop(&messages)

		// 打印最终回复
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role == openai.ChatMessageRoleAssistant && lastMsg.Content != "" {
			fmt.Println(lastMsg.Content)
			fmt.Println()
		}
	}
}

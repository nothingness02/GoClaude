package main

//todos:
// 优化交互界面.对外提供网络接口
// 提供除文字外更多有确定性的交互接口
import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/GoClaude/agent"
	"github.com/GoClaude/common"
	"github.com/GoClaude/prompt"
	"github.com/GoClaude/skills"
	"github.com/sashabaranov/go-openai"
)

func main() {
	agent.Init()
	Agent := agent.DefaultAgentInstance()
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

		agent.RunAgentLoopWithConfig(&messages, Agent)

		// 打印最终回复
		lastMsg := messages[len(messages)-1]
		if lastMsg.Role == openai.ChatMessageRoleAssistant && lastMsg.Content != "" {
			fmt.Println(lastMsg.Content)
			fmt.Println()
		}
	}
}

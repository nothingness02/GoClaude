package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/GoClaude/agent/toolprovider"
	"github.com/GoClaude/common"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"

	promptpkg "github.com/GoClaude/prompt"
)

// todos:
// 1.添加工具的动态注册
// 2.添加skill的动态卸载和注册
// 3.消化MCP服务
// 4.强化safepath的约束

// teamManager 已移至 teammate 模块

func isRelativePath(target, base string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	// 如果相对路径以 ".." 开头，说明跳出了 base 目录
	return !strings.HasPrefix(rel, "..")
}

func safePath(p string) (string, error) {
	joined := filepath.Join(common.WorkDir, p)
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		// 如果文件不存在，使用 Clean 后的路径继续检查
		resolved = filepath.Clean(joined)
	}
	if !isRelativePath(resolved, common.WorkDir) {
		return "", fmt.Errorf("path escapes workspace: %s", p)
	}
	return resolved, nil
}

func RunBash(command string) string {
	dangerous := []string{"rm -rf /", "sudo", "shutdown", "reboot", "> /dev/"}
	for _, d := range dangerous {
		if strings.Contains(command, d) {
			return "Error: Command contains dangerous operation"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = common.WorkDir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "Error: Timeout (120s)"
	}
	res := strings.TrimSpace(string(out))
	if err != nil && res == "" {
		return fmt.Sprintf("Error: %v", err)
	}
	if len(res) > 50000 {
		return res[:50000] + "\n... (truncated)"
	}
	if res == "" {
		return "(no output)"
	}
	return res
}

func RunRead(p string, limit float64) string {
	target, err := safePath(p)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	lines := strings.Split(string(content), "\n")
	l := int(limit)
	if l > 0 && len(lines) > l {
		lines = append(lines[:l], fmt.Sprintf("... (%d more)", len(lines)-l))
	}
	res := strings.Join(lines, "\n")
	if len(res) > 50000 {
		return res[:50000] + "\n... (truncated)"
	}
	return res
}
func RunWrite(p, content string) string {
	target, err := safePath(p)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	os.MkdirAll(filepath.Dir(target), 0755)
	err = os.WriteFile(target, []byte(content), 0644)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Wrote %d bytes", len(content))
}

func RunEdit(p, oldText, newText string) string {
	target, err := safePath(p)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	strContent := string(content)
	if !strings.Contains(strContent, oldText) {
		return fmt.Sprintf("Error: Text not found in %s", p)
	}
	newContent := strings.Replace(strContent, oldText, newText, 1)
	err = os.WriteFile(target, []byte(newContent), 0644)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Edited %s", p)
}

// RunSubagent 子Agent心智循环
func RunSubagent(prompt string, description string) string {
	fmt.Printf("\n[+] Spawning Subagent for: %s\n", description)
	pr := toolprovider.NewToolProvider()
	// 注册基础工具
	pr.Register("bash", "Run a shell command.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String},
			},
			Required: []string{"command"},
		}, func(args map[string]interface{}) string {
			return RunBash(args["command"].(string))
		})
	pr.Register("read_file", "Read file contents.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":  {Type: jsonschema.String},
				"limit": {Type: jsonschema.Number},
			},
			Required: []string{"path"},
		}, func(args map[string]interface{}) string {
			limit := 0.0
			if l, ok := args["limit"].(float64); ok {
				limit = l
			}
			return RunRead(args["path"].(string), limit)
		})
	pr.Register("write_file", "Write content to file.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":    {Type: jsonschema.String},
				"content": {Type: jsonschema.String},
			},
			Required: []string{"path", "content"},
		}, func(args map[string]interface{}) string {
			return RunWrite(args["path"].(string), args["content"].(string))
		})
	pr.Register("edit_file", "Replace exact text in file.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":     {Type: jsonschema.String},
				"old_text": {Type: jsonschema.String},
				"new_text": {Type: jsonschema.String},
			},
			Required: []string{"path", "old_text", "new_text"},
		}, func(args map[string]interface{}) string {
			return RunEdit(args["path"].(string), args["old_text"].(string), args["new_text"].(string))
		})
	sub_message := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: promptpkg.SUBANEGT_SYSTEM_PROMOT,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}
	roundsSinceTodo := 0
	for i := 0; i < 30; i++ {
		req := openai.ChatCompletionRequest{
			Model:       common.ModelID,
			Messages:    sub_message,
			Tools:       pr.GetTools(),
			MaxTokens:   4000,
			Temperature: 0.1,
		}
		ctx := context.Background()
		resp, err := common.Client.CreateChatCompletion(ctx, req)
		if err != nil {
			return fmt.Sprintf("API Error: %v\n", err)
		}
		msg := resp.Choices[0].Message
		sub_message = append(sub_message, msg)
		if len(msg.ToolCalls) == 0 {
			break
		}
		usedTodo := false
		for _, toolCall := range msg.ToolCalls {
			res := pr.HandleToolCall(toolCall)
			sub_message = append(sub_message, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    res,
				ToolCallID: toolCall.ID,
			})
		}
		if usedTodo {
			roundsSinceTodo = 0
		} else {
			roundsSinceTodo++
		}
		if roundsSinceTodo >= 3 {
			sub_message = append(sub_message, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: "<reminder>Update your todos.</reminder>",
			})
			// 重置计数器，避免连续轰炸
			roundsSinceTodo = 0
		}
	}
	lastMsg := sub_message[len(sub_message)-1]
	if lastMsg.Role == openai.ChatMessageRoleAssistant && lastMsg.Content != "" {
		return lastMsg.Content
	}
	return "(no summary)"
}

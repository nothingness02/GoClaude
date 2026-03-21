package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/GoClaude/common"
	sys_promot "github.com/GoClaude/prompt"
	"github.com/GoClaude/skills"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

func safePath(p string) (string, error) {
	target := filepath.Join(common.WorkDir, p)
	target = filepath.Clean(target)
	if !strings.HasPrefix(target, common.WorkDir) {
		return "", fmt.Errorf("path escapes workspace: %s", p)
	}
	return target, nil
}

func runBash(command string) string {
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

func runRead(p string, limit float64) string {
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
func runWrite(p, content string) string {
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

func runEdit(p, oldText, newText string) string {
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

func run_subagent(prompt string, description string) string {
	fmt.Printf("\n[+] Spawning Subagent for: %s\n", description)
	sub_message := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: sys_promot.SUBANEGT_SYSTEM_PROMOT,
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
			Tools:       base_tools,
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
			res := HandleToolCall(toolCall, &usedTodo)
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

var base_tools = []openai.Tool{
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "bash",
			Description: "Run a shell command.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"command": {Type: jsonschema.String},
				},
				Required: []string{"command"},
			},
		},
	}, {
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "read_file",
			Description: "Read file contents.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"path":  {Type: jsonschema.String},
					"limit": {Type: jsonschema.Number},
				},
				Required: []string{"path"},
			},
		},
	}, {
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "write_file",
			Description: "Write content to file.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"path":    {Type: jsonschema.String},
					"content": {Type: jsonschema.String},
				},
				Required: []string{"path", "content"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "edit_file",
			Description: "Replace exact text in file.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"path":     {Type: jsonschema.String},
					"old_text": {Type: jsonschema.String},
					"new_text": {Type: jsonschema.String},
				},
				Required: []string{"path", "old_text", "new_text"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "load_skill",
			Description: "Load specialized knowledge by name.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"name": {Type: jsonschema.String, Description: "Skill name to load"},
				},
				Required: []string{"name"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "todo",
			Description: "Update task list. Track progress on multi-step tasks.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"items": {
						Type: jsonschema.Array,
						Items: &jsonschema.Definition{
							Type: jsonschema.Object,
							Properties: map[string]jsonschema.Definition{
								"id":   {Type: jsonschema.String},
								"text": {Type: jsonschema.String},
								"status": {
									Type: jsonschema.String,
									Enum: []string{"pending", "in_progress", "completed"},
								},
							},
							Required: []string{"id", "text", "status"},
						},
					},
				},
				Required: []string{"items"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "create_task",
			Description: "Create a new tracked task.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"subject":     {Type: jsonschema.String},
					"description": {Type: jsonschema.String},
				}, Required: []string{"subject"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "get_task",
			Description: "Get JSON details of a specific task.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"task_id": {Type: jsonschema.Integer},
				},
				Required: []string{"task_id"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "update_task",
			Description: "Update task status or dependencies.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"task_id":        {Type: jsonschema.Integer},
					"status":         {Type: jsonschema.String, Enum: []string{"pending", "in_progress", "completed"}},
					"add_blocked_by": {Type: jsonschema.Array, Items: &jsonschema.Definition{Type: jsonschema.Integer}},
					"add_blocks":     {Type: jsonschema.Array, Items: &jsonschema.Definition{Type: jsonschema.Integer}},
				},
				Required: []string{"task_id"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "list_tasks",
			Description: "View all tasks and their current blocking status.",
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "run_background",
			Description: "Run a long-running bash command in the background. Returns a task ID immediately.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"command": {Type: jsonschema.String},
				},
				Required: []string{"command"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "check_background",
			Description: "Check the status and get the output of background tasks. Omit task_id to list all tasks.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"task_id": {Type: jsonschema.String, Description: "Optional ID of the task to check."},
				},
			},
		},
	},
}
var parent_tools = append(base_tools,
	openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "task",
			Description: "Spawn a subagent with fresh context. It shares the filesystem but not conversation history.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"prompt":      {Type: jsonschema.String},
					"description": {Type: jsonschema.String, Description: "Short description of the task"},
				},
				Required: []string{"prompt"},
			},
		},
	},
)

func GetBaseTools() []openai.Tool {
	return base_tools
}

func GetALLTools() []openai.Tool {
	return parent_tools
}

var todoManager = NewTodoManager()
var taskManager, _ = NewTaskManager(common.TASKS_DIR) //todos:错误的处理

// todos: 等待完善真正并行的sub_agent
func HandleToolCall(call openai.ToolCall, usedtodo *bool) string {
	var args map[string]interface{}
	json.Unmarshal([]byte(call.Function.Arguments), &args)
	var output string
	switch call.Function.Name {
	case "bash":
		output = runBash(args["command"].(string))
	case "read_file":
		limit := 0.0
		if l, ok := args["limit"].(float64); ok {
			limit = l
		}
		output = runRead(args["path"].(string), limit)
	case "write_file":
		output = runWrite(args["path"].(string), args["content"].(string))
	case "edit_file":
		output = runEdit(args["path"].(string), args["old_text"].(string), args["new_text"].(string))
	case "load_skill":
		output = skills.GlobalSkillLoader.GetContent(args["name"].(string))
	case "todo":
		*usedtodo = true
		var items []TodoItem
		rawItems, _ := json.Marshal(args["items"])
		json.Unmarshal(rawItems, &items)
		out, err := todoManager.Update(items)
		if err != nil {
			output = fmt.Sprintf("Error: %v", err)
		} else {
			output = out
		}
	case "task":
		prompt := args["prompt"].(string)
		desc := "No description"
		if d, ok := args["description"].(string); ok {
			desc = d
		}
		output = run_subagent(prompt, desc)
	case "create_task":
		desc := ""
		if d, ok := args["description"].(string); ok {
			desc = d
		}
		output = taskManager.Create(args["subject"].(string), desc)
	case "get_task":
		output = taskManager.Get(int(args["task_id"].(float64)))
	case "update_task":
		taskID := int(args["task_id"].(float64))
		status, _ := args["status"].(string)
		var blockedBy []int
		if vals, ok := args["add_blocked_by"].([]interface{}); ok {
			for _, v := range vals {
				blockedBy = append(blockedBy, int(v.(float64)))
			}
		}

		var blocks []int
		if vals, ok := args["add_blocks"].([]interface{}); ok {
			for _, v := range vals {
				blocks = append(blocks, int(v.(float64)))
			}
		}
		output = taskManager.Update(taskID, status, blockedBy, blocks)
	case "list_tasks":
		output = taskManager.ListAll()
	default:
		output = fmt.Sprintf("Unknown tool: %s", call.Function.Name)
	}
	preview := output
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	fmt.Printf("> %s: %s\n", call.Function.Name, strings.ReplaceAll(preview, "\n", " "))
	return output
}

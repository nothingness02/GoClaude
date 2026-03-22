package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/GoClaude/common"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

type Member struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}
type TeamConfig struct {
	TeamName string    `json:"team_name"`
	Members  []*Member `json:"members"`
}
type TeammateManager struct {
	Dir        string
	ConfigPath string
	Config     TeamConfig
	mu         sync.Mutex
	bus        *MessageBus
}

func NewTeammateManager(dir string, bus *MessageBus) *TeammateManager {
	os.MkdirAll(dir, 0755)
	configPath := filepath.Join(dir, "config.json")

	tm := &TeammateManager{
		Dir:        dir,
		ConfigPath: configPath,
		bus:        bus,
	}
	tm.loadConfig()
	return tm
}

// --------------------队友工具定义--------------
var teammateTools = []openai.Tool{
	base_tools[0], // bash (假设你已经在 main.go 之前定义好了 tools)
	base_tools[1], // read_file
	base_tools[2], // write_file
	base_tools[3], // edit_file
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "send_message",
			Description: "Send message to a teammate.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"to":       {Type: jsonschema.String},
					"content":  {Type: jsonschema.String},
					"msg_type": {Type: jsonschema.String, Enum: []string{"message", "broadcast"}},
				},
				Required: []string{"to", "content"},
			},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "read_inbox",
			Description: "Read and drain your inbox.",
			Parameters:  jsonschema.Definition{Type: jsonschema.Object},
		},
	},
	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "shutdown_response",
			Description: "Respond to a shutdown request. Approve to shut down, reject to keep working.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"request_id": {Type: jsonschema.String},
					"approve":    {Type: jsonschema.Boolean},
					"reason":     {Type: jsonschema.String},
				},
				Required: []string{"request_id", "approve"},
			},
		},
	},

	{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "plan_approval",
			Description: "Submit a plan for lead approval. Provide plan text.",
			Parameters: jsonschema.Definition{
				Type: jsonschema.Object,
				Properties: map[string]jsonschema.Definition{
					"plan": {Type: jsonschema.String},
				},
				Required: []string{"plan"},
			},
		},
	},
}

// --------------------核心逻辑 --------------—-
// todos:
// 1.支持Msg结构体Defs参数的传递
// 2. 精简传送参数，但是不遗漏信息
func (tm *TeammateManager) Spawn(name, role, prompt string) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	member := tm.findMember(name)
	if member != nil {
		if member.Status != "idle" && member.Status != "shutdown" {
			return fmt.Sprintf("Error: '%s' is currently %s", name, member.Status)
		}
		member.Status = "working"
		member.Role = role
	} else {
		member = &Member{Name: name, Role: role, Status: "working"}
		tm.Config.Members = append(tm.Config.Members, member)
	}
	tm.saveConfig()
	go tm.teammateLoop(name, role, prompt)
	return fmt.Sprintf("Spawned '%s' (role: %s)", name, role)

}

func (tm *TeammateManager) teammateLoop(name, role, prompt string) {
	sysPrompt := fmt.Sprintf(
		"You are '%s', role: %s, at %s. Use send_message to communicate. "+
			"Submit plans via plan_approval before major work. "+
			"Respond to shutdown_request with shutdown_response.",
		name, role, common.WorkDir)
	shouldExit := false
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: prompt},
	}
	for i := 0; i < 50; i++ {
		if shouldExit {
			break // 检测到同意退出，平滑终止 Goroutine
		}
		inbox := tm.bus.ReadInbox(name)
		for _, msg := range inbox {
			msgData, _ := json.Marshal(msg)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("<incoming_message>\n%s\n</incoming_message>", string(msgData)),
			})
		}
		req := openai.ChatCompletionRequest{
			Model:       common.ModelID,
			Messages:    messages,
			Tools:       teammateTools,
			MaxTokens:   4000,
			Temperature: 0.2,
		}
		ctx := context.Background()
		resp, err := common.Client.CreateChatCompletion(ctx, req)
		if err != nil {
			fmt.Printf("Teammate [%s] API Error: %v\n", name, err)
			break
		}
		respMsg := resp.Choices[0].Message
		messages = append(messages, respMsg)
		if len(respMsg.ToolCalls) == 0 {
			break // 任务完成或没有调用工具
		}
		for _, toolCall := range respMsg.ToolCalls {
			var args map[string]interface{}
			json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

			var output string
			switch toolCall.Function.Name {
			case "bash":
				output = runBash(args["command"].(string))
			case "read_file":
				output = runRead(args["path"].(string), 0)
			case "write_file":
				output = runWrite(args["path"].(string), args["content"].(string))
			case "edit_file":
				output = runEdit(args["path"].(string), args["old_text"].(string), args["new_text"].(string))
			case "send_message":
				msgType := "message"
				if t, ok := args["msg_type"].(string); ok {
					msgType = t
				}
				output = tm.bus.Send(name, args["to"].(string), args["content"].(string), msgType, nil)
			case "read_inbox":
				msgs := tm.bus.ReadInbox(name)
				data, _ := json.MarshalIndent(msgs, "", "  ")
				output = string(data)
			case "shutdown_response":
				reqID := args["request_id"].(string)
				approve := args["approve"].(bool)
				reason := ""
				if r, ok := args["reason"].(string); ok {
					reason = r
				}
				trackerMu.Lock()
				if req, exists := shutdownRequests[reqID]; exists {
					if approve {
						req.Status = "approved"
					} else {
						req.Status = "rejected"
					}
				}
				trackerMu.Unlock()
				extra := map[string]interface{}{"request_id": reqID, "approve": approve}
				tm.bus.Send(name, "lead", reason, "shutdown_response", extra)
				if approve {
					shouldExit = true
					output = "Shutdown approved. Preparing to exit."
				} else {
					output = "Shutdown rejected. Continuing work."
				}
			case "plan_approval":
				planText := args["plan"].(string)
				reqID := shortID()

				trackerMu.Lock()
				planRequests[reqID] = &PlanReq{From: name, Plan: planText, Status: "pending"}
				trackerMu.Unlock()

				extra := map[string]interface{}{"request_id": reqID, "plan": planText}
				tm.bus.Send(name, "lead", planText, "plan_approval_request", extra)

				output = fmt.Sprintf("Plan submitted (request_id=%s). Waiting for lead approval via inbox.", reqID)
			default:
				output = fmt.Sprintf("Unknown tool: %s", toolCall.Function.Name)
			}
			preview := output
			if len(preview) > 120 {
				preview = preview[:120] + "..."
			}
			fmt.Printf("  [%s] %s: %s\n", name, toolCall.Function.Name, strings.ReplaceAll(preview, "\n", " "))
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    output,
				ToolCallID: toolCall.ID,
			})
		}
	}
	tm.mu.Lock()
	if member := tm.findMember(name); member != nil {
		if shouldExit {
			member.Status = "shutdown"
		} else if member.Status != "shutdown" {
			member.Status = "idle"
		}
	}
	tm.saveConfig()
	tm.mu.Unlock()
}

func (tm *TeammateManager) ListAll() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.Config.Members) == 0 {
		return "No teammates."
	}
	lines := []string{fmt.Sprintf("Team: %s", tm.Config.TeamName)}
	for _, m := range tm.Config.Members {
		lines = append(lines, fmt.Sprintf("  %s (%s): %s", m.Name, m.Role, m.Status))
	}
	return strings.Join(lines, "\n")
}

// -------------------- util ---------------- 工具

func (tm *TeammateManager) loadConfig() {
	data, err := os.ReadFile(tm.ConfigPath)
	if err == nil {
		json.Unmarshal(data, &tm.Config)
	} else {
		tm.Config = TeamConfig{TeamName: "default", Members: []*Member{}}
	}
}

func (tm *TeammateManager) saveConfig() {
	data, _ := json.MarshalIndent(tm.Config, "", "")
	os.WriteFile(tm.ConfigPath, data, 0644)
}

func (tm *TeammateManager) findMember(name string) *Member {
	for _, m := range tm.Config.Members {
		if m.Name == name {
			return m
		}
	}
	return nil
}

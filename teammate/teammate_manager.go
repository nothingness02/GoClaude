package teammate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GoClaude/agent/toolprovider"
	"github.com/GoClaude/common"
	"github.com/GoClaude/messagebus"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// Member Teammate 成员
type Member struct {
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// TeamConfig 团队配置
type TeamConfig struct {
	TeamName string    `json:"team_name"`
	Members  []*Member `json:"members"`
}

// TrackerInterface tracker 接口
type TrackerInterface interface {
	Lock()
	Unlock()
	AddPlanRequest(reqID string, from, plan string)
	GetPlanRequest(reqID string) (from, plan string, exists bool)
}

// TaskManagerInterface 任务管理器接口
type TaskManagerInterface interface {
	ScanUnclaimed() []TaskInfo
	Claim(taskID int, assignee string) string
}

// TaskInfo 任务信息
type TaskInfo struct {
	ID          int
	Subject     string
	Description string
}

// TeammateManager Teammate 管理器
type TeammateManager struct {
	Dir          string
	ConfigPath   string
	Config       TeamConfig
	mu           sync.Mutex
	bus          *messagebus.MessageBus
	ToolProvider *toolprovider.ToolProvider
	Tracker      TrackerInterface
	TaskManager  TaskManagerInterface
}

// NewTeammateManager 创建 Teammate 管理器
func NewTeammateManager(dir string, bus *messagebus.MessageBus, tp *toolprovider.ToolProvider) *TeammateManager {
	os.MkdirAll(dir, 0755)
	configPath := filepath.Join(dir, "config.json")

	tm := &TeammateManager{
		Dir:          dir,
		ConfigPath:   configPath,
		bus:          bus,
		ToolProvider: tp,
	}
	tm.loadConfig()
	return tm
}

// SetTracker 设置 tracker
func (tm *TeammateManager) SetTracker(tracker TrackerInterface) {
	tm.Tracker = tracker
}

// SetTaskManager 设置任务管理器
func (tm *TeammateManager) SetTaskManager(tm2 TaskManagerInterface) {
	tm.TaskManager = tm2
}

// -------------------- Teammate 工具定义 --------------------

// GetTeammateTools 获取 Teammate 工具列表
func GetTeammateTools() []openai.Tool {
	return []openai.Tool{
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
		},
		{
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
		},
		{
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
				Description: "Edit a file using regex pattern matching and replacement.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"path":       {Type: jsonschema.String},
						"old_string": {Type: jsonschema.String},
						"new_string": {Type: jsonschema.String},
					},
					Required: []string{"path", "old_string", "new_string"},
				},
			},
		},
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
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "idle",
				Description: "Signal that you have no more work. Enters idle polling phase to look for new tasks.",
				Parameters:  jsonschema.Definition{Type: jsonschema.Object},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "claim_task",
				Description: "Claim a task from the task board by ID.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"task_id": {Type: jsonschema.Integer},
					},
					Required: []string{"task_id"},
				},
			},
		},
	}
}

// -------------------- 核心逻辑 --------------------

// Spawn 启动一个 Teammate
func (tm *TeammateManager) Spawn(name, role, prompt string, tp *toolprovider.ToolProvider) string {
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

	go tm.teammateLoop(name, role, prompt, tp)
	return fmt.Sprintf("Spawned '%s' (role: %s)", name, role)
}

func makeIdentityBlock(name, role string) openai.ChatCompletionMessage {
	return openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: fmt.Sprintf("<identity>You are '%s', role: %s. Continue your work.</identity>", name, role),
	}
}

// teammateLoop Teammate 的心智循环
func (tm *TeammateManager) teammateLoop(name, role, prompt string, tp *toolprovider.ToolProvider) {
	sysPrompt := fmt.Sprintf(
		"You are '%s', role: %s, at %s. Use send_message to communicate. "+
			"Submit plans via plan_approval before major work. "+
			"Respond to shutdown_request with shutdown_response.",
		name, role, common.WorkDir)

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
		{Role: openai.ChatMessageRoleUser, Content: prompt},
	}

	// 使用 ToolProvider 获取工具列表
	tools := tp.GetTools()

	for {
		// ==========================================
		// WORK PHASE (工作模式)
		// ==========================================
		idleRequested := false
		for i := 0; i < 50; i++ {
			inbox := tm.bus.ReadInbox(name)
			for _, msg := range inbox {
				msgData, _ := json.Marshal(msg)
				if msg.Type == "shutdown_request" {
					tm.setStatus(name, "shutdown")
					return // 强制下线
				}
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("<incoming_message>\n%s\n</incoming_message>", string(msgData)),
				})
			}

			req := openai.ChatCompletionRequest{
				Model:       common.ModelID,
				Messages:    messages,
				Tools:       tools,
				MaxTokens:   4000,
				Temperature: 0.2,
			}
			ctx := context.Background()
			resp, err := common.Client.CreateChatCompletion(ctx, req)
			if err != nil {
				tm.setStatus(name, "idle")
				return
			}
			respMsg := resp.Choices[0].Message
			messages = append(messages, respMsg)
			if len(respMsg.ToolCalls) == 0 {
				break // 任务完成或没有调用工具
			}

			for _, toolCall := range respMsg.ToolCalls {
				var output string
				switch toolCall.Function.Name {
				case "bash", "read_file", "write_file", "edit_file", "send_message", "read_inbox", "plan_approval", "idle", "claim_task":
					// 使用工具提供者处理
					if tp != nil {
						output = tp.HandleToolCall(toolCall)
					} else {
						output = "Error: Tool provider not configured"
					}
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

			if toolCall := respMsg.ToolCalls[len(respMsg.ToolCalls)-1]; toolCall.Function.Name == "idle" {
				idleRequested = true
				break
			}
			if idleRequested {
				break // 退出工作循环，进入摸鱼/轮询循环
			}
		}

		// ==========================================
		// IDLE PHASE (闲置轮询模式)
		// ==========================================
		tm.setStatus(name, "idle")
		resumeWork := false
		fmt.Printf("[System] %s entered IDLE state.\n", name)
		for polls := 0; polls < 12; polls++ {
			time.Sleep(5 * time.Second)
			// 检查收件箱
			inbox := tm.bus.ReadInbox(name)
			if len(inbox) > 0 {
				for _, msg := range inbox {
					if msg.Type == "shutdown_request" {
						tm.setStatus(name, "shutdown")
						return
					}
					msgData, _ := json.Marshal(msg)
					messages = append(messages, openai.ChatCompletionMessage{
						Role: openai.ChatMessageRoleUser, Content: string(msgData),
					})
				}
				resumeWork = true
				break
			}

			// B. 去任务看板看看有没有能做的活儿
			if tm.TaskManager != nil {
				unclaimedTasks := tm.TaskManager.ScanUnclaimed()
				if len(unclaimedTasks) > 0 {
					task := unclaimedTasks[0]
					tm.TaskManager.Claim(task.ID, name)

					taskPrompt := fmt.Sprintf("<auto-claimed>Task #%d: %s\n%s</auto-claimed>", task.ID, task.Subject, task.Description)

					// 身份重塑：防止上下文过长导致遗忘
					if len(messages) <= 3 { // 如果被自动压缩过，塞入强身份
						messages = append([]openai.ChatCompletionMessage{makeIdentityBlock(name, role)}, messages...)
					}

					messages = append(messages, openai.ChatCompletionMessage{
						Role: openai.ChatMessageRoleUser, Content: taskPrompt,
					})

					fmt.Printf("=> [Autonomy] %s automatically claimed Task #%d\n", name, task.ID)
					resumeWork = true
					break
				}
			}
		}
		if !resumeWork {
			fmt.Printf("[System] %s timed out. Shutting down.\n", name)
			tm.setStatus(name, "shutdown")
			return
		}
		tm.setStatus(name, "working")
	}
}

// ListAll 列出所有 Teammate
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

// -------------------- 工具函数 --------------------

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

func (tm *TeammateManager) GetMemberStatus(name string) string {
	for _, m := range tm.Config.Members {
		if m.Name == name {
			return m.Status
		}
	}
	return "not have this member"
}

func (tm *TeammateManager) setStatus(name, status string) {
	task := tm.findMember(name)
	if task == nil {
		return
	}
	task.Status = status

	// 如果设置为 shutdown，广播通知给所有其他成员
	if status == "shutdown" {
		// 获取所有非自己的活跃成员
		var others []string
		for _, m := range tm.Config.Members {
			if m.Name != name && m.Status != "shutdown" {
				others = append(others, m.Name)
			}
		}
		// 广播关闭通知
		tm.bus.BroadcastShutdown(name, others)
	}
}

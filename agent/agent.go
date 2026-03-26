package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/GoClaude/agent/toolprovider"
	"github.com/GoClaude/backgroudtask"
	"github.com/GoClaude/common"
	"github.com/GoClaude/compact"
	"github.com/GoClaude/messagebus"
	"github.com/GoClaude/tasks"
	"github.com/GoClaude/teammate"
	"github.com/GoClaude/todo"
	"github.com/sashabaranov/go-openai"
)

var taskManager *tasks.TaskManager
var MsgBus *messagebus.MessageBus
var teamManager *teammate.TeammateManager

func Init() {
	var err error
	taskManager, err = tasks.NewTaskManager(common.TASKS_DIR)
	if err != nil {
		fmt.Println("任务面板启动失败")
		return
	}
	MsgBus, err = messagebus.NewMessageBus(common.INBOX_DIR)
	if err != nil {
		fmt.Println("消息总线启动失败")
		return
	}
	teamManager = teammate.NewTeammateManager(common.TEAM_DIR, MsgBus, nil)
}

// AgentConfig Agent运行配置
type AgentInstance struct {
	Name             string
	Model            string
	MaxTokens        int
	Temperature      float32
	MaxRounds        int
	EnableCompaction bool
	EnableTodoRemind bool
	ToolProvider     *toolprovider.ToolProvider
	ToolWhitelist    []string // 白名单，为空时使用全部工具

	// 实例依赖（不再需要全局配置）
	BgManager *backgroudtask.BackgroundManager // 后台任务管理器
	TodoMgr   *todo.TodoManager                // Todo 管理器
	MsgBus    *messagebus.MessageBus           // 消息总线
	TaskMgr   *tasks.TaskManager               // 任务管理器
	TeamMgr   *teammate.TeammateManager        // 团队管理器
}

type Option func(*AgentInstance)

func WithName(name string) Option {
	return func(ai *AgentInstance) {
		ai.Name = name
		ai.ToolProvider = InitLeaderToolProvider(name, ai.BgManager, ai.TodoMgr, ai.MsgBus, ai.TaskMgr, ai.TeamMgr)
	}
}

func WithModel(model string) Option {
	return func(ai *AgentInstance) {
		ai.Model = model
	}
}

func WithMaxTokens(maxToken int) Option {
	return func(ai *AgentInstance) {
		ai.MaxTokens = maxToken
	}
}

// DefaultAgentInstance 返回默认配置
func DefaultAgentInstance() *AgentInstance {
	bg := backgroudtask.NewBackgroundManager(common.WorkDir)
	todoMgr := todo.NewTodoManager()
	return &AgentInstance{
		Name:             "leader",
		Model:            common.ModelID,
		MaxTokens:        4000,
		Temperature:      0.2,
		MaxRounds:        30,
		EnableCompaction: true,
		EnableTodoRemind: true,
		ToolProvider:     InitLeaderToolProvider("leader", bg, todoMgr, MsgBus, taskManager, teamManager),
		ToolWhitelist:    nil, // 使用全局全部工具

		// 实例依赖
		BgManager: bg,
		TodoMgr:   todoMgr,
		MsgBus:    MsgBus,
		TaskMgr:   taskManager,
		TeamMgr:   teamManager,
	}
}

func NewAgentInstance(opts ...Option) *AgentInstance {
	agentinstance := DefaultAgentInstance()
	for _, op := range opts {
		op(agentinstance)
	}
	return agentinstance
}

// Notify 通知 Agent 后台任务完成
func Notify(BgManager *backgroudtask.BackgroundManager, messages *[]openai.ChatCompletionMessage) {
	notifs := BgManager.DrainNotifications()
	if len(notifs) > 0 {
		b, _ := json.MarshalIndent(notifs, "", "  ")
		sysEventMsg := fmt.Sprintf("<background_notifications>\n%s\n</background_notifications>\nReview these completed tasks and update your plans accordingly.", string(b))

		*messages = append(*messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: sysEventMsg,
		})
	}
}

// RunAgentLoop 主Agent心智循环
// messages: 消息历史指针，会被修改
// 返回: 是否正常结束
func RunAgentLoop(messages *[]openai.ChatCompletionMessage) bool {
	return RunAgentLoopWithConfig(messages, DefaultAgentInstance())
}

// 主agent没有外循环的上限
// RunAgentLoopWithConfig 带配置的主Agent心智循环
func RunAgentLoopWithConfig(messages *[]openai.ChatCompletionMessage, agent *AgentInstance) bool {
	roundsSinceTodo := 0
	model := agent.Model
	for round := 0; round < agent.MaxRounds || agent.MaxRounds <= 0; round++ {
		// 消息压缩
		if agent.EnableCompaction {
			compact.Micro_compact(messages)
			if compact.EstimateTokens(messages) > common.THRESHOLD {
				compact.Auto_compact(messages)
			}
		}

		// 通知后台任务完成（使用实例依赖）
		Notify(agent.BgManager, messages)

		req := openai.ChatCompletionRequest{
			Model:       model,
			Messages:    *messages,
			Tools:       agent.ToolProvider.GetTools(),
			MaxTokens:   agent.MaxTokens,
			Temperature: agent.Temperature,
		}

		ctx := context.Background()
		resp, err := common.Client.CreateChatCompletion(ctx, req)
		if err != nil {
			fmt.Printf("API Error: %v\n", err)
			return false
		}

		msg := resp.Choices[0].Message
		*messages = append(*messages, msg)

		// 如果没有调用工具，则回合结束
		if len(msg.ToolCalls) == 0 {
			return true
		}

		usedTodo := false
		// 处理工具调用
		for _, toolCall := range msg.ToolCalls {
			if toolCall.Function.Name == "todo" {
				usedTodo = true
			}
			output := agent.ToolProvider.HandleToolCall(toolCall)
			// 将工具执行结果追加到消息记录
			*messages = append(*messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    output,
				ToolCallID: toolCall.ID,
			})
		}

		if agent.EnableTodoRemind {
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

	return false // 达到最大轮次限制
}

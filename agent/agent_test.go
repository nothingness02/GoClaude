package agent

import (
	"testing"

	"github.com/GoClaude/backgroudtask"
	"github.com/GoClaude/messagebus"
	"github.com/GoClaude/tasks"
	"github.com/GoClaude/teammate"
	"github.com/GoClaude/todo"
)

func TestAgentInstance_Dependencies(t *testing.T) {
	// 初始化全局依赖
	Init()

	// 验证 AgentInstance 结构体包含所有必需依赖字段
	agent := DefaultAgentInstance()

	// 验证 Name 字段
	if agent.Name != "leader" {
		t.Errorf("Expected Name to be 'leader', got '%s'", agent.Name)
	}

	// 验证 Model 配置
	if agent.Model == "" {
		t.Error("Expected Model to be set")
	}

	// 验证 BgManager (后台任务管理器)
	if agent.BgManager == nil {
		t.Error("Expected BgManager to be not nil")
	}
	if _, ok := interface{}(agent.BgManager).(*backgroudtask.BackgroundManager); !ok {
		t.Error("Expected BgManager to be of type *backgroudtask.BackgroundManager")
	}

	// 验证 TodoMgr (Todo 管理器)
	if agent.TodoMgr == nil {
		t.Error("Expected TodoMgr to be not nil")
	}
	if _, ok := interface{}(agent.TodoMgr).(*todo.TodoManager); !ok {
		t.Error("Expected TodoMgr to be of type *todo.TodoManager")
	}

	// 验证 MsgBus (消息总线)
	if agent.MsgBus == nil {
		t.Error("Expected MsgBus to be not nil")
	}
	if _, ok := interface{}(agent.MsgBus).(*messagebus.MessageBus); !ok {
		t.Error("Expected MsgBus to be of type *messagebus.MessageBus")
	}

	// 验证 TaskMgr (任务管理器)
	if agent.TaskMgr == nil {
		t.Error("Expected TaskMgr to be not nil")
	}
	if _, ok := interface{}(agent.TaskMgr).(*tasks.TaskManager); !ok {
		t.Error("Expected TaskMgr to be of type *tasks.TaskManager")
	}

	// 验证 TeamMgr (团队管理器)
	if agent.TeamMgr == nil {
		t.Error("Expected TeamMgr to be not nil")
	}
	if _, ok := interface{}(agent.TeamMgr).(*teammate.TeammateManager); !ok {
		t.Error("Expected TeamMgr to be of type *teammate.TeammateManager")
	}

	// 验证 ToolProvider
	if agent.ToolProvider == nil {
		t.Error("Expected ToolProvider to be not nil")
	}
}

func TestAgentInstance_Config(t *testing.T) {
	agent := DefaultAgentInstance()

	// 验证默认配置
	if agent.MaxTokens != 4000 {
		t.Errorf("Expected MaxTokens to be 4000, got %d", agent.MaxTokens)
	}
	if agent.Temperature != 0.2 {
		t.Errorf("Expected Temperature to be 0.2, got %f", agent.Temperature)
	}
	if agent.MaxRounds != 30 {
		t.Errorf("Expected MaxRounds to be 30, got %d", agent.MaxRounds)
	}
	if !agent.EnableCompaction {
		t.Error("Expected EnableCompaction to be true")
	}
	if !agent.EnableTodoRemind {
		t.Error("Expected EnableTodoRemind to be true")
	}
	if agent.ToolWhitelist != nil {
		t.Error("Expected ToolWhitelist to be nil")
	}
}

func TestToolProviderRegistration(t *testing.T) {
	// 初始化全局依赖
	Init()

	agent := DefaultAgentInstance()

	// 验证工具提供者已注册工具
	tools := agent.ToolProvider.List()

	// 验证基础工具已注册
	expectedTools := []string{
		"bash",
		"read_file",
		"write_file",
		"edit_file",
		"load_skill",
		"todo",
		"create_task",
		"get_task",
		"update_task",
		"list_tasks",
		"run_background",
		"check_background",
		"sub_agent",
		"send_message",
		"spawn_teammate",
		"list_teammates",
		"read_inbox",
		"check_shutdown_status",
	}

	for _, expected := range expectedTools {
		found := false
		for _, tool := range tools {
			if tool == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool '%s' to be registered", expected)
		}
	}

	t.Logf("Total tools registered: %d", len(tools))
}

func TestAgentInstanceOpts(t *testing.T) {
	Init()
	agent := NewAgentInstance(WithName("master"),
		WithMaxTokens(50000),
		WithModel("qwen"))
	if agent.Name != "master" {
		t.Fatalf(" Name options fail")
	}
	if agent.Model != "qwen" {
		t.Fatalf(" Name options fail")
	}
	if agent.MaxTokens != 50000 {
		t.Fatalf(" Name options fail")
	}
	t.Logf("options all success")
}

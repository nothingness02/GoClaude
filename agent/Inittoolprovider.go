package agent

import (
	"encoding/json"
	"fmt"

	"github.com/GoClaude/agent/toolprovider"
	"github.com/GoClaude/backgroudtask"
	"github.com/GoClaude/messagebus"
	"github.com/GoClaude/skills"
	"github.com/GoClaude/tasks"
	"github.com/GoClaude/teammate"
	"github.com/GoClaude/todo"
	tools "github.com/GoClaude/tools"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// InitToolProvider 初始化工具提供者，注册所有内置工具
func InitLeaderToolProvider(name string, leaderBgManager *backgroudtask.BackgroundManager, todoMgr *todo.TodoManager, msgBus *messagebus.MessageBus, taskMgr *tasks.TaskManager, teamMgr *teammate.TeammateManager) *toolprovider.ToolProvider {
	provider := toolprovider.NewToolProvider()
	// 注册基础工具
	provider.Register("bash", "Run a shell command.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String},
			},
			Required: []string{"command"},
		}, func(args map[string]interface{}) string {
			return tools.RunBash(args["command"].(string))
		})

	provider.Register("read_file", "Read file contents.",
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
			return tools.RunRead(args["path"].(string), limit)
		})

	provider.Register("write_file", "Write content to file.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":    {Type: jsonschema.String},
				"content": {Type: jsonschema.String},
			},
			Required: []string{"path", "content"},
		}, func(args map[string]interface{}) string {
			return tools.RunWrite(args["path"].(string), args["content"].(string))
		})

	provider.Register("edit_file", "Replace exact text in file.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":     {Type: jsonschema.String},
				"old_text": {Type: jsonschema.String},
				"new_text": {Type: jsonschema.String},
			},
			Required: []string{"path", "old_text", "new_text"},
		}, func(args map[string]interface{}) string {
			return tools.RunEdit(args["path"].(string), args["old_text"].(string), args["new_text"].(string))
		})

	provider.Register("load_skill", "Load specialized knowledge by name.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Skill name to load"},
			},
			Required: []string{"name"},
		}, func(args map[string]interface{}) string {
			return skills.GlobalSkillLoader.GetContent(args["name"].(string))
		})

	provider.Register("todo", "Update task list. Track progress on multi-step tasks.",
		jsonschema.Definition{
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
		}, func(args map[string]interface{}) string {
			var items []todo.TodoItem
			rawItems, _ := json.Marshal(args["items"])
			json.Unmarshal(rawItems, &items)
			out, err := todoMgr.Update(items)
			if err != nil {
				return fmt.Sprintf("Error: %v", err)
			}
			return out
		})

	provider.Register("create_task", "Create a new tracked task.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"subject":     {Type: jsonschema.String},
				"description": {Type: jsonschema.String},
			}, Required: []string{"subject"},
		}, func(args map[string]interface{}) string {
			desc := ""
			if d, ok := args["description"].(string); ok {
				desc = d
			}
			return taskMgr.Create(args["subject"].(string), desc)
		})

	provider.Register("get_task", "Get JSON details of a specific task.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"task_id": {Type: jsonschema.Integer},
			},
			Required: []string{"task_id"},
		}, func(args map[string]interface{}) string {
			return taskMgr.Get(int(args["task_id"].(float64)))
		})

	provider.Register("update_task", "Update task status or dependencies.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"task_id":        {Type: jsonschema.Integer},
				"status":         {Type: jsonschema.String, Enum: []string{"pending", "in_progress", "completed"}},
				"add_blocked_by": {Type: jsonschema.Array, Items: &jsonschema.Definition{Type: jsonschema.Integer}},
				"add_blocks":     {Type: jsonschema.Array, Items: &jsonschema.Definition{Type: jsonschema.Integer}},
			},
			Required: []string{"task_id"},
		}, func(args map[string]interface{}) string {
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
			return taskMgr.Update(taskID, status, blockedBy, blocks)
		})

	provider.Register("list_tasks", "View all tasks and their current blocking status.",
		jsonschema.Definition{
			Type: jsonschema.Object,
		}, func(args map[string]interface{}) string {
			return taskMgr.ListAll()
		})

	provider.Register("run_background", "Run a long-running bash command in the background. Returns a task ID immediately.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String},
			},
			Required: []string{"command"},
		}, func(args map[string]interface{}) string {
			return leaderBgManager.Run(args["command"].(string))
		})

	provider.Register("check_background", "Check the status and get the output of background tasks. Omit task_id to list all tasks.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"task_id": {Type: jsonschema.String, Description: "Optional ID of the task to check."},
			},
		}, func(args map[string]interface{}) string {
			taskID := ""
			if tid, ok := args["task_id"].(string); ok {
				taskID = tid
			}
			return leaderBgManager.Check(taskID)
		})

	// 注册父级工具（需要 subagent 上下文的工具）
	provider.Register("sub_agent", "Spawn a subagent with fresh context. It shares the filesystem but not conversation history.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"prompt":      {Type: jsonschema.String},
				"description": {Type: jsonschema.String, Description: "Short description of the task"},
			},
			Required: []string{"prompt"},
		}, func(args map[string]interface{}) string {
			prompt := args["prompt"].(string)
			desc := "No description"
			if d, ok := args["description"].(string); ok {
				desc = d
			}
			return tools.RunSubagent(prompt, desc)
		})
	provider.Register("send_message", "Communicate with other agents. Use 'message' for chat, 'broadcast' for all agents, 'shutdown_request' to stop an agent, 'plan_approval_request' to seek permission, and 'plan_approval_response' to reply. Requires target 'to' and 'content'.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"to":      {Type: jsonschema.String},
				"content": {Type: jsonschema.String},
				"msg_type": {Type: jsonschema.String, Enum: []string{"message",
					"broadcast",
					"shutdown_request",
					"plan_approval_request",
					"plan_approval_response"}},
			},
			Required: []string{"to", "content", "msg_type"},
		}, func(args map[string]interface{}) string {
			to := args["to"].(string)
			content := args["content"].(string)
			msgType := "message"
			if mt, ok := args["msg_type"].(string); ok {
				msgType = mt
			}
			result := msgBus.Send(name, to, content, msgType, nil)
			return result
		})
	provider.Register("spawn_teammate",
		"Spawn a persistent teammate agent running in the background.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name":   {Type: jsonschema.String},
				"role":   {Type: jsonschema.String},
				"prompt": {Type: jsonschema.String, Description: "Initial instructions for the teammate."},
			},
			Required: []string{"name", "role", "prompt"},
		},
		func(args map[string]interface{}) string {
			out := teamMgr.Spawn(args["name"].(string), args["role"].(string), args["prompt"].(string), InitTeammateToolProvider(name, msgBus, taskMgr))
			return out
		})
	provider.Register("list_teammates", "List all teammates and their statuses.", jsonschema.Definition{Type: jsonschema.Object},
		func(args map[string]interface{}) string {
			out := teamMgr.ListAll()
			return out
		})
	provider.Register("read_inbox", "Read and drain your inbox.",
		jsonschema.Definition{Type: jsonschema.Object}, func(args map[string]interface{}) string {
			msgs := msgBus.ReadInbox(name)
			data, _ := json.MarshalIndent(msgs, "", "  ")
			return string(data)
		})
	provider.Register("check_shutdown_status", "Check the status of a previously sent shutdown request.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String},
			},
			Required: []string{"name"},
		}, func(args map[string]interface{}) string {
			name := args["name"].(string)
			status := teamMgr.GetMemberStatus(name)
			return fmt.Sprintf("the role is %s", status)
		})
	return provider
}

// InitTeammateToolProvider 初始化 Teammate 的工具提供者
// name: teammate 名称，用于 read_inbox、send_message、claim_task 等工具
func InitTeammateToolProvider(name string, bus *messagebus.MessageBus, taskMgr *tasks.TaskManager) *toolprovider.ToolProvider {
	provider := toolprovider.NewToolProvider()

	// 注册基础工具
	provider.Register("bash", "Run a shell command.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String},
			},
			Required: []string{"command"},
		}, func(args map[string]interface{}) string {
			return tools.RunBash(args["command"].(string))
		})

	provider.Register("read_file", "Read file contents.",
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
			return tools.RunRead(args["path"].(string), limit)
		})

	provider.Register("write_file", "Write content to file.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":    {Type: jsonschema.String},
				"content": {Type: jsonschema.String},
			},
			Required: []string{"path", "content"},
		}, func(args map[string]interface{}) string {
			return tools.RunWrite(args["path"].(string), args["content"].(string))
		})

	provider.Register("edit_file", "Replace exact text in file.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":     {Type: jsonschema.String},
				"old_text": {Type: jsonschema.String},
				"new_text": {Type: jsonschema.String},
			},
			Required: []string{"path", "old_text", "new_text"},
		}, func(args map[string]interface{}) string {
			return tools.RunEdit(args["path"].(string), args["old_text"].(string), args["new_text"].(string))
		})

	// 注册 teammate 通信工具
	// 使用闭包捕获的 name 参数
	provider.Register("send_message", "Communicate with other agents. Use 'message' for chat, 'broadcast' for all agents, 'shutdown_request' to stop an agent, 'plan_approval_request' to seek permission, and 'plan_approval_response' to reply. Requires target 'to' and 'content'.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"to":      {Type: jsonschema.String},
				"content": {Type: jsonschema.String},
				"msg_type": {Type: jsonschema.String,
					Enum: []string{"message", "broadcast",
						"plan_approval_request",
						"plan_approval_response"}},
			},
			Required: []string{"to", "content", "msg_type"},
		}, func(args map[string]interface{}) string {
			to := args["to"].(string)
			content := args["content"].(string)
			msgType := "message"
			if mt, ok := args["msg_type"].(string); ok {
				msgType = mt
			}
			return bus.Send(name, to, content, msgType, nil)
		})

	provider.Register("read_inbox", "Read your inbox messages.",
		jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: map[string]jsonschema.Definition{},
		}, func(args map[string]interface{}) string {
			msgs := bus.ReadInbox(name)
			data, _ := json.MarshalIndent(msgs, "", "  ")
			return string(data)
		})
	provider.Register("idle", "Request to enter idle mode.",
		jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: map[string]jsonschema.Definition{},
		}, func(args map[string]interface{}) string {
			return "Entering idle mode"
		})

	provider.Register("claim_task", "Claim a task from the task board.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"task_id": {Type: jsonschema.Integer},
			},
			Required: []string{"task_id"},
		}, func(args map[string]interface{}) string {
			taskID := int(args["task_id"].(float64))
			result := taskMgr.Claim(taskID, name)
			return result
		})

	return provider
}
func InitSubToolProvider(leaderBgManager *backgroudtask.BackgroundManager, todoMgr *todo.TodoManager, taskMgr *tasks.TaskManager) *toolprovider.ToolProvider {
	provider := toolprovider.NewToolProvider()
	// 注册基础工具
	provider.Register("bash", "Run a shell command.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String},
			},
			Required: []string{"command"},
		}, func(args map[string]interface{}) string {
			return tools.RunBash(args["command"].(string))
		})

	provider.Register("read_file", "Read file contents.",
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
			return tools.RunRead(args["path"].(string), limit)
		})

	provider.Register("write_file", "Write content to file.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":    {Type: jsonschema.String},
				"content": {Type: jsonschema.String},
			},
			Required: []string{"path", "content"},
		}, func(args map[string]interface{}) string {
			return tools.RunWrite(args["path"].(string), args["content"].(string))
		})

	provider.Register("edit_file", "Replace exact text in file.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"path":     {Type: jsonschema.String},
				"old_text": {Type: jsonschema.String},
				"new_text": {Type: jsonschema.String},
			},
			Required: []string{"path", "old_text", "new_text"},
		}, func(args map[string]interface{}) string {
			return tools.RunEdit(args["path"].(string), args["old_text"].(string), args["new_text"].(string))
		})

	provider.Register("load_skill", "Load specialized knowledge by name.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"name": {Type: jsonschema.String, Description: "Skill name to load"},
			},
			Required: []string{"name"},
		}, func(args map[string]interface{}) string {
			return skills.GlobalSkillLoader.GetContent(args["name"].(string))
		})

	provider.Register("todo", "Update task list. Track progress on multi-step tasks.",
		jsonschema.Definition{
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
		}, func(args map[string]interface{}) string {
			var items []todo.TodoItem
			rawItems, _ := json.Marshal(args["items"])
			json.Unmarshal(rawItems, &items)
			out, err := todoMgr.Update(items)
			if err != nil {
				return fmt.Sprintf("Error: %v", err)
			}
			return out
		})

	provider.Register("create_task", "Create a new tracked task.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"subject":     {Type: jsonschema.String},
				"description": {Type: jsonschema.String},
			}, Required: []string{"subject"},
		}, func(args map[string]interface{}) string {
			desc := ""
			if d, ok := args["description"].(string); ok {
				desc = d
			}
			return taskMgr.Create(args["subject"].(string), desc)
		})

	provider.Register("get_task", "Get JSON details of a specific task.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"task_id": {Type: jsonschema.Integer},
			},
			Required: []string{"task_id"},
		}, func(args map[string]interface{}) string {
			return taskMgr.Get(int(args["task_id"].(float64)))
		})

	provider.Register("update_task", "Update task status or dependencies.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"task_id":        {Type: jsonschema.Integer},
				"status":         {Type: jsonschema.String, Enum: []string{"pending", "in_progress", "completed"}},
				"add_blocked_by": {Type: jsonschema.Array, Items: &jsonschema.Definition{Type: jsonschema.Integer}},
				"add_blocks":     {Type: jsonschema.Array, Items: &jsonschema.Definition{Type: jsonschema.Integer}},
			},
			Required: []string{"task_id"},
		}, func(args map[string]interface{}) string {
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
			return taskMgr.Update(taskID, status, blockedBy, blocks)
		})

	provider.Register("list_tasks", "View all tasks and their current blocking status.",
		jsonschema.Definition{
			Type: jsonschema.Object,
		}, func(args map[string]interface{}) string {
			return taskMgr.ListAll()
		})

	provider.Register("run_background", "Run a long-running bash command in the background. Returns a task ID immediately.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String},
			},
			Required: []string{"command"},
		}, func(args map[string]interface{}) string {
			return leaderBgManager.Run(args["command"].(string))
		})

	provider.Register("check_background", "Check the status and get the output of background tasks. Omit task_id to list all tasks.",
		jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"task_id": {Type: jsonschema.String, Description: "Optional ID of the task to check."},
			},
		}, func(args map[string]interface{}) string {
			taskID := ""
			if tid, ok := args["task_id"].(string); ok {
				taskID = tid
			}
			return leaderBgManager.Check(taskID)
		})
	return provider
}

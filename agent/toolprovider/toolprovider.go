package toolprovider

import (
	"encoding/json"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// ToolFunc 工具执行函数类型
type ToolFunc func(args map[string]interface{}) string

// ToolDefinition 工具定义结构
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  jsonschema.Definition
	Func        ToolFunc
}

// ToolProvider 动态工具提供者
type ToolProvider struct {
	registry map[string]ToolDefinition
}

// NewToolProvider 创建新的工具提供者实例
func NewToolProvider() *ToolProvider {
	return &ToolProvider{
		registry: make(map[string]ToolDefinition),
	}
}

// Register 注册工具
// name: 工具名称
// description: 工具描述
// parameters: JSON Schema 参数定义
// fn: 工具执行函数
func (tp *ToolProvider) Register(name, description string, parameters jsonschema.Definition, fn ToolFunc) {
	tp.registry[name] = ToolDefinition{
		Name:        name,
		Description: description,
		Parameters:  parameters,
		Func:        fn,
	}
}

// Unregister 注销工具
func (tp *ToolProvider) Unregister(name string) bool {
	if _, ok := tp.registry[name]; ok {
		delete(tp.registry, name)
		return true
	}
	return false
}

// Get 获取工具定义
func (tp *ToolProvider) Get(name string) (ToolDefinition, bool) {
	def, ok := tp.registry[name]
	return def, ok
}

// List 列出所有已注册的工具名
func (tp *ToolProvider) List() []string {
	names := make([]string, 0, len(tp.registry))
	for name := range tp.registry {
		names = append(names, name)
	}
	return names
}

// GetTools 转换为 OpenAI 工具格式
func (tp *ToolProvider) GetTools() []openai.Tool {
	tools := make([]openai.Tool, 0, len(tp.registry))
	for _, def := range tp.registry {
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			},
		})
	}
	return tools
}

// Count 返回已注册工具数量
func (tp *ToolProvider) Count() int {
	return len(tp.registry)
}

// HandleToolCallFromProvider 从提供者处理工具调用
func (tp *ToolProvider) HandleToolCall(call openai.ToolCall) string {
	def, ok := tp.Get(call.Function.Name)
	if !ok {
		return "Error: tool not found"
	}

	var args map[string]interface{}
	// 直接使用已经解析好的 Arguments
	if call.Function.Arguments != "" {
		// arguments 已经是字符串形式的 JSON
		// 需要先解析
		_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
	}

	return def.Func(args)
}

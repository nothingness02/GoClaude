package messagebus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Valid_Msg_Types 支持的消息类型
var Valid_Msg_Types = map[string]bool{
	"message":                true, // 普通消息（公开）
	"broadcast":              true, // 广播（公开）
	"shutdown_request":       true, // 关闭请求（发送给目标）
	"shutdown_response":      true, // 关闭响应
	"shutdown_notification":  true, // 关闭通知（广播给其他成员）
	"plan_approval_request":  true, // 计划审批请求（私有）
	"plan_approval_response": true, // 计划审批响应（私有）
}

// Msg 消息结构
type Msg struct {
	Type      string                 `json:"type"`
	From      string                 `json:"from"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Defs      map[string]interface{} `json:"extra,omitempty"`
}

// MessageBus 消息总线
type MessageBus struct {
	Dir string
	mu  sync.Mutex
}

// NewMessageBus 创建新的消息总线
func NewMessageBus(dir string) (*MessageBus, error) {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}
	return &MessageBus{
		Dir: dir,
	}, nil
}

// Send 发送消息
func (mb *MessageBus) Send(sender string, to string, content string, msgType string, extra map[string]interface{}) string {
	if _, ok := Valid_Msg_Types[msgType]; !ok {
		return fmt.Sprintf("Error: Invalid type %s. Valid: %s", msgType, "[message,broadcast,shutdown_request,shutdown_response,plan_approval_response]")
	}
	msg := Msg{
		Type:      msgType,
		From:      sender,
		Content:   content,
		Timestamp: time.Now(),
		Defs:      extra,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err.Error()
	}
	mb.mu.Lock()
	defer mb.mu.Unlock()
	inboxPath := filepath.Join(mb.Dir, fmt.Sprintf("%s.jsonl", to))
	f, err := os.OpenFile(inboxPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Sprintf("Error writing to inbox: %v", err)
	}
	defer f.Close()

	f.WriteString(string(data) + "\n")
	return fmt.Sprintf("Sent %s to %s", msgType, to)
}

// ReadInbox 读取收件箱
func (mb *MessageBus) ReadInbox(name string) []Msg {
	mb.mu.Lock()
	defer mb.mu.Unlock()
	inboxPath := filepath.Join(mb.Dir, fmt.Sprintf("%s.jsonl", name))
	content, err := os.ReadFile(inboxPath)
	if err != nil || len(content) == 0 {
		return []Msg{}
	}
	var messages []Msg
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	for _, line := range lines {
		if line != "" {
			var msg Msg
			if err := json.Unmarshal([]byte(line), &msg); err == nil {
				messages = append(messages, msg)
			}
		}
	}
	os.WriteFile(inboxPath, []byte(""), 0644)
	return messages
}

// Broadcast 广播消息
func (mb *MessageBus) Broadcast(sender, content string, teammates []string) string {
	count := 0
	for _, name := range teammates {
		if name != sender {
			mb.Send(sender, name, content, "broadcast", nil)
			count++
		}
	}
	return fmt.Sprintf("Broadcast to %d teammates", count)
}

// BroadcastShutdown 广播关闭通知给所有成员
func (mb *MessageBus) BroadcastShutdown(sender string, teammates []string) string {
	count := 0
	for _, name := range teammates {
		if name != sender {
			mb.Send(sender, name, fmt.Sprintf("%s has shut down", sender), "shutdown_notification", nil)
			count++
		}
	}
	return fmt.Sprintf("Broadcast shutdown notification to %d teammates", count)
}

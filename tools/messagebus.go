package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// todos:
// 1.将文件通信，修改成异步
// 2。 支持golang原生的channel 通信

var Valid_Msg_Types map[string]bool = map[string]bool{
	"message":                true,
	"broadcast":              true,
	"shutdown_request":       true,
	"shutdown_response":      true,
	"plan_approval_response": true,
	"plan_approval_request":  true,
}

type Msg struct {
	Type      string                 `json:"type"`
	From      string                 `json:"from"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Defs      map[string]interface{} `json:"extra,omitempty"`
}

type MessageBus struct {
	Dir string
	mu  sync.Mutex
}

func NewMessageBus(dir string) (*MessageBus, error) {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}
	return &MessageBus{
		Dir: dir,
	}, nil
}

func (mb *MessageBus) Send(sender string, to string, content string, msg_type string, extra map[string]interface{}) string {
	if _, ok := Valid_Msg_Types[msg_type]; !ok {
		return fmt.Sprintf("Error: Invalid type %s. Valid: %s", msg_type, "[message,broadcast,shutdown_request,shutdown_response,plan_approval_response]")
	}
	msg := Msg{
		Type:      msg_type,
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
	return fmt.Sprintf("Sent %s to %s", msg_type, to)
}

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

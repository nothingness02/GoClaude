package tools

import (
	"context"
	"crypto/rand"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/GoClaude/common"
)

// BgTask 记录后台任务的详细状态
type BgTask struct {
	Status  string
	Result  string
	Command string
}

// Notification 记录需要推送给 LLM 的任务完成通知
type Notification struct {
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Command string `json:"command"`
	Result  string `json:"result"`
}
type BackgroundManager struct {
	tasks map[string]*BgTask
	queue []Notification
	mu    sync.Mutex
}

func NewBackgroundManager() *BackgroundManager {
	return &BackgroundManager{
		tasks: make(map[string]*BgTask),
		queue: make([]Notification, 0),
	}
}
func (bg *BackgroundManager) Run(command string) string {
	taskID := shortID()
	bg.mu.Lock()
	bg.tasks[taskID] = &BgTask{
		Status:  "running",
		Command: command,
	}
	bg.mu.Unlock()
	go bg.execute(taskID, command)
	return fmt.Sprintf("Background task %s started: %s", taskID, truncate(command, 80))
}
func (bg *BackgroundManager) execute(taskID string, command string) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = common.WorkDir
	out, err := cmd.CombinedOutput()
	status := "completed"
	output := strings.TrimSpace(string(out))
	if ctx.Err() == context.DeadlineExceeded {
		status = "timeout"
		output = "Error: Timeout (300s)"
	} else if err != nil && output == "" {
		status = "error"
		output = fmt.Sprintf("Error: %v", err)
	}
	if len(output) > common.THRESHOLD {
		output = output[:common.THRESHOLD]
	}
	if output == "" {
		output = "(no output)"
	}
	bg.mu.Lock()
	defer bg.mu.Unlock()
	bg.tasks[taskID].Status = status
	bg.tasks[taskID].Result = output
	bg.queue = append(bg.queue, Notification{
		TaskID:  taskID,
		Status:  status,
		Command: truncate(command, 80),
		Result:  truncate(output, 500), // 给 LLM 的通知只需 500 字符的预览
	})
}

func (bg *BackgroundManager) Check(taskID string) string {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	if taskID != "" {
		t, exists := bg.tasks[taskID]
		if !exists {
			return fmt.Sprintf("Error: Unknown task %s", taskID)
		}
		res := t.Result
		if res == "" {
			res = "(running)"
		}
		return fmt.Sprintf("[%s] %s\n%s", t.Status, truncate(t.Command, 60), res)
	}
	if len(bg.tasks) == 0 {
		return "No background tasks."
	}
	var lines []string
	for tid, t := range bg.tasks {
		lines = append(lines, fmt.Sprintf("%s: [%s] %s", tid, t.Status, truncate(t.Command, 60)))
	}
	return strings.Join(lines, "\n")
}

func (bg *BackgroundManager) DrainNotifications() []Notification {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	if len(bg.queue) == 0 {
		return nil
	}
	notifs := make([]Notification, len(bg.queue))
	copy(notifs, bg.queue)
	bg.queue = nil
	return notifs
}

//------------功能函数-------------

// 生成 8 位的短 UUID (Hex)
func shortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// 阶段过长的输出
func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

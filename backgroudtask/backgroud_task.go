package backgroudtask

import (
	"context"
	"crypto/rand"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ============ BackgroundManager 后台任务管理 ============

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

// BackgroundManager 后台任务管理器
type BackgroundManager struct {
	WorkDir string
	Tasks   map[string]*BgTask
	Queue   []Notification
	mu      sync.Mutex
}

// NewBackgroundManager 创建后台任务管理器
func NewBackgroundManager(workDir string) *BackgroundManager {
	return &BackgroundManager{
		WorkDir: workDir,
		Tasks:   make(map[string]*BgTask),
		Queue:   make([]Notification, 0),
	}
}

// Run 启动后台任务
func (bg *BackgroundManager) Run(command string) string {
	taskID := ShortID()
	bg.mu.Lock()
	bg.Tasks[taskID] = &BgTask{
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
	cmd.Dir = bg.WorkDir
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
	if len(output) > 80000 {
		output = output[:80000]
	}
	if output == "" {
		output = "(no output)"
	}
	bg.mu.Lock()
	defer bg.mu.Unlock()
	bg.Tasks[taskID].Status = status
	bg.Tasks[taskID].Result = output
	bg.Queue = append(bg.Queue, Notification{
		TaskID:  taskID,
		Status:  status,
		Command: truncate(command, 80),
		Result:  truncate(output, 500),
	})
}

// Check 检查任务状态
func (bg *BackgroundManager) Check(taskID string) string {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	if taskID != "" {
		t, exists := bg.Tasks[taskID]
		if !exists {
			return fmt.Sprintf("Error: Unknown task %s", taskID)
		}
		res := t.Result
		if res == "" {
			res = "(running)"
		}
		return fmt.Sprintf("[%s] %s\n%s", t.Status, truncate(t.Command, 60), res)
	}
	if len(bg.Tasks) == 0 {
		return "No background tasks."
	}
	var lines []string
	for tid, t := range bg.Tasks {
		lines = append(lines, fmt.Sprintf("%s: [%s] %s", tid, t.Status, truncate(t.Command, 60)))
	}
	return strings.Join(lines, "\n")
}

// DrainNotifications 取出并清空通知队列
func (bg *BackgroundManager) DrainNotifications() []Notification {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	if len(bg.Queue) == 0 {
		return nil
	}
	notifs := make([]Notification, len(bg.Queue))
	copy(notifs, bg.Queue)
	bg.Queue = nil
	return notifs
}

// ShortID 生成 8 位短 ID
func ShortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// truncate 截断过长的输出
func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

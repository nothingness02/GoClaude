package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Task 定义任务的数据结构
type Task struct {
	ID          int    `json:"id"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Status      string `json:"status"`
	BlockedBy   []int  `json:"blockedBy"` //被谁阻塞
	Blocks      []int  `json:"blocks"`    //阻塞了谁
	Owner       string `json:"owner"`     // 属于那个agent(todos:  为后续的多agent系统做准备)
}

type TaskManager struct {
	Dir    string
	nextID int
	mu     sync.Mutex
}

// NewTaskManager 初始化并计算当前最大的 ID
func NewTaskManager(dir string) (*TaskManager, error) {
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	tm := &TaskManager{Dir: dir}
	tm.nextID = tm.maxID() + 1
	return tm, nil
}

func (tm *TaskManager) maxID() int {
	files, err := os.ReadDir(tm.Dir)
	if err != nil {
		return 0
	}
	max := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "task_") && strings.HasSuffix(f.Name(), ".json") {
			idStr := strings.TrimSuffix(strings.TrimPrefix(f.Name(), "task_"), ".json")
			if id, err := strconv.Atoi(idStr); err == nil && id > max {
				max = id
			}
		}
	}
	return max
}

func (tm *TaskManager) taskPath(id int) string {
	return filepath.Join(tm.Dir, fmt.Sprintf("task_%d.json", id))
}

func (tm *TaskManager) load(id int) (*Task, error) {
	data, err := os.ReadFile(tm.taskPath(id))
	if err != nil {
		return nil, fmt.Errorf("Task %d not found", id)
	}
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (tm *TaskManager) save(task *Task) error {
	data, err := json.MarshalIndent(task, "", "")
	if err != nil {
		return err
	}
	return os.WriteFile(tm.taskPath(task.ID), data, 0644)
}

func (tm *TaskManager) Create(subject, description string) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	task := &Task{
		ID:          tm.nextID,
		Subject:     subject,
		Description: description,
		Status:      "pending",
		BlockedBy:   []int{},
		Blocks:      []int{},
	}
	tm.save(task)
	tm.nextID++
	data, _ := json.MarshalIndent(task, "", "  ")
	return string(data)
}

// Get 获取单个任务
func (tm *TaskManager) Get(id int) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, err := tm.load(id)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	data, _ := json.MarshalIndent(task, "", "  ")
	return string(data)
}

// Update 更新任务状态和依赖
func (tm *TaskManager) Update(id int, status string, addBlockedBy, addBlocks []int) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, err := tm.load(id)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if status != "" {
		if status != "pending" && status != "in_progress" && status != "completed" {
			return fmt.Sprintf("Error: Invalid status: %s", status)
		}
		task.Status = status
		// Magic: 当任务完成时，去其他任务里解除对它的依赖阻塞
		if status == "completed" {
			tm.clearDependency(id)
		}
	}

	if len(addBlockedBy) > 0 {
		task.BlockedBy = uniqueAppend(task.BlockedBy, addBlockedBy...)
	}

	if len(addBlocks) > 0 {
		task.Blocks = uniqueAppend(task.Blocks, addBlocks...)
		// Bidirectional: 把当前任务 ID 塞进被阻塞任务的 blockedBy 列表
		for _, blockedID := range addBlocks {
			if blockedTask, err := tm.load(blockedID); err == nil {
				blockedTask.BlockedBy = uniqueAppend(blockedTask.BlockedBy, id)
				tm.save(blockedTask)
			}
		}
	}

	tm.save(task)
	data, _ := json.MarshalIndent(task, "", "  ")
	return string(data)
}

// ListAll 格式化输出所有任务
func (tm *TaskManager) ListAll() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	// todos:更换更加高效的读取策略
	var tasks []Task
	files, _ := os.ReadDir(tm.Dir)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "task_") {
			idStr := strings.TrimSuffix(strings.TrimPrefix(f.Name(), "task_"), ".json")
			id, _ := strconv.Atoi(idStr)
			if task, err := tm.load(id); err == nil {
				tasks = append(tasks, *task)
			}
		}
	}

	if len(tasks) == 0 {
		return "No tasks."
	}

	// 保证按 ID 排序
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	var lines []string
	for _, t := range tasks {
		marker := "[?]"
		switch t.Status {
		case "pending":
			marker = "[ ]"
		case "in_progress":
			marker = "[>]"
		case "completed":
			marker = "[x]"
		}

		blocked := ""
		if len(t.BlockedBy) > 0 {
			blocked = fmt.Sprintf(" (blocked by: %v)", t.BlockedBy)
		}
		lines = append(lines, fmt.Sprintf("%s #%d: %s%s", marker, t.ID, t.Subject, blocked))
	}

	return strings.Join(lines, "\n")
}

// 扫描可以做的任务
func (tm *TaskManager) ScanUnclaimed() []Task {
	tm.mu.Lock()
	defer tm.mu.TryLock()
	var unclaimed []Task
	files, _ := os.ReadDir(tm.Dir)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "task_") {
			idStr := strings.TrimSuffix(strings.TrimPrefix(f.Name(), "task_"), ".json")
			id, _ := strconv.Atoi(idStr)
			if task, err := tm.load(id); err == nil {
				if task.Status == "pendingh" && task.Owner == "" && len(task.BlockedBy) == 0 {
					unclaimed = append(unclaimed, *task)
				}
			}
		}
	}
	return unclaimed
}

// 分配任务
func (tm *TaskManager) Claim(taskID int, owner string) string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	task, err := tm.load(taskID)
	if err != nil {
		return fmt.Sprintf("Error: Task %d not found", taskID)
	}
	if task.Owner != "" {
		return fmt.Sprintf("Error: Task %d is already claimed by %s", taskID, task.Owner)
	}
	task.Owner = owner
	task.Status = "in_progress"
	tm.save(task)
	return fmt.Sprintf("Claimed task #%d for %s", taskID, owner)
}

func (tm *TaskManager) clearDependency(completedID int) {
	files, _ := os.ReadDir(tm.Dir)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "task_") {
			idStr := strings.TrimSuffix(strings.TrimPrefix(f.Name(), "task_"), ".json")
			id, _ := strconv.Atoi(idStr)
			if id == completedID {
				continue
			}

			if task, err := tm.load(id); err == nil {
				originalLen := len(task.BlockedBy)
				task.BlockedBy = removeElement(task.BlockedBy, completedID)
				if len(task.BlockedBy) < originalLen {
					tm.save(task) // 只有发生变动才保存
				}
			}
		}
	}
}

// 辅助函数
func removeElement(slice []int, elem int) []int {
	var res []int
	for _, v := range slice {
		if v != elem {
			res = append(res, v)
		}
	}
	return res
}
func uniqueAppend(slice []int, items ...int) []int {
	seen := make(map[int]bool)
	for _, v := range slice {
		seen[v] = true
	}
	var result []int
	for _, v := range append(slice, items...) {
		if !seen[v] {
			result = append(result, v)
			seen[v] = true
		}
	}
	return result
}

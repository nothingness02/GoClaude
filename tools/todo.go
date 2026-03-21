package tools

import (
	"fmt"
	"strings"
)

type TodoItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}
type TodoManager struct {
	items []TodoItem
}

func NewTodoManager() *TodoManager {
	return &TodoManager{
		items: []TodoItem{},
	}
}
func (todo *TodoManager) Update(items []TodoItem) (string, error) {
	if len(items) > 20 {
		return "", fmt.Errorf("Max 20 todos allowed")
	}
	var validated []TodoItem
	in_progress_count := 0
	for i, item := range items {
		text := strings.TrimSpace(item.Text)
		status := strings.ToLower(item.Status)
		if status == "" {
			status = "pending"
		}
		itemID := item.ID
		if itemID == "" {
			itemID = fmt.Sprintf("todo-%d", i+1)
		}
		if text == "" {
			return "", fmt.Errorf("Todo text cannot be empty")
		}
		if status != "pending" && status != "in_progress" && status != "done" {
			return "", fmt.Errorf("Invalid status: %s", item.Status)
		}
		if status == "in_progress" {
			in_progress_count++
		}
		validated = append(validated, TodoItem{
			ID:     itemID,
			Text:   text,
			Status: status,
		})
	}
	if in_progress_count > 1 {
		return "", fmt.Errorf("Only one task can be in_progress at a time")
	}
	todo.items = validated
	return "OK", nil
}

func (todo *TodoManager) Render() string {
	if len(todo.items) == 0 {
		return "No todos yet."
	}
	var lines []string
	done := 0
	for _, item := range todo.items {
		marker := ""
		switch item.Status {
		case "pending":
			marker = "[ ]"
		case "in_progress":
			marker = "[>]"
		case "completed":
			marker = "[x]"
			done++
		}
		lines = append(lines, fmt.Sprintf("%s #%s: %s", marker, item.ID, item.Text))
	}
	lines = append(lines, fmt.Sprintf("\n(%d/%d completed)", done, len(todo.items)))
	return strings.Join(lines, "\n")
}

package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func detect_repo_root(cwd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "git", "rev-pardse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "Error: Timeout (120s)"
	}
	root := strings.TrimSpace(string(out))
	if err != nil && root == "" {
		return fmt.Sprintf("Error: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return root
}

type EventBus struct {
	Dir string
}

func NewEventBus(dir string) *EventBus {
	return &EventBus{
		Dir: dir,
	}
}

func (es *EventBus) emit(event string,
	task map[string]interface{},
	worktree map[string]interface{},
	err string,
) {

}

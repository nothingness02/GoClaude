package linter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LinterManager struct {
	Dir        string
	ConfigPath string
	Rules      map[string][]string
	mu         sync.Mutex
}

func NewLinterManager(workDir string) *LinterManager {
	lm := &LinterManager{
		Dir:        workDir,
		ConfigPath: filepath.Join(workDir, ".agent_linters.json"),
		Rules:      make(map[string][]string),
	}
	lm.loadConfig()
	return lm
}

func (lm *LinterManager) loadConfig() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	data, err := os.ReadFile(lm.ConfigPath)
	if err == nil {
		json.Unmarshal(data, &lm.Rules)
	}
}

func (lm *LinterManager) saveConfig() {
	data, _ := json.MarshalIndent(lm.Rules, "", "  ")
	os.WriteFile(lm.ConfigPath, data, 0644)
}

// SetRule 动态添加或更新一条强约束规则
func (lm *LinterManager) RegisterRule(ext, command string) string {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if command == "" {
		delete(lm.Rules, ext)
		lm.saveConfig()
		return fmt.Sprintf("Removed linter rule for '%s'", ext)
	}

	lm.Rules[ext] = append(lm.Rules[ext], command)
	lm.saveConfig()
	return fmt.Sprintf("Set linter for '%s' to run: %s", ext, command)
}

// ValidateProject 扫描工作区，如果存在对应后缀的文件，则执行相应的 Linter
func (lm *LinterManager) ValidateProject() (bool, string) {
	lm.mu.Lock()
	// 复制一份规则，避免长时间阻塞
	rules := make(map[string][]string)
	for k, v := range lm.Rules {
		rules[k] = v
	}
	lm.mu.Unlock()

	if len(rules) == 0 {
		return true, "No linters configured. Skipping validation."
	}

	// 简单扫描工作区是否包含特定后缀的文件
	extFound := make(map[string]bool)
	filepath.Walk(lm.Dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			ext := filepath.Ext(path)
			if _, exists := rules[ext]; exists {
				extFound[ext] = true
			}
		}
		return nil
	})

	var errorLogs []string
	success := true

	// 执行对应的 Linter
	for ext := range extFound {
		cmds := rules[ext]

		for _, cmdStr := range cmds {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
			cmd.Dir = lm.Dir
			out, err := cmd.CombinedOutput()
			cancel()

			if err != nil {
				success = false
				errorLogs = append(errorLogs, fmt.Sprintf("❌ Linter failed for %s (Command: %s):\n%s", ext, cmdStr, string(out)))
			} else {
				errorLogs = append(errorLogs, fmt.Sprintf("✅ Linter passed for %s", ext))
			}
		}
	}

	return success, strings.Join(errorLogs, "\n\n")
}

func (lm *LinterManager) UnregisterRule(ext, command string) string {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if Rules, ok := lm.Rules[ext]; ok {
		newRules := make([]string, 0, len(Rules))
		for _, s := range Rules {
			if !strings.Contains(s, command) {
				newRules = append(newRules, s)
			}
		}
		lm.Rules[ext] = newRules
		return fmt.Sprintf("Removed linter '%s' for '%s'", command, ext)
	}
	return "not have this rule"
}

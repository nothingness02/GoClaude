package common

import (
	"os"

	"github.com/GoClaude/client"
	"github.com/GoClaude/config"
)

var (
	WorkDir        string
	SkillsDir      string
	ModelID        string
	TRANSCRIPT_DIR string
	TASKS_DIR      string
	Keep_RECENT    int
	THRESHOLD      int
	TEAM_DIR       string
	INBOX_DIR      string
)

// todo: 将common优化掉
// Client 导出 config.Client（使用接口类型）
var Client client.ClientInterface

func init() {
	workDir, _ := os.Getwd()
	_ = config.Init(workDir)

	cfg := config.GetConfig()
	WorkDir = cfg.WorkDir
	SkillsDir = cfg.SkillsDir
	ModelID = cfg.ModelID
	TRANSCRIPT_DIR = cfg.TranscriptDir
	TASKS_DIR = cfg.TasksDir
	Keep_RECENT = cfg.KeepRecent
	THRESHOLD = cfg.Threshold
	TEAM_DIR = cfg.TeamDir
	INBOX_DIR = cfg.InboxDir

	// 直接使用 config.Client
	Client = client.Client
}

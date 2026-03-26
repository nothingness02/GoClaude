package config

import "testing"

func TestConfig(t *testing.T) {
	Init("test")
	cfg := GetConfig()
	t.Log(cfg.TeamDir)
	t.Log(cfg.TasksDir)
	t.Log(cfg.TranscriptDir)
	t.Log(cfg.InboxDir)
	t.Log(cfg.KeepRecent)
	t.Log(cfg.Threshold)
	t.Log(cfg.SkillsDir)
}

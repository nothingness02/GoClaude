package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
)

var (
	WorkDir        string
	SkillsDir      string
	Client         *openai.Client
	ModelID        string
	TRANSCRIPT_DIR string
	TASKS_DIR      string
	Keep_RECENT    int
	THRESHOLD      int
)

func init() {
	_ = godotenv.Load("../.env") // 加载 .env 文件
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Warning: OPENAI_API_KEY is not set")
	}
	baseURL := os.Getenv("OPENAI_BASE_URL")
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	Client = openai.NewClientWithConfig(config)
	ModelID = os.Getenv("OPENAI_MODEL_ID")
	if ModelID == "" {
		ModelID = openai.GPT4o
	}
	WorkDir, _ = os.Getwd()
	SkillsDir = filepath.Join(WorkDir, "skills")
	TRANSCRIPT_DIR = filepath.Join(WorkDir, "transcripts")
	TASKS_DIR = filepath.Join(WorkDir, "tasks")
	Keep_RECENT = 3
	THRESHOLD = 80000
}

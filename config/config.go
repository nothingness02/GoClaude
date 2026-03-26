package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/GoClaude/client"
	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/viper"
)

// Config 全局配置结构
type Config struct {
	WorkDir       string ``
	SkillsDir     string
	ModelID       string
	TranscriptDir string
	TasksDir      string
	TeamDir       string
	InboxDir      string
	KeepRecent    int
	Threshold     int
	APIKey        string
	BaseURL       string
}

// 全局配置实例
var Cfg *Config

// Init 初始化配置
func Init(workDir string) error {
	// 加载 .env 文件
	_ = godotenv.Load(filepath.Join(workDir, ".env"))

	// 设置 viper 配置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(workDir)
	viper.AddConfigPath(filepath.Join(workDir, "config"))

	// 设置默认值
	viper.SetDefault("model", "qwen3-max")
	viper.SetDefault("keep_recent", 3)
	viper.SetDefault("threshold", 80000)
	viper.SetDefault("skills_dir", "skills")
	viper.SetDefault("transcript_dir", "transcripts")
	viper.SetDefault("tasks_dir", "tasks")
	viper.SetDefault("team_dir", "team")

	// 读取环境变量
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	modelID := os.Getenv("OPENAI_MODEL_ID")

	// 读取配置文件（如果存在）
	_ = viper.ReadInConfig()

	// 使用默认值
	if modelID == "" {
		modelID = viper.GetString("model")
	}
	if modelID == "" {
		modelID = openai.GPT4o
	}

	// 创建配置
	cfg := &Config{
		WorkDir:       workDir,
		SkillsDir:     filepath.Join(workDir, viper.GetString("skills_dir")),
		ModelID:       modelID,
		TranscriptDir: filepath.Join(workDir, viper.GetString("transcript_dir")),
		TasksDir:      filepath.Join(workDir, viper.GetString("tasks_dir")),
		TeamDir:       filepath.Join(workDir, viper.GetString("team_dir")),
		InboxDir:      filepath.Join(workDir, viper.GetString("team_dir"), "inbox"),
		KeepRecent:    viper.GetInt("keep_recent"),
		Threshold:     viper.GetInt("threshold"),
		APIKey:        apiKey,
		BaseURL:       baseURL,
	}

	// 验证 API key
	if apiKey == "" {
		fmt.Println("Warning: OPENAI_API_KEY is not set")
	}

	// 创建 OpenAI 客户端
	openaiCfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		openaiCfg.BaseURL = baseURL
	}
	openaiClient := openai.NewClientWithConfig(openaiCfg)

	// 包装为接口
	client.Client = &client.OpenAIClient{Client: openaiClient}
	Cfg = cfg

	return nil
}

// GetConfig 获取配置副本
func GetConfig() *Config {
	if Cfg == nil {
		panic("config not initialized, call config.Init first")
	}
	return Cfg
}

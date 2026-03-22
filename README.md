# GoClaude

GoClaude 是一个使用 Go 语言编写的 CLI 工具，通过 OpenAI API 实现与 Claude Code 类似的功能。

## 功能特性

- **交互式 CLI**: 简洁的命令行界面，支持实时对话
- **工具调用**: 支持自定义工具扩展 AI 能力
- **技能系统**: 可加载自定义技能模块
- **消息压缩**: 自动管理上下文长度，优化 token 使用
- **待办事项跟踪**: 智能提醒更新任务进度
- **团队协作**: 支持团队成员和收件箱功能
- **后台任务**: 支持异步后台任务处理
- **消息总线**: 组件间通信机制

## 环境配置

### 1. 创建 .env 文件

在项目根目录创建 `.env` 文件，配置以下环境变量：

```bash
# OpenAI API 密钥（必需）
OPENAI_API_KEY=your_api_key_here

# 可选：自定义 API 地址
OPENAI_BASE_URL=https://api.openai.com/v1

# 可选：指定模型，默认为 gpt-4o
OPENAI_MODEL_ID=gpt-4o
```

### 2. 运行程序

```bash
go run main.go
```

## 使用方法

启动后，进入交互式对话模式：

```
s03 >> 你好
[AI 回复]

s03 >> 帮我写一个 hello world 程序
[AI 回复并执行工具]

s03 >> q       # 退出程序
```

## 项目结构

```
GoClaude/
├── main.go              # 主程序入口
├── common/              # 通用配置和工具
├── compact/             # 消息压缩模块
├── prompt/              # 提示词模板
├── skills/              # 技能系统
├── tools/               # 工具函数
│   ├── tools.go         # 工具定义
│   ├── tasks.go         # 任务管理
│   ├── todo.go          # 待办事项
│   ├── background_task.go
│   ├── messagebus.go
│   ├── tracker.go
│   └── teammate_manager.go
├── transcripts/         # 对话记录
└── tasks/               # 任务数据
```

## 依赖

- Go 1.25+
- github.com/sashabaranov/go-openai
- github.com/joho/godotenv

## 许可证

MIT License

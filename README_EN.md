# GoClaude

GoClaude is a CLI tool written in Go that uses the OpenAI API to provide functionality similar to Claude Code.

## Features

- **Interactive CLI**: Clean command-line interface with real-time conversation
- **Tool Calling**: Extends AI capabilities with custom tools
- **Skills System**: Loadable custom skill modules
- **Message Compaction**: Automatic context length management and token optimization
- **Todo Tracking**: Smart reminders for updating task progress
- **Team Collaboration**: Team member and inbox functionality
- **Background Tasks**: Asynchronous background task processing
- **Message Bus**: Inter-component communication mechanism

## Setup

### 1. Create .env file

Create a `.env` file in the project root with the following environment variables:

```bash
# OpenAI API Key (required)
OPENAI_API_KEY=your_api_key_here

# Optional: Custom API endpoint
OPENAI_BASE_URL=https://api.openai.com/v1

# Optional: Specify model, defaults to gpt-4o
OPENAI_MODEL_ID=gpt-4o
```

### 2. Run the program

```bash
go run main.go
```

## Usage

After starting, enter interactive conversation mode:

```
s03 >> Hello
[AI response]

s03 >> Write me a hello world program
[AI responds and executes tools]

s03 >> q       # Quit the program
```

## Project Structure

```
GoClaude/
├── main.go              # Main entry point
├── common/              # Common configuration and utilities
├── compact/            # Message compaction module
├── prompt/              # Prompt templates
├── skills/              # Skills system
├── tools/               # Tool functions
│   ├── tools.go         # Tool definitions
│   ├── tasks.go         # Task management
│   ├── todo.go          # Todo tracking
│   ├── background_task.go
│   ├── messagebus.go
│   ├── tracker.go
│   └── teammate_manager.go
├── transcripts/         # Conversation logs
└── tasks/               # Task data
```

## Dependencies

- Go 1.25+
- github.com/sashabaranov/go-openai
- github.com/joho/godotenv

## License

MIT License

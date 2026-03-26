package client

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

// ClientInterface OpenAI 客户端接口
type ClientInterface interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
}

// OpenAIClient OpenAI 客户端包装
type OpenAIClient struct {
	Client *openai.Client
}

func (o *OpenAIClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return o.Client.CreateChatCompletion(ctx, req)
}

// 全局客户端
var Client ClientInterface

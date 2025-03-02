package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	openai "github.com/sashabaranov/go-openai"
)

var (
	client    *openai.Client
	systemMsg openai.ChatCompletionMessage
)

func initGPT() {
	client = openai.NewClient(os.Getenv("GPT_TOKEN"))

	systemMsg = openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: os.Getenv("GPT_SYSTEM_MSG"),
	}
	if os.Getenv("GPT_BUDDY") != "" {
		systemMsg.Content = strings.ReplaceAll(systemMsg.Content, "GPT_BUDDY", os.Getenv("GPT_BUDDY"))
	}
}

func getGPTAnswer(msg []*tgbotapi.Message) (string, error) {
	ctx := context.Background()

	messages := make([]openai.ChatCompletionMessage, len(msg)+1)

	messages[0] = systemMsg

	for i, m := range msg {
		messages[len(msg)-i] = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Name:    m.From.UserName,
			Content: m.Text,
		}
	}

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    os.Getenv("GPT_MODEL"),
			Messages: messages,
		})

	if len(messages) < 3 {
		log.Printf("Debug: messages %v", messages)
	}

	if err != nil {
		log.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no answer")
}

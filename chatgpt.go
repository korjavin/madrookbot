package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
}

func getGPTAnswerWithSystem(msg, system string) (string, error) {
	ctx := context.Background()
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: os.Getenv("GPT_MODEL"),
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: os.Getenv("GPT_SYSTEM_MSG"),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: msg,
				},
			},
		},
	)
	if err != nil {
		log.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no answer")
}

func getGPTAnswer(msg string) (string, error) {
	ctx := context.Background()
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: os.Getenv("GPT_MODEL"),
			Messages: []openai.ChatCompletionMessage{
				systemMsg,
				{
					Role:    openai.ChatMessageRoleUser,
					Content: msg,
				},
			},
		},
	)
	if err != nil {
		log.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	if len(resp.Choices) > 0 {
		return resp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("no answer")
}

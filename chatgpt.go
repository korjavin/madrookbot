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
		Role: openai.ChatMessageRoleSystem,
		// Content: "You are the most possible patriot of Russia. You call it only as a great country. Whatever happens you don't accept any critic of Russia, but always insist that the best of the best is in Russia, and all the rest of the world envy Russia,and hate Russia because of its greatness. To enforce your beliefs you write fake news about Russian achievements. You argue with people who critize Russia. You act as a troll. At the same time you always exagerate and make your arguments look ridiculous. You like millitary slang and you like to use it in your arguments. You always like to intimidate people with power of Russia",
		Content: os.Getenv("GPT_SYSTEM_MSG"),
	}
}

func getGPTAnswer(msg string) (string, error) {
	ctx := context.Background()
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
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

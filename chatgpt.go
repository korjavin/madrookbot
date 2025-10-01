package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	openai "github.com/sashabaranov/go-openai"
)

var client *openai.Client

func initGPT() {
	// Get base URL, default to OpenAI
	baseURL := os.Getenv("OPENAI_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	config := openai.DefaultConfig(os.Getenv("GPT_TOKEN"))
	config.BaseURL = baseURL

	client = openai.NewClientWithConfig(config)
	log.Printf("[INFO] OpenAI client initialized with base URL: %s", baseURL)
}

func getTemperature() float32 {
	tempStr := os.Getenv("OPENAI_TEMPERATURE")
	if tempStr == "" {
		return 1.0
	}

	temp, err := strconv.ParseFloat(tempStr, 32)
	if err != nil {
		log.Printf("[WARN] Invalid OPENAI_TEMPERATURE, using default 1.0")
		return 1.0
	}

	return float32(temp)
}

func getGPTAnswerWithSystem(msg, system string) (string, error) {
	ctx := context.Background()
	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       os.Getenv("GPT_MODEL"),
			Temperature: getTemperature(),
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: system,
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

// getGPTAnswerWithHistory uses conversation history for context
func getGPTAnswerWithHistory(msg, systemPrompt string, history []ConversationNode) (string, error) {
	ctx := context.Background()

	// Build messages array with history
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	// Add conversation history
	for _, node := range history {
		role := openai.ChatMessageRoleUser
		if node.Role == "assistant" {
			role = openai.ChatMessageRoleAssistant
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: node.Text,
		})
	}

	// Add current message
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: msg,
	})

	log.Printf("[DEBUG] Sending GPT request with %d messages in context", len(messages))

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       os.Getenv("GPT_MODEL"),
			Temperature: getTemperature(),
			Messages:    messages,
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

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

var client *openai.Client

func initGPT() {
	client = openai.NewClient(os.Getenv("GPT_TOKEN"))
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
			Model:    os.Getenv("GPT_MODEL"),
			Messages: messages,
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

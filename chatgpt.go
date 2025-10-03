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

// getSearchChatHistoryTool returns the search_chat_history tool definition
func getSearchChatHistoryTool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "search_chat_history",
			Description: "Searches the private Telegram group chat (that can be refenced just as 'chat') history to find relevant messages based on a user's query. Use this to answer questions about past conversations, find specific information, or recall what someone said or wrote. Use it when you have asks about specific user messages or opinions expressed in the chat.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "A detailed semantic search query. Should be a set of keywords the user's intent, e.g., 'Marketing budget, last quarter, results'",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func getGPTAnswerWithSystem(msg, system string) (string, error) {
	return getGPTAnswerWithSystemAndTools(msg, system, nil)
}

// getGPTAnswerWithSystemAndTools allows passing tools for specific use cases
func getGPTAnswerWithSystemAndTools(msg, system string, tools []openai.Tool) (string, error) {
	ctx := context.Background()

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: system,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: msg,
		},
	}

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       os.Getenv("GPT_MODEL"),
			Temperature: getTemperature(),
			ServiceTier: "flex",
			Messages:    messages,
			Tools:       tools,
		},
	)
	if err != nil {
		log.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no answer")
	}

	// Check if the model wants to call a tool
	if len(resp.Choices[0].Message.ToolCalls) > 0 && tools != nil {
		return handleToolCalls(ctx, messages, resp.Choices[0].Message, tools)
	}

	return resp.Choices[0].Message.Content, nil
}

func getEmbedding(text string) ([]float32, error) {
	ctx := context.Background()
	embeddingModel := os.Getenv("OPENAI_EMBEDDING_MODEL")
	if embeddingModel == "" {
		embeddingModel = "text-embedding-ada-002"
	}

	req := openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(embeddingModel),
	}

	resp, err := client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Data) > 0 {
		return resp.Data[0].Embedding, nil
	}

	return nil, fmt.Errorf("no embedding returned")
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

	tools := []openai.Tool{getSearchChatHistoryTool()}

	log.Printf("[DEBUG] Sending GPT request with %d messages in context and tool support", len(messages))

	resp, err := client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       os.Getenv("GPT_MODEL"),
			Temperature: getTemperature(),
			ServiceTier: "flex",
			Messages:    messages,
			Tools:       tools,
		},
	)
	if err != nil {
		log.Printf("ChatCompletion error: %v\n", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no answer")
	}

	// Check if the model wants to call a tool
	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		return handleToolCalls(ctx, messages, resp.Choices[0].Message, tools)
	}

	return resp.Choices[0].Message.Content, nil
}

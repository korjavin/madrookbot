package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	openai "github.com/sashabaranov/go-openai"
)

// handleToolCalls processes tool calls from the GPT response and returns the final answer
func handleToolCalls(ctx context.Context, messages []openai.ChatCompletionMessage, assistantMsg openai.ChatCompletionMessage, tools []openai.Tool) (string, error) {
	// Add the assistant's message with tool calls to the conversation
	messages = append(messages, assistantMsg)

	// Execute each tool call
	for _, toolCall := range assistantMsg.ToolCalls {
		log.Printf("[INFO] GPT requested tool call: %s with args: %s", toolCall.Function.Name, toolCall.Function.Arguments)

		var toolResult string
		var err error

		switch toolCall.Function.Name {
		case "search_chat_history":
			toolResult, err = executeSearchChatHistory(toolCall.Function.Arguments)
		default:
			err = fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
		}

		if err != nil {
			toolResult = fmt.Sprintf("Error: %v", err)
			log.Printf("[ERROR] Tool execution failed: %v", err)
		}

		// Add tool response to messages
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    toolResult,
			ToolCallID: toolCall.ID,
		})
	}

	// Send the conversation back to GPT with the tool results
	log.Printf("[DEBUG] Sending tool results back to GPT")
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
		return "", fmt.Errorf("error getting final response after tool call: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response after tool call")
	}

	return resp.Choices[0].Message.Content, nil
}

// executeSearchChatHistory calls the tool-api search endpoint
func executeSearchChatHistory(argsJSON string) (string, error) {
	// Parse arguments
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	log.Printf("[INFO] Searching chat history for: %s", args.Query)

	// Call the search API
	results, err := searchChatHistory(args.Query, 5) // Get top 5 results
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No relevant messages found in chat history.", nil
	}

	// Format results for GPT
	response := fmt.Sprintf("Found %d relevant messages:\n\n", len(results))
	for i, result := range results {
		response += fmt.Sprintf("%d. [Score: %.2f]\n", i+1, result.Score)
		response += fmt.Sprintf("   Text: %s\n", result.Text)
		response += fmt.Sprintf("   Link: %s\n", result.TelegramLink)
		response += fmt.Sprintf("   Time: %s\n\n", result.TimestampUTC)
	}

	return response, nil
}

// SearchResult represents a search result from tool-api
type SearchResult struct {
	Text         string  `json:"text"`
	TelegramLink string  `json:"telegram_link"`
	TimestampUTC string  `json:"timestamp_utc"`
	Score        float32 `json:"score"`
}

// SearchResponse is the response from tool-api
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

// searchChatHistory calls the tool-api search endpoint
func searchChatHistory(query string, topK int) ([]SearchResult, error) {
	toolAPIURL := os.Getenv("TOOL_API_URL")
	if toolAPIURL == "" {
		toolAPIURL = "http://tool-api:8081"
	}

	// Prepare request
	reqBody := map[string]interface{}{
		"query": query,
		"top_k": topK,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request
	resp, err := http.Post(toolAPIURL+"/search", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call tool-api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tool-api returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return searchResp.Results, nil
}

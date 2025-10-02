package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var geminiClient *genai.Client

// initGemini initializes the Gemini client for image generation
func initGemini() error {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY not set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return fmt.Errorf("failed to create Gemini client: %v", err)
	}

	geminiClient = client
	model := getGeminiImageModel()
	log.Printf("[INFO] Gemini client initialized with model: %s", model)
	return nil
}

// getGeminiImageModel returns the configured Gemini image model
func getGeminiImageModel() string {
	model := os.Getenv("GEMINI_IMAGE_MODEL")
	if model == "" {
		return "imagen-3.0-generate-001" // Default to Imagen 3
	}
	return model
}

// generateImage generates an image using Gemini's Imagen model
func generateImage(prompt string) (*bytes.Buffer, error) {
	if geminiClient == nil {
		return nil, fmt.Errorf("Gemini client not initialized")
	}

	ctx := context.Background()
	model := geminiClient.GenerativeModel(getGeminiImageModel())

	// Generate image
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("image generation failed: %v", err)
	}

	// Extract image data from response
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no image generated")
	}

	// The response should contain image data as Blob
	for _, part := range resp.Candidates[0].Content.Parts {
		if blob, ok := part.(genai.Blob); ok {
			// Blob contains the image data
			buffer := bytes.NewBuffer(blob.Data)
			return buffer, nil
		}
		// Try to extract from text if it's base64
		if text, ok := part.(genai.Text); ok {
			// Decode base64 if needed
			decoded, err := base64.StdEncoding.DecodeString(string(text))
			if err == nil && len(decoded) > 0 {
				buffer := bytes.NewBuffer(decoded)
				return buffer, nil
			}
		}
	}

	return nil, fmt.Errorf("could not extract image data from response")
}

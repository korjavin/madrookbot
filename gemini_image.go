package main

import (
	"bytes"
	"context"
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
		return "gemini-2.5-flash-image-preview" // Default to Gemini 2.5 Flash Image
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

	log.Printf("[DEBUG] Response has %d candidates, %d parts", len(resp.Candidates), len(resp.Candidates[0].Content.Parts))

	// Gemini 2.5 Flash Image returns both text description AND image blobs
	// We need to find and extract the image blob(s)
	var imageBuffer *bytes.Buffer
	var textDescription string

	for i, part := range resp.Candidates[0].Content.Parts {
		log.Printf("[DEBUG] Part %d type: %T", i, part)

		switch p := part.(type) {
		case genai.Blob:
			log.Printf("[DEBUG] Found Blob with %d bytes, MimeType: %s", len(p.Data), p.MIMEType)
			// Return the first image blob found
			if imageBuffer == nil {
				imageBuffer = bytes.NewBuffer(p.Data)
			}
		case genai.Text:
			textPart := string(p)
			if len(textPart) > 100 {
				log.Printf("[DEBUG] Found Text (first 100 chars): %s...", textPart[:100])
			} else {
				log.Printf("[DEBUG] Found Text: %s", textPart)
			}
			textDescription = textPart
		}
	}

	if imageBuffer != nil {
		log.Printf("[INFO] Successfully extracted image from response (with description: %s)", textDescription)
		return imageBuffer, nil
	}

	return nil, fmt.Errorf("no image blob found in response (only got: %s)", textDescription)
}

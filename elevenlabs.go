package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

var (
	elevenLabsAPIKey   string
	elevenLabsVoiceID  string
	elevenLabsModelID  string
	elevenLabsOnce     sync.Once
	elevenLabsInitErr  error
)

type voicesResponse struct {
	Voices []struct {
		VoiceID string `json:"voice_id"`
		Name    string `json:"name"`
	} `json:"voices"`
}

// initElevenLabs initializes the ElevenLabs client and caches the voice ID.
// This is called once to avoid repeated API calls for voice lookups.
func initElevenLabs() {
	elevenLabsOnce.Do(func() {
		elevenLabsAPIKey = os.Getenv("ELEVENLABS_API_KEY")
		if elevenLabsAPIKey == "" {
			elevenLabsInitErr = fmt.Errorf("ELEVENLABS_API_KEY is not set")
			log.Printf("[ERROR] %v", elevenLabsInitErr)
			return
		}

		elevenLabsModelID = os.Getenv("ELEVENLABS_MODEL_ID")
		if elevenLabsModelID == "" {
			elevenLabsInitErr = fmt.Errorf("ELEVENLABS_MODEL_ID is not set")
			log.Printf("[ERROR] %v", elevenLabsInitErr)
			return
		}

		// Check if voice ID is directly provided
		elevenLabsVoiceID = os.Getenv("ELEVENLABS_VOICE_ID")
		if elevenLabsVoiceID != "" {
			log.Printf("[INFO] ElevenLabs initialized with voice ID: %s", elevenLabsVoiceID)
			return
		}

		// Fallback to voice name lookup
		voiceName := os.Getenv("ELEVENLABS_VOICE_NAME")
		if voiceName == "" {
			elevenLabsInitErr = fmt.Errorf("either ELEVENLABS_VOICE_ID or ELEVENLABS_VOICE_NAME must be set")
			log.Printf("[ERROR] %v", elevenLabsInitErr)
			return
		}

		// Fetch available voices to find the voice ID
		req, err := http.NewRequest("GET", "https://api.elevenlabs.io/v1/voices", nil)
		if err != nil {
			elevenLabsInitErr = fmt.Errorf("could not create request: %v", err)
			log.Printf("[ERROR] %v", elevenLabsInitErr)
			return
		}
		req.Header.Set("xi-api-key", elevenLabsAPIKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			elevenLabsInitErr = fmt.Errorf("could not fetch voices: %v", err)
			log.Printf("[ERROR] %v", elevenLabsInitErr)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			elevenLabsInitErr = fmt.Errorf("API returned status %d", resp.StatusCode)
			log.Printf("[ERROR] %v", elevenLabsInitErr)
			return
		}

		var voicesResp voicesResponse
		if err := json.NewDecoder(resp.Body).Decode(&voicesResp); err != nil {
			elevenLabsInitErr = fmt.Errorf("could not decode voices response: %v", err)
			log.Printf("[ERROR] %v", elevenLabsInitErr)
			return
		}

		// Find the voice ID by name
		for _, v := range voicesResp.Voices {
			if v.Name == voiceName {
				elevenLabsVoiceID = v.VoiceID
				log.Printf("[INFO] ElevenLabs initialized with voice '%s' (ID: %s)", voiceName, elevenLabsVoiceID)
				return
			}
		}

		elevenLabsInitErr = fmt.Errorf("voice '%s' not found", voiceName)
		log.Printf("[ERROR] %v", elevenLabsInitErr)
	})
}

// makeSpeech generates speech from text using the ElevenLabs API.
// It returns an io.ReadCloser with the audio data.
func makeSpeech(text string) io.ReadCloser {
	// Initialize ElevenLabs if not already done
	initElevenLabs()

	if elevenLabsInitErr != nil {
		return nil
	}

	// Prepare request body
	requestBody := map[string]interface{}{
		"text":     text,
		"model_id": elevenLabsModelID,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("[ERROR] Could not marshal request: %v", err)
		return nil
	}

	// Create HTTP request for text-to-speech
	url := fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s", elevenLabsVoiceID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("[ERROR] Could not create request: %v", err)
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", elevenLabsAPIKey)

	// Make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[ERROR] Could not generate speech: %v", err)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Printf("[ERROR] API returned status %d: %s", resp.StatusCode, string(body))
		return nil
	}

	// Return the audio stream directly
	return resp.Body
}

package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"sync"

	"github.com/mutablelogic/go-client/pkg/elevenlabs"
)

var (
	elevenLabsClient *elevenlabs.Client
	elevenLabsVoiceID string
	elevenLabsModelID string
	elevenLabsOnce sync.Once
	elevenLabsInitErr error
)

// initElevenLabs initializes the ElevenLabs client and caches the voice ID.
// This is called once to avoid repeated API calls for voice lookups.
func initElevenLabs() {
	elevenLabsOnce.Do(func() {
		apiKey := os.Getenv("ELEVENLABS_API_KEY")
		if apiKey == "" {
			elevenLabsInitErr = log.Output(2, "[ERROR] ELEVENLABS_API_KEY is not set")
			return
		}

		voiceName := os.Getenv("ELEVENLABS_VOICE_NAME")
		if voiceName == "" {
			elevenLabsInitErr = log.Output(2, "[ERROR] ELEVENLABS_VOICE_NAME is not set")
			return
		}

		elevenLabsModelID = os.Getenv("ELEVENLABS_MODEL_ID")
		if elevenLabsModelID == "" {
			elevenLabsInitErr = log.Output(2, "[ERROR] ELEVENLABS_MODEL_ID is not set")
			return
		}

		// Create a new client
		client, err := elevenlabs.New(apiKey)
		if err != nil {
			log.Printf("[ERROR] Could not create elevenlabs client: %v", err)
			elevenLabsInitErr = err
			return
		}
		elevenLabsClient = client

		// Find the voice ID from the voice name (only done once at startup)
		voices, err := client.Voices()
		if err != nil {
			log.Printf("[ERROR] Could not get voices from elevenlabs: %v", err)
			elevenLabsInitErr = err
			return
		}

		for _, v := range voices {
			if v.Name == voiceName {
				elevenLabsVoiceID = v.Id
				log.Printf("[INFO] ElevenLabs initialized with voice '%s' (ID: %s)", voiceName, elevenLabsVoiceID)
				return
			}
		}

		log.Printf("[ERROR] Voice '%s' not found", voiceName)
		elevenLabsInitErr = log.Output(2, "[ERROR] Voice not found")
	})
}

// makeSpeech generates speech from text using the ElevenLabs API.
// It returns an io.ReadCloser with the audio data.
func makeSpeech(text string) io.ReadCloser {
	// Initialize ElevenLabs client if not already done
	initElevenLabs()

	if elevenLabsInitErr != nil {
		return nil
	}

	// Create a buffer to store the audio
	var buf bytes.Buffer

	// Generate the speech
	_, err := elevenLabsClient.TextToSpeech(&buf, elevenLabsVoiceID, text, elevenlabs.OptModel(elevenLabsModelID))
	if err != nil {
		log.Printf("[ERROR] Could not generate speech: %v", err)
		return nil
	}

	return io.NopCloser(&buf)
}

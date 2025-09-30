package main

import (
	"bytes"
	"io"
	"log"
	"os"

	"github.com/mutablelogic/go-client/pkg/elevenlabs"
)

// makeSpeech generates speech from text using the ElevenLabs API.
// It returns an io.ReadCloser with the audio data.
func makeSpeech(text string) io.ReadCloser {
	apiKey := os.Getenv("ELEVENLABS_API_KEY")
	if apiKey == "" {
		log.Println("[ERROR] ELEVENLABS_API_KEY is not set")
		return nil
	}
	voiceName := os.Getenv("ELEVENLABS_VOICE_NAME")
	if voiceName == "" {
		log.Println("[ERROR] ELEVENLABS_VOICE_NAME is not set")
		return nil
	}
	modelId := os.Getenv("ELEVENLABS_MODEL_ID")
	if modelId == "" {
		log.Println("[ERROR] ELEVENLABS_MODEL_ID is not set")
		return nil
	}

	// Create a new client
	client, err := elevenlabs.New(apiKey)
	if err != nil {
		log.Printf("[ERROR] Could not create elevenlabs client: %v", err)
		return nil
	}

	// Find the voice ID from the voice name
	voices, err := client.Voices()
	if err != nil {
		log.Printf("[ERROR] Could not get voices from elevenlabs: %v", err)
		return nil
	}

	var voiceId string
	for _, v := range voices {
		if v.Name == voiceName {
			voiceId = v.Id
			break
		}
	}

	if voiceId == "" {
		log.Printf("[ERROR] Voice '%s' not found", voiceName)
		return nil
	}

	// Create a buffer to store the audio
	var buf bytes.Buffer

	// Generate the speech
	_, err = client.TextToSpeech(&buf, voiceId, text, elevenlabs.OptModel(modelId))
	if err != nil {
		log.Printf("[ERROR] Could not generate speech: %v", err)
		return nil
	}

	return io.NopCloser(&buf)
}
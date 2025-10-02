package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {
	// Initialize media suggestions database
	err := initMediaSuggestions()
	if err != nil {
		log.Printf("Error initializing media suggestions: %v", err)
	}

	// Initialize conversation cache
	initConversationCache()

	// Initialize activity tracking database
	err = initActivityDatabase()
	if err != nil {
		log.Printf("Error initializing activity database: %v", err)
	}

	if os.Getenv("GPT_TOKEN") != "" {
		model := os.Getenv("GPT_MODEL")
		if model == "" {
			model = "not specified"
		}
		log.Printf("GPT_TOKEN is set, using model: %s", model)
		initGPT()
	}

	// Initialize Gemini for image generation if API key is set
	if os.Getenv("GEMINI_API_KEY") != "" {
		err = initGemini()
		if err != nil {
			log.Printf("Error initializing Gemini: %v", err)
		}
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("[INFO] Shutdown signal received")
		SaveConversationsOnShutdown()
		if db != nil {
			db.Close()
		}
		os.Exit(0)
	}()

	// Start scheduled tasks
	go scheduleMediaTasks()
	go scheduleActivityCleanup()

	botGo()
}

// getClassGroupID returns the Telegram group ID where the class is held
func getClassGroupID() (int64, error) {
	groupIDStr := os.Getenv("CLASS_GROUP_ID")
	if groupIDStr == "" {
		return 0, fmt.Errorf("CLASS_GROUP_ID environment variable not set")
	}

	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CLASS_GROUP_ID: %v", err)
	}

	return groupID, nil
}

// getOwnerID returns the Telegram owner ID from the OWNER_ID environment variable
func getOwnerID() (int, error) {
	ownerIDStr := os.Getenv("OWNER_ID")
	if ownerIDStr == "" {
		// Fallback to default ID if not set
		return 0, errors.New("owner is not set")
	}

	ownerID, err := strconv.Atoi(ownerIDStr)
	if err != nil {
		log.Printf("Invalid OWNER_ID environment variable: %v", err)
		return 0, err
	}

	return ownerID, nil
}

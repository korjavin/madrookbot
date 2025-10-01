package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
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

	if os.Getenv("GPT_TOKEN") != "" {
		log.Printf("GPT_TOKEN is set, using GPT-3")
		initGPT()
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

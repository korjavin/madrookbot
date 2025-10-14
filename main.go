package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Initialize conversation cache
	initConversationCache()

	// Initialize activity tracking database
	err := initActivityDatabase()
	if err != nil {
		log.Printf("Error initializing activity database: %v", err)
	}

	// Initialize classes table
	err = initClassesTable()
	if err != nil {
		log.Printf("Error initializing classes table: %v", err)
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

	// Initialize Qdrant client if memory feature is enabled
	if os.Getenv("MEMORY_GROUP_ID") != "" {
		log.Println("[INFO] MEMORY_GROUP_ID is set, initializing Qdrant client...")
		err = initQdrantClient()
		if err != nil {
			log.Fatalf("Error initializing Qdrant client: %v", err)
		}
	}

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("[INFO] Shutdown signal received")
		stopClassScheduler()
		stopNewsScheduler()
		SaveConversationsOnShutdown()
		if db != nil {
			db.Close()
		}
		os.Exit(0)
	}()

	// Start scheduled tasks
	go scheduleActivityCleanup()

	botGo()
}


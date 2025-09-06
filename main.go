package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

var (
	//	defaultVoice = "Raveena" <- hardcoded in makeSpeech
	voices = map[string]bool{
		"Russell":  true,
		"Nicole":   true,
		"Joanna":   true,
		"Salli":    true,
		"Kimberly": true,
		"Kendra":   true,
		"Justin":   true,
		"Joey":     true,
		"Ivy":      true,
		"Emma":     true,
		"Brian":    true,
		"Amy":      true,
		"Raveena":  true,
		"Geraint":  true,
	}

	prefs map[int]string
)

func main() {
	var err error
	prefs, err = GetAllVoices()
	if err != nil {
		log.Printf("Error getting all voices: %v", err)
		prefs = make(map[int]string)
	}

	// Initialize media suggestions database
	err = initMediaSuggestions()
	if err != nil {
		log.Printf("Error initializing media suggestions: %v", err)
	}


	var ff filterFunc
	if os.Getenv("GPT_TOKEN") != "" {
		log.Printf("GPT_TOKEN is set, using GPT-3")
		initGPT()
		keywords := strings.Split(os.Getenv("GPT_KEYWORDS"), ",")
		ff = generatorOfContainFuncs(keywords)
	}

	// Start scheduled tasks
	go scheduleMediaTasks()

	botGo(ff)
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

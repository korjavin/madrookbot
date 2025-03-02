package main

import (
	"log"
	"os"
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
	
	// print all env
	for _, e := range os.Environ() {
		log.Println(e)
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

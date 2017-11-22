package main

import (
	"log"
)

var (
	defaultVoice string
	voices       map[string]bool
	prefs        map[int]string
)

func init() {
	defaultVoice = "Raveena"
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

}

func main() {
	prefs = make(map[int]string)
	err := loadprefs()
	if err != nil {
		log.Printf("Prefs error: %v", err)
	}
	botGo()
}

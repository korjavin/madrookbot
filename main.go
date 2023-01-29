package main

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

	prefs = map[int]string{}
)

func main() {
	botGo()
}

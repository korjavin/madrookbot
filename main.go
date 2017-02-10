package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
)

var (
	defaultVoice string
	voices       map[string]bool
	prefs        map[int]string
)

func init() {
	defaultVoice = os.Getenv("BOT_VOICE")
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
	loadprefs()
	bot_go()
}
func makeAudio(id int64, text string, userVoice string, uid int) error {
	voice := defaultVoice
	if val, ok := prefs[uid]; ok {
		voice = val
	}
	if userVoice != "" {
		if voices[userVoice] {
			voice = userVoice
		}
	}
	text = "<speak>" + text + "</speak>"
	fileext := fmt.Sprintf("file_%06d.mp3", id)

	args := "aws polly synthesize-speech --text-type ssml --text " + strconv.Quote(text) + " --output-format mp3 --voice-id " + voice + " " + fileext
	log.Println(args)

	lsCmd := exec.Command("sh", "-c", args)
	_, err := lsCmd.Output()
	if err != nil {
		return err
	}
	return nil
}

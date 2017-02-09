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
)

func init() {
	defaultVoice = os.Getenv("BOT_VOICE")
}

func main() {
	bot_go()
}
func makeAudio(id int64, text string, userVoice string) error {
	voice := defaultVoice
	if userVoice != "" {
		voice = userVoice
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

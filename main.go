package main

import (
	"io/ioutil"
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
func makeAudio(text string, userVoice string, uid int) (string, error) {
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
	tmpfile, err := ioutil.TempFile("./", "voice")
	if err != nil {
		log.Printf("File error: %v", err)
	} else {
		defer func() {
			err := os.Remove(tmpfile.Name())
			if err != nil {
				log.Printf("File remove error: %v", err)
			}
		}()
	}
	fileext := tmpfile.Name() + ".mp3"

	args := "aws polly synthesize-speech --text-type ssml --text " + strconv.Quote(text) + " --output-format mp3 --voice-id " + voice + " " + fileext
	log.Println(args)

	lsCmd := exec.Command("sh", "-c", args)
	_, err = lsCmd.Output()
	if err != nil {
		return "", err
	}
	return fileext, nil
}

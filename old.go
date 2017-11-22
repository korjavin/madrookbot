package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func sendAudio(text string, voice string, uid int, cid int64, mid int) {
	re := regexp.MustCompile("\\n")
	text = re.ReplaceAllString(text, " ")
	fileext, err := makeAudio(text, voice, uid)
	if err != nil {
		log.Printf("Make: %v ", err)
	}

	msg := tgbotapi.NewVoiceUpload(cid, fileext)
	msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true, Selective: true}
	msg.Caption = "Voice"
	if mid != 0 {
		msg.ReplyToMessageID = mid
	}
	_, err = bot.Send(msg)

	if err != nil {
		log.Printf("Send: %v ", err)
	}
	err = os.Remove(fileext)
	if err != nil {
		log.Printf("Can't remove file: %v ", err)
	}
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
func getFile(url string) (string, error) {
	tmpfile, err := ioutil.TempFile("./", "voice")
	if err != nil {
		log.Printf("File error: %v", err)
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", err

	}
	io.Copy(tmpfile, resp.Body)
	return tmpfile.Name(), nil

}

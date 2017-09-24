package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"os"
	"regexp"
	"strings"
)

var (
	bot  *tgbotapi.BotAPI
	name string
)

func botGo() {
	var err error
	bot, err = tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	me, err := bot.GetMe()
	if err != nil {
		log.Panicf("me: %#v \n", err)
	}
	name = me.UserName

	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	fsms := make(map[int]*dialogue)

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Printf("[ERROR] getting update channel %v\n", err)
	}

	for update := range updates {
		// if update.EditedMessage.
		if update.Message == nil && update.EditedMessage == nil {
			continue
		}
		var text string
		var from int
		var messg *tgbotapi.Message

		if update.Message != nil {
			messg = update.Message
		}
		if update.EditedMessage != nil {
			messg = update.EditedMessage
		}
		text = messg.Text
		from = messg.From.ID
		if _, ok := fsms[from]; !ok {
			fsms[from] = newDialogue(from)
		}

		fsm := fsms[from]

		if strings.HasPrefix(strings.ToUpper(text), "/CANCEL") {
			fsm.state.Event("cancel")

			answer := "Command canceled."

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID

			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}

		switch fsm.state.Current() {
		case "waitaudio":
			if messg.Voice == nil {
				answer := "Sorry, I don't hear you. Send me a voice or /cancel command"
				msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
				_, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
			} else {
				vmess := messg.Voice
				// var vfile tgbotapi.FileConfig
				// vfile.FileID = vmess.FileID
				// file, err := bot.GetFile(vfile)
				// if err != nil {
				// 	log.Printf("Send: %v ", err)
				// }
				// log.Printf("File: %+v ", file)
				url, err := bot.GetFileDirectURL(vmess.FileID)
				filename, err := getFile(url)
				uploadFile("")

				answer := "I got file , your link is " + filename
				msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
				_, err = bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
			}
		case "waitvoice":
			answer := ""
			if !voices[text] {
				answer = "Sorry,I don't have this voice: '" + text + "', command canceled"
				fsm.state.Event("cancel")
				msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
				msg.ReplyToMessageID = messg.MessageID

				msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true, Selective: true}

				_, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
			} else {
				answer = "Okay, Dear " + messg.From.FirstName + ", I will use the voice " + text + " for your messages! You can still override voice by using square brackets."
				sendAudio(answer, text, messg.From.ID, messg.Chat.ID, messg.MessageID)
				prefs[messg.From.ID] = text
				err := saveprefs(messg.From.ID, text)
				if err != nil {
					log.Printf("Save prefs: %v ", err)
				}
				fsm.state.Event("setvoice")
			}
			continue
		case "waitidiom":
			split := strings.Split(text, " ")
			answer := getIdiom(strings.Join(split, "+"))
			if answer == "" {
				answer = "Sorry, nothing about " + text
			} else {
				go sendEvent("translation", "define", text)
			}
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			fsm.state.Event("setterm")
			continue
		case "waitterm":
			answer := getDefinition(text)
			if answer == "" {
				answer = "Sorry, nothing about " + text
			} else {
				go sendEvent("translation", "define", text)
			}
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			fsm.state.Event("setterm")
			continue
		case "waitoxford":
			answer := getOxfordDefinition(text)
			if answer == "" {
				answer = "Sorry, nothing about " + text + "\n You can edit your message or send new \n Or /cancel"
			} else {
				go sendEvent("translation", "oxford", text)
				fsm.state.Event("setterm")
			}
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		default:
		}

		if strings.HasPrefix(strings.ToUpper(text), "/HELP") {
			answer := "You can send me any text to read aloud, but please mention me by @" + name
			answer += "\nIf you want me to change my voice send me voice-name in square brackets like [Joey] "
			answer += "\n   /setvoice command for setting default voice (just for you)"
			answer += "\n   /define term :  show the definition from Merriam-Webster dictionary"
			answer += "\n   /oxford term :  show the definition from Oxford dictionary"
			answer += "\n   /idiom term  :  show the definition from idioms.thefreedictionary.com"
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			go sendEvent("command", "help", "")
			continue
		}
		if strings.HasPrefix(strings.ToUpper(text), "/DEFINE") {

			split := strings.Split(text, " ")
			answer := ""
			if len(split) < 2 {
				answer = " Please, use send me any text to search in Merriam-Webster dictionary"
				fsm.state.Event("waitterm")
			} else {
				answer = getDefinition(split[1])
				if answer == "" {
					answer = "Sorry, nothing about " + split[1]
				} else {
					go sendEvent("translation", "define", split[1])
				}
			}

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(text), "/IDIOM") {
			split := strings.Split(text, " ")
			answer := ""
			if len(split) < 2 {
				answer = " Please send me any text to search in idioms.thefreedictionary.com"
				fsm.state.Event("waitidiom")
			} else {
				answer = getIdiom(strings.Join(split[1:], "+"))
				if answer == "" {
					answer = "Sorry, nothing about " + strings.Join(split[1:], " ")
				} else {
					go sendEvent("translation", "define", split[1])
				}
			}

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(text), "/OXFORD") {
			split := strings.Split(text, " ")
			answer := ""
			if len(split) < 2 {
				answer = "Please send me any text to search in the oxford dictionary"
				fsm.state.Event("waitoxford")
			} else {
				answer = getOxfordDefinition(split[1])
				if answer == "" {
					answer = "Sorry, nothing about " + split[1]
				} else {
					go sendEvent("translation", "oxford", split[1])
				}
			}

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(text), "/SETVOICE") {
			fsm.state.Event("waitvoice")

			var buttons []tgbotapi.KeyboardButton

			for k := range voices {
				buttons = append(buttons, tgbotapi.KeyboardButton{Text: k})
			}
			answer := "Please choose a voice from the list and send to me."
			markup := tgbotapi.ReplyKeyboardMarkup{
				Keyboard: [][]tgbotapi.KeyboardButton{
					buttons,
				},
				OneTimeKeyboard: true,
				ResizeKeyboard:  true,
				Selective:       true,
			}

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			msg.ReplyMarkup = markup
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(text), "/AUDIO") {
			fsm.state.Event("audio")

			answer := "Please send me an audio to upload on audioboom."

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID

			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if !strings.Contains(strings.ToUpper(text), strings.ToUpper(name)) {
			continue
		}

		log.Printf("[%s] %s \n", messg.From.UserName, text)

		text = regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(text, "")

		revoice := regexp.MustCompile(`\[(\w+)\]`)
		voice := revoice.FindString(text)
		voice = regexp.MustCompile(`\[|\]`).ReplaceAllLiteralString(voice, "")
		text = revoice.ReplaceAllLiteralString(text, "")

		sendAudio(text, voice, messg.From.ID, messg.Chat.ID, messg.MessageID)

		go sendEvent("voice", "generate", messg.From.UserName)
	}
}
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

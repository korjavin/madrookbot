package main

import (
	"gopkg.in/telegram-bot-api.v4"
	"log"
	"os"
	"regexp"
	"strings"
)

var (
	bot  *tgbotapi.BotAPI
	name string
)

func init() {
	name = os.Getenv("BOT_NAME")
}

func botGo() {
	var err error
	bot, err = tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	replies := make(map[int]bool)

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil && update.CallbackQuery == nil {
			continue
		}
		if update.CallbackQuery != nil {
			continue
		}
		if update.Message.ReplyToMessage != nil {
			_, ok := replies[update.Message.ReplyToMessage.MessageID]
			if ok {
				delete(replies, update.Message.ReplyToMessage.MessageID)
				text := update.Message.Text
				answer := ""
				if !voices[text] {
					answer = "Sorry,I don't have this voice: " + text + "\n Choose another /setvoice"
				} else {
					answer = "Okay I will use the voice " + text + " for your messages! You can still override voice by using square brackets."
					sendAudio(answer, text, update.Message.From.ID, update.Message.Chat.ID, update.Message.MessageID)
					prefs[update.Message.From.ID] = text
					err := saveprefs(update.Message.From.ID, text)
					if err != nil {
						log.Printf("Save prefs: %v ", err)
					}
					continue
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, answer)
				msg.ReplyToMessageID = update.Message.MessageID

				msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true, Selective: true}

				_, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
				continue
			}

			continue
		}
		if strings.HasPrefix(strings.ToUpper(update.Message.Text), "/HELP") {
			answer := " You can send me any text to read aloud, but please mention me by @" + name
			answer += "\n If you want me to change my voice send me voice-name in square brackets like [Joey] "
			answer += "\n /setvoice command for setting default voice (just for you)"
			answer += "\n /define term :  show the definition from Merriam-Webster dictionary"
			answer += "\n /oxford term :  show the definition from oxford dictionary"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, answer)
			msg.ReplyToMessageID = update.Message.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			go sendEvent("command", "help", "")
			continue
		}
		if strings.HasPrefix(strings.ToUpper(update.Message.Text), "/DEFINE") {

			split := strings.Split(update.Message.Text, " ")
			answer := ""
			if len(split) < 2 {
				answer = " Please, use /define term "
			} else {
				answer = getDefinition(split[1])
				if answer == "" {
					answer = "Sorry, nothing about " + split[1]
				} else {
					go sendEvent("translation", "define", split[1])
				}
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, answer)
			msg.ReplyToMessageID = update.Message.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(update.Message.Text), "/OXFORD") {

			split := strings.Split(update.Message.Text, " ")
			answer := ""
			if len(split) < 2 {
				answer = " Please, use /oxford term "
			} else {
				answer = getOxfordDefinition(split[1])
				if answer == "" {
					answer = "Sorry, nothing about " + split[1]
				} else {
					go sendEvent("translation", "oxford", split[1])
				}
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, answer)
			msg.ReplyToMessageID = update.Message.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(update.Message.Text), "/SETVOICE") {
			split := strings.Split(update.Message.Text, " ")
			answer := ""
			if len(split) < 2 {
				var buttons []tgbotapi.KeyboardButton

				for k := range voices {
					buttons = append(buttons, tgbotapi.KeyboardButton{Text: k})
				}
				answer = "Please choose a voice from the list."
				markup := tgbotapi.ReplyKeyboardMarkup{
					Keyboard: [][]tgbotapi.KeyboardButton{
						buttons,
					},
					OneTimeKeyboard: true,
					ResizeKeyboard:  true,
					Selective:       true,
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, answer)
				msg.ReplyToMessageID = update.Message.MessageID
				msg.ReplyMarkup = markup
				mc, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
				replies[mc.MessageID] = true
				// log.Printf("Send: %#v ", mc)
				continue

			} else if !voices[split[1]] {
				answer = "I don't have this voice: " + split[1] + "\n please chose another one from the /list"
			} else {
				answer = "Okay I will use the voice " + split[1] + " for your messages! You can temporarily override this by using square brackets."
				sendAudio(answer, split[1], update.Message.From.ID, update.Message.Chat.ID, update.Message.MessageID)
				prefs[update.Message.From.ID] = split[1]
				err := saveprefs(update.Message.From.ID, split[1])
				if err != nil {
					log.Printf("Saveprefs: %v ", err)
				}
				go sendEvent("voice", "set", split[1])
				continue
			}

			msg := tgbotapi.NewMessage(update.Message.Chat.ID, answer)
			msg.ReplyToMessageID = update.Message.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if !strings.Contains(strings.ToUpper(update.Message.Text), strings.ToUpper(name)) {
			continue
		}

		log.Printf("[%s] %s \n", update.Message.From.UserName, update.Message.Text)

		text := regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(update.Message.Text, "")

		revoice := regexp.MustCompile(`\[(\w+)\]`)
		voice := revoice.FindString(text)
		voice = regexp.MustCompile(`\[|\]`).ReplaceAllLiteralString(voice, "")
		text = revoice.ReplaceAllLiteralString(text, "")

		sendAudio(text, voice, update.Message.From.ID, update.Message.Chat.ID, update.Message.MessageID)

		go sendEvent("voice", "generate", update.Message.From.UserName)
	}
}
func sendAudio(text string, voice string, uid int, cid int64, mid int) {
	fileext, err := makeAudio(text, voice, uid)
	if err != nil {
		log.Printf("Make: %v ", err)
	}

	msg := tgbotapi.NewVoiceUpload(cid, fileext)
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

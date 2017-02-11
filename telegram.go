package main

import (
	"fmt"
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

func bot_go() {
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
					answer = "Okay I will use the voice " + text + " for your messages! \nYou can still overlap voice by using square brackets like [Kendra]"
					prefs[update.Message.From.ID] = text
					saveprefs(update.Message.From.ID, text)
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, answer)
				msg.ReplyToMessageID = update.Message.MessageID

				msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{true, true}

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
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, answer)
			msg.ReplyToMessageID = update.Message.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
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

				for k, _ := range voices {
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
				answer = "Okay I will use the voice " + split[1] + " for your messages! \n You can temporarily overlap this by using  square brackets like [Kendra]"
				prefs[update.Message.From.ID] = split[1]
				saveprefs(update.Message.From.ID, split[1])
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

		err = makeAudio(update.Message.Chat.ID, text, voice, update.Message.From.ID)
		if err != nil {
			log.Printf("Make: %v ", err)
		}

		fileext := fmt.Sprintf("file_%06d.mp3", update.Message.Chat.ID)

		msg := tgbotapi.NewVoiceUpload(update.Message.Chat.ID, fileext)
		// msg.Title = "Voice"
		// msg.Performer = "MadRookBot"
		// msg.MimeType = "audio/mpeg"
		msg.Caption = "Voice"
		msg.ReplyToMessageID = update.Message.MessageID
		_, err := bot.Send(msg)

		if err != nil {
			log.Printf("Send: %v ", err)
		}
	}
}

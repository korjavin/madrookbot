package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/olekukonko/tablewriter"
)

var removeVoice = regexp.MustCompile(`\[(\w+)\]`)

func botGo() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	me, err := bot.GetMe()
	if err != nil {
		log.Panicf("me: %#v \n", err)
	}
	name := me.UserName

	bot.Debug = false
	sg := make(map[int]*SheetGenerator)

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	fsms := make(map[int]*dialogue)

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Printf("[ERROR] getting update channel %v\n", err)
	}

	for update := range updates {
		if update.Message == nil && update.EditedMessage == nil && update.CallbackQuery == nil {
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

		log.Printf("[INFO] state for %s is  %s  \n", messg.From.UserName, fsm.state.Current())

		if strings.HasPrefix(strings.ToUpper(text), "/CANCEL") {
			_ = fsm.state.Event("cancel")

			msg := tgbotapi.NewMessage(messg.Chat.ID, "Command canceled.")
			msg.ReplyToMessageID = messg.MessageID

			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}

		switch fsm.state.Current() {
		case "waitvoice":
			answer := ""
			if !voices[text] {
				answer = "Sorry,I don't have this voice: '" + text + "', command canceled"
				_ = fsm.state.Event("cancel")
				msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
				msg.ReplyToMessageID = messg.MessageID

				msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true, Selective: true}

				_, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
			} else {
				answer = "Okay, Dear " + messg.From.FirstName + ", I will use the voice " + text + " for your messages! You can still override voice by using square brackets."

				res := makeSpeech(answer)
				if res != nil {
					file := tgbotapi.FileReader{
						Name:   "filename",
						Reader: res,
						Size:   -1,
					}
					msg := tgbotapi.NewVoiceUpload(messg.Chat.ID, file)
					msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true, Selective: true}
					msg.ReplyToMessageID = messg.MessageID
					_, err = bot.Send(msg)

					if err != nil {
						log.Printf("Send: %v ", err)
					}
				}

				prefs[messg.From.ID] = text
				_ = fsm.state.Event("setvoice")
			}
			continue
		case "waitidiom":
			split := strings.Split(text, " ")
			answer := getIdiom(strings.Join(split, "+"))
			if answer == "" {
				answer = "Sorry, nothing about " + text
			}
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			_ = fsm.state.Event("setterm")
			continue
		case "waitoxford":
			answer := getOxfordDefinition(text)
			if answer == "" {
				answer = "Sorry, nothing about " + text + "\n You can edit your message or send new \n Or /cancel"
			} else {
				_ = fsm.state.Event("setterm")
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

		if strings.HasPrefix(strings.ToUpper(text), "/NEWBINGO") {
			var words []string
			split := strings.Split(text, " ")
			if len(split) > 1 {
				words = append(words, split[1:]...)
			}

			sg[messg.From.ID] = NewSheetGenerator(words, time.Now().Unix())
			answer := fmt.Sprintf("New bingo generator created. \n There are %d words in it\n You can add new with /ADD word command", len(words))
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
		}
		if strings.HasPrefix(strings.ToUpper(text), "/ADD") {
			var answer string
			if _, ok := sg[messg.From.ID]; !ok {
				sg[messg.From.ID] = NewSheetGenerator([]string{}, time.Now().Unix())
			}
			var words []string
			split := strings.Split(text, " ")
			if len(split) > 1 {
				words = append(words, split[1:]...)
				for _, w := range words {
					sg[messg.From.ID].AddWord(w)
				}
				answer = fmt.Sprintf("%d words added\n Thank you. You can use /BINGO now", len(words))
			} else {
				answer = "Give me a word or two to add"
			}
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}

		}
		if strings.HasPrefix(strings.ToUpper(text), "/BINGO") {
			var answer string
			if _, ok := sg[messg.From.ID]; !ok {
				answer = "I don't have words for you. Create own by /NEWBINGO"
			} else {
				split := strings.Split(text, " ")
				var size int
				if len(split) > 1 {
					size, _ = strconv.Atoi(split[1])
				}
				words := sg[messg.From.ID].GenerateOne(size)
				var captions []string
				for range words[0] {
					captions = append(captions, "-")
				}

				buf := bytes.NewBufferString("```")
				table := tablewriter.NewWriter(buf)
				table.SetHeader(captions)
				table.AppendBulk(words)
				table.Render()
				buf.WriteString("```")
				answer = buf.String()
			}

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ParseMode = "Markdown"
			// msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}

		}
		if strings.HasPrefix(strings.ToUpper(text), "/HELP") {
			answer := `You can send me any text to read aloud, but please mention me by @` + name +
				` /oxford term :  show the definition from Oxford dictionary
			   /idiom term  :  show the definition from idioms.thefreedictionary.com`

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
				_ = fsm.state.Event("waitidiom")
			} else {
				answer = getIdiom(strings.Join(split[1:], "+"))
				if answer == "" {
					answer = "Sorry, nothing about " + strings.Join(split[1:], " ")
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
				_ = fsm.state.Event("waitoxford")
			} else {
				answer = getOxfordDefinition(split[1])
				if answer == "" {
					answer = "Sorry, nothing about " + split[1]
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
			_ = fsm.state.Event("waitvoice")

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

		if !strings.Contains(strings.ToUpper(text), strings.ToUpper(name)) {
			continue
		}

		log.Printf("[%s] %s \n", messg.From.UserName, text)

		text = regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(text, "")
		text = removeVoice.ReplaceAllLiteralString(text, "")

		// sendAudio(text, voice, messg.From.ID, messg.Chat.ID, messg.MessageID)
		res := makeSpeech(text)
		if res != nil {
			file := tgbotapi.FileReader{
				Name:   "filename",
				Reader: res,
				Size:   -1,
			}
			msg := tgbotapi.NewVoiceUpload(messg.Chat.ID, file)
			msg.ReplyToMessageID = messg.MessageID
			_, err = bot.Send(msg)

			if err != nil {
				log.Printf("Send: %v ", err)
			}
		}
	}
}

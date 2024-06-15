package main

import (
	"log"
	"os"
	"regexp"
	"time"

	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type message struct {
	text string
	user string
}

var removeVoice = regexp.MustCompile(`\[(\w+)\]`)

func botGo(filter filterFunc) {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	_, err = bot.GetMe()
	if err != nil {
		log.Panicf("me: %#v \n", err)
	}

	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Printf("[ERROR] getting update channel %v\n", err)
	}

	var messages [300]message
	ts := time.Now().Add(-time.Hour)

	for update := range updates {
		if update.Message == nil && update.EditedMessage == nil && update.CallbackQuery == nil {
			continue
		}
		var messg *tgbotapi.Message

		if update.Message != nil {
			messg = update.Message
		}
		if update.EditedMessage != nil {
			messg = update.EditedMessage
		}
		if messg.From.ID == bot.Self.ID ||
			update.EditedMessage != nil {
			continue
		}
		text := messg.Text

		// lets put text and messg.From.UserName to a slice, let's keep last 300 messages and remove the oldest one

		for i := 0; i < 299; i++ {
			messages[i] = messages[i+1]
		}
		messages[299].text = text
		messages[299].user = messg.From.UserName

		if strings.HasPrefix(strings.ToUpper(text), "/HELP") {
			answer := `I read all the message and make summary of them when asked.`

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}

		if client != nil {
			if strings.HasPrefix(strings.ToUpper(text), "/SUMMARY") { // || messg.ReplyToMessage != nil &&
				// messg.ReplyToMessage.From.ID == bot.Self.ID) {
				var txt string
				if time.Since(ts).Seconds() < 360 {
					txt = "I can't make summary more often than once in 6 minutes."
				} else {
					txt, err = getGPTAnswer(messagesToText(messages))
					if err != nil {
						log.Printf("chat: %v ", err)
					}
					ts = time.Now()
				}
				msg := tgbotapi.NewMessage(messg.Chat.ID, txt)
				msg.ReplyToMessageID = messg.MessageID
				_, err = bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
				continue
			}
		}

	}
}

type filterFunc func(string) bool

func generatorOfContainFuncs(keywords []string) filterFunc {
	if len(keywords) == 1 {
		return func(msg string) bool {
			return false
		}
	}
	return func(msg string) bool {
		for _, keyword := range keywords {
			if strings.Contains(strings.ToUpper(msg), strings.ToUpper(keyword)) {
				log.Printf("keyword: %s <- %s", keyword, msg)
				return true
			}
		}
		return false
	}
}

func messagesToText(messages [300]message) string {
	var text string
	for i := 0; i < 300; i++ {
		if messages[i].text == "" {
			continue
		}
		text += messages[i].user + ": " + messages[i].text + "\n"
	}
	return text
}

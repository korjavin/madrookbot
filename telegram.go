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
	bot *tgbotapi.BotAPI
)

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

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		if !strings.Contains(strings.ToUpper(update.Message.Text), "@MADROOKBOT") {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		text := regexp.MustCompile(`(?i)@madrookbot`).ReplaceAllLiteralString(update.Message.Text, "")

		err = makeAudio(update.Message.Chat.ID, text)
		if err != nil {
			log.Printf("Make: %v ", err)
		}

		fileext := fmt.Sprintf("file_%06d.mp3", update.Message.Chat.ID)

		msg := tgbotapi.NewAudioUpload(update.Message.Chat.ID, fileext)
		msg.Title = "Voice"
		msg.Performer = "MadRookBot"
		msg.MimeType = "audio/mpeg"
		msg.ReplyToMessageID = update.Message.MessageID
		_, err := bot.Send(msg)

		if err != nil {
			log.Printf("Send: %v ", err)
		}
	}
}

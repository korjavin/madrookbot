package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	lru "github.com/hashicorp/golang-lru"
)

var cache *lru.Cache // Define the cache variable

func init() {
	var err error
	cache, err = lru.New(2000) // Initialize the cache with a size of 100
	if err != nil {
		log.Panic("Can't initialize cache")
	}
}

func botGo(buddy string) {
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

	for update := range updates {
		if update.Message == nil {
			continue
		}
		var from int
		var messg *tgbotapi.Message

		if update.Message != nil {
			messg = update.Message
		}

		from = messg.From.ID

		if from == bot.Self.ID {
			continue
		}

		cache.Add(messg.MessageID, messg)

		if client != nil {
			if messg.From.UserName == buddy {
				var dialogue []*tgbotapi.Message
				dialogue = append(dialogue, messg)
				chainmsg := messg
				for chainmsg.ReplyToMessage != nil {
					if m, exists := cache.Get(chainmsg.ReplyToMessage.MessageID); exists {
						dialogue = append(dialogue, m.(*tgbotapi.Message))
						chainmsg = m.(*tgbotapi.Message)
					} else {
						break
					}
				}
				txt, err := getGPTAnswer(dialogue)
				if err != nil {
					log.Printf("chat: %v ", err)
				}
				msg := tgbotapi.NewMessage(messg.Chat.ID, txt)
				msg.ReplyToMessageID = messg.MessageID
				m, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
				cache.Add(m.MessageID, &m)
				continue
			}
		}

	}
}

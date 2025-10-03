package main

import (
	"fmt"
	"log"
	"net/url"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// pinMessage pins a message in a chat
func pinMessage(bot *tgbotapi.BotAPI, chatID int64, messageID int) error {
	params := url.Values{}
	params.Set("chat_id", strconv.FormatInt(chatID, 10))
	params.Set("message_id", strconv.Itoa(messageID))
	params.Set("disable_notification", "false")

	_, err := bot.MakeRequest("pinChatMessage", params)
	if err != nil {
		return fmt.Errorf("failed to pin message: %v", err)
	}

	log.Printf("[TELEGRAM] Pinned message %d in chat %d", messageID, chatID)
	return nil
}

// unpinMessage unpins a specific message in a chat
func unpinMessage(bot *tgbotapi.BotAPI, chatID int64, messageID int) error {
	params := url.Values{}
	params.Set("chat_id", strconv.FormatInt(chatID, 10))
	params.Set("message_id", strconv.Itoa(messageID))

	_, err := bot.MakeRequest("unpinChatMessage", params)
	if err != nil {
		return fmt.Errorf("failed to unpin message: %v", err)
	}

	log.Printf("[TELEGRAM] Unpinned message %d in chat %d", messageID, chatID)
	return nil
}

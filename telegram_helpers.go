package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// pinMessage pins a message in a chat
func pinMessage(bot *tgbotapi.BotAPI, chatID int64, messageID int) error {
	params := make(tgbotapi.Params)
	params["chat_id"] = strconv.FormatInt(chatID, 10)
	params["message_id"] = strconv.Itoa(messageID)
	params["disable_notification"] = "false"

	_, err := bot.MakeRequest("pinChatMessage", params)
	if err != nil {
		return fmt.Errorf("failed to pin message: %v", err)
	}

	log.Printf("[TELEGRAM] Pinned message %d in chat %d", messageID, chatID)
	return nil
}

// unpinMessage unpins a specific message in a chat
func unpinMessage(bot *tgbotapi.BotAPI, chatID int64, messageID int) error {
	params := make(tgbotapi.Params)
	params["chat_id"] = strconv.FormatInt(chatID, 10)
	params["message_id"] = strconv.Itoa(messageID)

	_, err := bot.MakeRequest("unpinChatMessage", params)
	if err != nil {
		return fmt.Errorf("failed to unpin message: %v", err)
	}

	log.Printf("[TELEGRAM] Unpinned message %d in chat %d", messageID, chatID)
	return nil
}

// splitMessageText splits a long message into chunks that fit Telegram's character limit
// Telegram's limit is 4096 characters per message
func splitMessageText(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		// If remaining text fits in one message, add it and we're done
		if len(remaining) <= maxLength {
			chunks = append(chunks, remaining)
			break
		}

		// Find a good breaking point (prefer newline, then space, then hard cut)
		cutPoint := maxLength

		// Look for newline in the last 200 characters of the chunk
		if lastNewline := findLastIndex(remaining[:maxLength], "\n"); lastNewline > maxLength-200 && lastNewline > 0 {
			cutPoint = lastNewline + 1 // Include the newline
		} else if lastSpace := findLastIndex(remaining[:maxLength], " "); lastSpace > maxLength-100 && lastSpace > 0 {
			// Look for space in the last 100 characters of the chunk
			cutPoint = lastSpace + 1 // Include the space
		}

		chunks = append(chunks, remaining[:cutPoint])
		remaining = remaining[cutPoint:]
	}

	return chunks
}

// findLastIndex returns the index of the last instance of substr in s, or -1 if not found
func findLastIndex(s, substr string) int {
	// Simple implementation of strings.LastIndex for clarity
	if len(substr) == 0 {
		return len(s)
	}
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// sendMessageWithSplit sends a message to Telegram, automatically splitting it if too long
// Returns the last sent message and any error encountered
// If the message is under the limit, it's sent as-is
// If over the limit, it's split into chunks and sent sequentially with 500ms delay between chunks
func sendMessageWithSplit(bot *tgbotapi.BotAPI, msg tgbotapi.MessageConfig) (tgbotapi.Message, error) {
	const telegramMaxLength = 4096

	// Check if message needs splitting
	if len(msg.Text) <= telegramMaxLength {
		return bot.Send(msg)
	}

	// Split the message
	chunks := splitMessageText(msg.Text, telegramMaxLength)
	log.Printf("[TELEGRAM] Message too long (%d chars), split into %d chunk(s)", len(msg.Text), len(chunks))

	var lastMsg tgbotapi.Message
	var err error

	// Send each chunk
	for i, chunk := range chunks {
		chunkMsg := tgbotapi.MessageConfig{
			BaseChat: msg.BaseChat,
			Text:     chunk,
		}
		// Preserve reply markup only on the last chunk
		if i == len(chunks)-1 {
			chunkMsg.ReplyMarkup = msg.ReplyMarkup
		}
		// Preserve reply to message ID only on the first chunk
		if i == 0 {
			chunkMsg.ReplyToMessageID = msg.ReplyToMessageID
		}

		lastMsg, err = bot.Send(chunkMsg)
		if err != nil {
			return lastMsg, fmt.Errorf("failed to send chunk %d/%d: %w", i+1, len(chunks), err)
		}

		log.Printf("[TELEGRAM] Successfully sent chunk %d/%d (message ID: %d)", i+1, len(chunks), lastMsg.MessageID)

		// Small delay between chunks to avoid rate limiting
		if i < len(chunks)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return lastMsg, nil
}

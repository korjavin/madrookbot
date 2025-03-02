package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// handleMediaSuggestion checks if a message contains a media suggestion
// and handles it accordingly
func handleMediaSuggestion(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// Only process in group chats
	if !message.Chat.IsGroup() && !message.Chat.IsSuperGroup() {
		return
	}

	// Get class group ID
	classGroupID, err := getClassGroupID()
	if err != nil || int64(message.Chat.ID) != classGroupID {
		return
	}

	// Check if message contains a suggestion
	suggestedURL := ExtractSuggestionFromMessage(message.Text)
	if suggestedURL == "" {
		return
	}

	// Add to database
	_, err = AddMediaSuggestion(suggestedURL, message.From.UserName)
	if err != nil {
		log.Printf("Error adding media suggestion: %v", err)
		return
	}

	// Acknowledge the suggestion
	reply := fmt.Sprintf("Thanks for your suggestion, @%s! I've added it to our collection.",
		message.From.UserName)
	msg := tgbotapi.NewMessage(message.Chat.ID, reply)
	msg.ReplyToMessageID = message.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending suggestion acknowledgment: %v", err)
	}
}

// handleMediaSelection handles the selection of media by the owner
func handleMediaSelection(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	// Only process in group chats
	if !message.Chat.IsGroup() && !message.Chat.IsSuperGroup() {
		return false
	}

	// Get class group ID and owner ID
	classGroupID, err := getClassGroupID()
	if err != nil || int64(message.Chat.ID) != classGroupID {
		return false
	}

	ownerID, err := getOwnerID()
	if err != nil || message.From.ID != ownerID {
		return false
	}

	// Check if the message is a number
	selectionStr := strings.TrimSpace(message.Text)
	selectionNum, err := strconv.Atoi(selectionStr)
	if err != nil {
		return false
	}

	// Get current suggestions
	suggestions, err := GetMediaSuggestions()
	if err != nil {
		log.Printf("Error getting media suggestions: %v", err)
		return false
	}

	// Check if selection is valid
	if selectionNum <= 0 || selectionNum > len(suggestions) {
		reply := fmt.Sprintf("Invalid selection number. Please choose a number between 1 and %d.", len(suggestions))
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return true
	}

	// Get selected media
	selected := suggestions[selectionNum-1]

	// Mark as selected
	err = SelectMedia(selected.ID)
	if err != nil {
		log.Printf("Error selecting media: %v", err)
		return false
	}

	// Announce selection
	announcement := fmt.Sprintf("🎉 *Media Selected for This Week*\n\n"+
		"We'll be discussing:\n[%s](%s)\n\nSuggested by: @%s\n\n"+
		"Join our discussion on Sunday at 18:00 Berlin time!",
		truncateURL(selected.URL),
		selected.URL,
		selected.Suggester)

	msg := tgbotapi.NewMessage(message.Chat.ID, announcement)
	msg.ParseMode = "Markdown"
	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending selection announcement: %v", err)
		return true

	}

	_ = sentMsg
	/*

		// Doesnt work because of tgbotapi limits. Uncomment it when migrate to another lib

		// Pin the message
		pinCfg := tgbotapi.NewPinChatMessageConfig(message.Chat.ID, sentMsg.MessageID)
		pinCfg.DisableNotification = false
		_, err = bot.Request(pinCfg)
		if err != nil {
			log.Printf("Error pinning message: %v", err)
		}

	*/

	return true
}

// showCurrentMedia shows the current media list
func showCurrentMedia(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	suggestions, err := GetMediaSuggestions()
	if err != nil {
		log.Printf("Error getting media suggestions: %v", err)
		reply := "Sorry, I couldn't retrieve the current media list."
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	mediaList := FormatMediaList(suggestions)
	msg := tgbotapi.NewMessage(message.Chat.ID, mediaList)
	msg.ParseMode = "Markdown"
	msg.ReplyToMessageID = message.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending media list: %v", err)
	}
}

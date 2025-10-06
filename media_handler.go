package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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
		"We'll be discussing:\n(%s)\n\nSuggested by: @%s\n\n"+
		"Join our discussion on Sunday at 18:00 Berlin time!",
		selected.URL,
		selected.Suggester)

	msg := tgbotapi.NewMessage(message.Chat.ID, announcement)
	msg.DisableWebPagePreview = true
	msg.ParseMode = ""
	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("Error sending selection announcement: %v", err)
		return true

	}

	_ = sentMsg

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
	msg.DisableWebPagePreview = true
	msg.ReplyToMessageID = message.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending media list: %v at %v", err, mediaList)
	}
}

// handleDeleteSuggestion handles the deletion of a media suggestion
func handleDeleteSuggestion(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// Check if the message has the format /del N where N is a number
	text := message.Text
	if !strings.HasPrefix(strings.ToUpper(text), "/DEL ") {
		return
	}

	// Extract the ID from the command
	parts := strings.Fields(text)
	if len(parts) != 2 {
		reply := "Invalid format. Use /del [number] to delete a suggestion."
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	// Parse the number to get the index
	indexStr := parts[1]
	index, err := strconv.Atoi(indexStr)
	if err != nil || index <= 0 {
		reply := "Invalid number. Please provide a positive number."
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	// Get all suggestions
	suggestions, err := GetMediaSuggestions()
	if err != nil {
		log.Printf("Error getting media suggestions: %v", err)
		reply := "Sorry, I couldn't retrieve the current media list."
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	// Check if the index is valid
	if index > len(suggestions) {
		reply := fmt.Sprintf("Invalid number. There are only %d suggestions.", len(suggestions))
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	// Get the suggestion by index (0-based array vs 1-based user input)
	suggestion := suggestions[index-1]

	// Check if the user is authorized to delete this suggestion
	ownerID, err := getOwnerID()
	isOwner := err == nil && message.From.ID == ownerID
	isSuggester := message.From.UserName == suggestion.Suggester

	if !isOwner && !isSuggester {
		reply := "You can only delete your own suggestions, unless you're the owner."
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	// Delete the suggestion
	err = DeleteMedia(suggestion.ID)
	if err != nil {
		log.Printf("Error deleting suggestion: %v", err)
		reply := "Sorry, I couldn't delete the suggestion. Please try again later."
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		msg.ReplyToMessageID = message.MessageID
		bot.Send(msg)
		return
	}

	// Confirm the deletion
	reply := fmt.Sprintf("Successfully deleted suggestion: [%s](%s)",
		truncateURL(suggestion.URL), suggestion.URL)
	msg := tgbotapi.NewMessage(message.Chat.ID, reply)
	msg.ReplyToMessageID = message.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending confirmation: %v", err)
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleMediaSelection handles the selection of media by the owner
func handleMediaSelection(b *bot.Bot, ctx context.Context, message *models.Message) bool {
	// Only process in group chats
	if message.Chat.Type != "group" && message.Chat.Type != "supergroup" {
		return false
	}

	// Get class group ID and owner ID
	classGroupID, err := getClassGroupID()
	if err != nil || message.Chat.ID != classGroupID {
		return false
	}

	ownerID, err := getOwnerID()
	if err != nil || message.From.ID != int64(ownerID) {
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
		params := &bot.SendMessageParams{
			ChatID:           message.Chat.ID,
			Text:             reply,
			ReplyToMessageID: message.ID,
		}
		b.SendMessage(ctx, params)
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

	params := &bot.SendMessageParams{
		ChatID:                message.Chat.ID,
		Text:                  announcement,
		DisableWebPagePreview: true,
		ReplyToMessageID:      message.ID,
	}

	sentMsg, err := b.SendMessage(ctx, params)
	if err != nil {
		log.Printf("Error sending selection announcement: %v", err)
		return true
	}

	// Pin message
	pinParams := &bot.PinChatMessageParams{
		ChatID:    message.Chat.ID,
		MessageID: sentMsg.ID,
	}
	_, err = b.PinChatMessage(ctx, pinParams)
	if err != nil {
		log.Printf("Error pinning message: %v", err)
	}

	return true
}

// showCurrentMedia shows the current media list
func showCurrentMedia(b *bot.Bot, ctx context.Context, message *models.Message) {
	suggestions, err := GetMediaSuggestions()
	if err != nil {
		log.Printf("Error getting media suggestions: %v", err)
		params := &bot.SendMessageParams{
			ChatID:           message.Chat.ID,
			Text:             "Sorry, I couldn't retrieve the current media list.",
			ReplyToMessageID: message.ID,
		}
		b.SendMessage(ctx, params)
		return
	}

	mediaList := FormatMediaList(suggestions)
	params := &bot.SendMessageParams{
		ChatID:                message.Chat.ID,
		Text:                  mediaList,
		DisableWebPagePreview: true,
		ReplyToMessageID:      message.ID,
	}
	_, err = b.SendMessage(ctx, params)
	if err != nil {
		log.Printf("Error sending media list: %v at %v", err, mediaList)
	}
}

// handleDeleteSuggestion handles the deletion of a media suggestion
func handleDeleteSuggestion(b *bot.Bot, ctx context.Context, message *models.Message) {
	// Check if the message has the format /del N where N is a number
	text := message.Text
	if !strings.HasPrefix(strings.ToUpper(text), "/DEL ") {
		return
	}

	// Extract the ID from the command
	parts := strings.Fields(text)
	if len(parts) != 2 {
		reply := "Invalid format. Use /del [number] to delete a suggestion."
		params := &bot.SendMessageParams{
			ChatID:           message.Chat.ID,
			Text:             reply,
			ReplyToMessageID: message.ID,
		}
		b.SendMessage(ctx, params)
		return
	}

	// Parse the number to get the index
	indexStr := parts[1]
	index, err := strconv.Atoi(indexStr)
	if err != nil || index <= 0 {
		reply := "Invalid number. Please provide a positive number."
		params := &bot.SendMessageParams{
			ChatID:           message.Chat.ID,
			Text:             reply,
			ReplyToMessageID: message.ID,
		}
		b.SendMessage(ctx, params)
		return
	}

	// Get all suggestions
	suggestions, err := GetMediaSuggestions()
	if err != nil {
		log.Printf("Error getting media suggestions: %v", err)
		reply := "Sorry, I couldn't retrieve the current media list."
		params := &bot.SendMessageParams{
			ChatID:           message.Chat.ID,
			Text:             reply,
			ReplyToMessageID: message.ID,
		}
		b.SendMessage(ctx, params)
		return
	}

	// Check if the index is valid
	if index > len(suggestions) {
		reply := fmt.Sprintf("Invalid number. There are only %d suggestions.", len(suggestions))
		params := &bot.SendMessageParams{
			ChatID:           message.Chat.ID,
			Text:             reply,
			ReplyToMessageID: message.ID,
		}
		b.SendMessage(ctx, params)
		return
	}

	// Get the suggestion by index (0-based array vs 1-based user input)
	suggestion := suggestions[index-1]

	// Check if the user is authorized to delete this suggestion
	ownerID, err := getOwnerID()
	isOwner := err == nil && message.From.ID == int64(ownerID)
	isSuggester := message.From.Username == suggestion.Suggester

	if !isOwner && !isSuggester {
		reply := "You can only delete your own suggestions, unless you're the owner."
		params := &bot.SendMessageParams{
			ChatID:           message.Chat.ID,
			Text:             reply,
			ReplyToMessageID: message.ID,
		}
		b.SendMessage(ctx, params)
		return
	}

	// Delete the suggestion
	err = DeleteMedia(suggestion.ID)
	if err != nil {
		log.Printf("Error deleting suggestion: %v", err)
		reply := "Sorry, I couldn't delete the suggestion. Please try again later."
		params := &bot.SendMessageParams{
			ChatID:           message.Chat.ID,
			Text:             reply,
			ReplyToMessageID: message.ID,
		}
		b.SendMessage(ctx, params)
		return
	}

	// Confirm the deletion
	reply := fmt.Sprintf("Successfully deleted suggestion: [%s](%s)",
		truncateURL(suggestion.URL), suggestion.URL)
	params := &bot.SendMessageParams{
		ChatID:           message.Chat.ID,
		Text:             reply,
		ReplyToMessageID: message.ID,
	}
	_, err = b.SendMessage(ctx, params)
	if err != nil {
		log.Printf("Error sending confirmation: %v", err)
	}
}

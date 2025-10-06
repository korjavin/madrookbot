package main

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleMessageReactionUpdate processes message reaction updates
func handleMessageReactionUpdate(reaction *tgbotapi.MessageReactionUpdated) {
	if reaction == nil {
		return
	}

	// Only process reactions from users (not channels)
	if reaction.User == nil {
		return
	}

	user := reaction.User
	messageID := reaction.MessageID

	log.Printf("[REACTION] User %s (ID: %d) reacted to message %d",
		user.UserName, user.ID, messageID)
	log.Printf("[REACTION] Old reactions: %d, New reactions: %d",
		len(reaction.OldReaction), len(reaction.NewReaction))

	// Check if this is adding or removing a reaction
	if len(reaction.NewReaction) > len(reaction.OldReaction) {
		// User added a reaction - track as RSVP
		err := trackRSVP(messageID, user.ID, user.UserName)
		if err != nil {
			log.Printf("[ERROR] Failed to track RSVP: %v", err)
		} else {
			log.Printf("[RSVP] Tracked RSVP from %s for message %d", user.UserName, messageID)
		}
	} else if len(reaction.NewReaction) < len(reaction.OldReaction) {
		// User removed a reaction - remove RSVP
		err := removeRSVP(messageID, user.ID)
		if err != nil {
			log.Printf("[ERROR] Failed to remove RSVP: %v", err)
		} else {
			log.Printf("[RSVP] Removed RSVP from %s for message %d", user.UserName, messageID)
		}
	}
}

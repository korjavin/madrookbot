package main

import (
	"encoding/json"
	"log"
)

// MessageReactionUpdated represents a change of a reaction on a message performed by a user
type MessageReactionUpdated struct {
	Chat        Chat          `json:"chat"`
	MessageID   int           `json:"message_id"`
	User        *User         `json:"user,omitempty"`
	ActorChat   *Chat         `json:"actor_chat,omitempty"`
	Date        int           `json:"date"`
	OldReaction []ReactionType `json:"old_reaction"`
	NewReaction []ReactionType `json:"new_reaction"`
}

// ReactionType describes the type of a reaction
type ReactionType struct {
	Type          string               `json:"type"`
	Emoji         string               `json:"emoji,omitempty"`
	CustomEmoji   string               `json:"custom_emoji,omitempty"`
}

// Chat represents a Telegram chat
type Chat struct {
	ID int64 `json:"id"`
}

// User represents a Telegram user
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// handleMessageReaction processes message reaction updates
func handleMessageReaction(rawUpdate json.RawMessage) {
	var updateData struct {
		MessageReaction *MessageReactionUpdated `json:"message_reaction"`
	}

	err := json.Unmarshal(rawUpdate, &updateData)
	if err != nil {
		log.Printf("[ERROR] Failed to parse message_reaction update: %v", err)
		return
	}

	if updateData.MessageReaction == nil {
		return
	}

	reaction := updateData.MessageReaction

	// Only process reactions from users (not channels)
	if reaction.User == nil {
		return
	}

	user := reaction.User
	messageID := reaction.MessageID

	log.Printf("[REACTION] User %s (ID: %d) reacted to message %d",
		user.Username, user.ID, messageID)
	log.Printf("[REACTION] Old reactions: %d, New reactions: %d",
		len(reaction.OldReaction), len(reaction.NewReaction))

	// Check if this is adding or removing a reaction
	if len(reaction.NewReaction) > len(reaction.OldReaction) {
		// User added a reaction - track as RSVP
		err := trackRSVP(messageID, user.ID, user.Username)
		if err != nil {
			log.Printf("[ERROR] Failed to track RSVP: %v", err)
		} else {
			log.Printf("[RSVP] Tracked RSVP from %s for message %d", user.Username, messageID)
		}
	} else if len(reaction.NewReaction) < len(reaction.OldReaction) {
		// User removed a reaction - remove RSVP
		err := removeRSVP(messageID, user.ID)
		if err != nil {
			log.Printf("[ERROR] Failed to remove RSVP: %v", err)
		} else {
			log.Printf("[RSVP] Removed RSVP from %s for message %d", user.Username, messageID)
		}
	}
}

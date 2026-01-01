package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleHelpCommand handles the /help command
func handleHelpCommand(bot *tgbotapi.BotAPI, messg *tgbotapi.Message, botName string) {
	answer := `Commands:
/idiom <term> - Show the definition from idioms.thefreedictionary.com
/stat - Show group activity statistics (admins only, 1/hour)

Mention me @` + botName + ` to ask questions (reply to continue conversation)`

	// Add optional features if enabled
	if geminiClient != nil {
		answer += `
Use "image: <prompt>" to generate images with AI`
	}
	answer += `
Use "read: <text>" to convert text to speech`

	msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
	msg.ReplyToMessageID = messg.MessageID
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Send: %v ", err)
	}
}

// handleClassCommand handles the /class command (owner only)
func handleClassCommand(bot *tgbotapi.BotAPI, messg *tgbotapi.Message) {
	if !isOwner(messg.From.ID) {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can create classes.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	description := strings.TrimSpace(messg.Text[7:]) // Remove "/class "
	description = sanitizeClassDescription(description)

	if description == "" {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Please provide a class description. Usage: /class <description>")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if there's already an active class
	existingClass, err := getActiveClass()
	if err != nil {
		log.Printf("[ERROR] Failed to check for active class: %v", err)
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Error checking for existing classes.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	if existingClass != nil && !existingClass.Unpinned {
		msg := tgbotapi.NewMessage(messg.Chat.ID,
			fmt.Sprintf("There's already an active class scheduled: %s\nUse /cancelclass first if you want to replace it.",
				existingClass.Description))
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	class, err := createClass(description)
	if err != nil {
		log.Printf("[ERROR] Failed to create class: %v", err)
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Failed to create class. Please try again.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	berlin, _ := time.LoadLocation("Europe/Berlin")
	msg := tgbotapi.NewMessage(messg.Chat.ID,
		fmt.Sprintf("✅ Class created!\n\nTopic: %s\nScheduled: %s\n\nAnnouncement will be posted soon.",
			class.Description,
			class.ScheduledTime.In(berlin).Format("Monday, January 2 at 15:04 MST")))
	msg.ReplyToMessageID = messg.MessageID
	bot.Send(msg)
}

// handleCancelClassCommand handles the /cancelclass command (owner only)
func handleCancelClassCommand(bot *tgbotapi.BotAPI, messg *tgbotapi.Message) {
	if !isOwner(messg.From.ID) {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can cancel classes.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	class, err := getActiveClass()
	if err != nil {
		log.Printf("[ERROR] Failed to get active class: %v", err)
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Error checking for active classes.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	if class == nil {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "No active class to cancel.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	err = cancelClass(class.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to cancel class: %v", err)
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Failed to cancel class.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Unpin the announcement if it was posted
	if class.AnnouncementMessageID > 0 && !class.Unpinned {
		groupID, _ := getClassGroupID()
		unpinMessage(bot, groupID, class.AnnouncementMessageID)
	}

	msg := tgbotapi.NewMessage(messg.Chat.ID,
		fmt.Sprintf("❌ Class cancelled: %s", class.Description))
	msg.ReplyToMessageID = messg.MessageID
	bot.Send(msg)
}

// handleIdiomCommand handles the /idiom command
func handleIdiomCommand(bot *tgbotapi.BotAPI, messg *tgbotapi.Message) {
	split := strings.Split(messg.Text, " ")
	answer := ""
	if len(split) < 2 {
		answer = "Please provide a term to search. Usage: /idiom <term>"
	} else {
		answer = getIdiom(strings.Join(split[1:], "+"))
		if answer == "" {
			answer = "Sorry, nothing found about " + strings.Join(split[1:], " ")
		}
	}

	msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
	msg.ReplyToMessageID = messg.MessageID
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Send: %v ", err)
	}
}

// handleINewsCommand handles the /inews command (owner only, DM only)
func handleINewsCommand(bot *tgbotapi.BotAPI, messg *tgbotapi.Message) {
	// Check if it's a DM (private chat)
	if !messg.Chat.IsPrivate() {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "This command only works in direct messages.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if user is owner
	if !isOwner(messg.From.ID) {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can use this command.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if memory group is set
	if os.Getenv("MEMORY_GROUP_ID") == "" {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (MEMORY_GROUP_ID not set).")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if RANDOM_MESSAGE_PROMPT is set
	if os.Getenv("RANDOM_MESSAGE_PROMPT") == "" {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (RANDOM_MESSAGE_PROMPT not set).")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Start interactive session
	startInteractiveNewsSession(bot, messg.From.ID, messg.Chat.ID)
}

// handleNewsCommand handles the /news command (owner only, DM only)
func handleNewsCommand(bot *tgbotapi.BotAPI, messg *tgbotapi.Message) {
	// Check if it's a DM (private chat)
	if !messg.Chat.IsPrivate() {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "This command only works in direct messages.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if user is owner
	if !isOwner(messg.From.ID) {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can use this command.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if memory group is set
	if os.Getenv("MEMORY_GROUP_ID") == "" {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (MEMORY_GROUP_ID not set).")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if RANDOM_MESSAGE_PROMPT is set
	if os.Getenv("RANDOM_MESSAGE_PROMPT") == "" {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (RANDOM_MESSAGE_PROMPT not set).")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Send "working on it" message
	workingMsg := tgbotapi.NewMessage(messg.Chat.ID, "🔄 Testing random message feature...\n\n1. Getting random message from last 20 hours...")
	sentWorking, _ := bot.Send(workingMsg)

	// Step 1: Get random message
	randomMessage, err := getRandomRecentMessage()
	if err != nil {
		editMsg := tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
			fmt.Sprintf("❌ Failed to get random message: %v", err))
		bot.Send(editMsg)
		return
	}

	editMsg := tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
		fmt.Sprintf("✅ Got message (%.100s...)\n\n2. Extracting topic...", randomMessage))
	bot.Send(editMsg)

	// Step 2: Extract topic
	topic, err := extractTopicFromMessage(randomMessage)
	if err != nil {
		editMsg := tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
			fmt.Sprintf("❌ Failed to extract topic: %v", err))
		bot.Send(editMsg)
		return
	}

	editMsg = tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
		fmt.Sprintf("✅ Extracted topic: \"%s\"\n\n3. Generating random message...", topic))
	bot.Send(editMsg)

	// Step 3: Generate random message
	generatedMessage, err := generateRandomMessage(topic)
	if err != nil {
		editMsg := tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
			fmt.Sprintf("❌ Failed to generate message: %v", err))
		bot.Send(editMsg)
		return
	}

	// Send final result
	editMsg = tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
		"✅ Test complete! Here's what would be posted:")
	bot.Send(editMsg)

	resultMsg := tgbotapi.NewMessage(messg.Chat.ID, generatedMessage)
	bot.Send(resultMsg)

	// Send debug info
	debugMsg := tgbotapi.NewMessage(messg.Chat.ID,
		fmt.Sprintf("📊 Debug info:\n• Topic: %s\n• Source message: %.100s...",
			topic, randomMessage))
	bot.Send(debugMsg)

	// Reset alert state since /NEWS was used successfully
	resetNewsFeature()
}

// handleSendToChanCommand handles the /send_to_chan command (owner only, DM only)
func handleSendToChanCommand(bot *tgbotapi.BotAPI, messg *tgbotapi.Message) {
	// Check if it's a DM (private chat)
	if !messg.Chat.IsPrivate() {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "This command only works in direct messages.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if user is owner
	if !isOwner(messg.From.ID) {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can use this command.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if memory group is set
	memoryGroupIDStr := os.Getenv("MEMORY_GROUP_ID")
	if memoryGroupIDStr == "" {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (MEMORY_GROUP_ID not set).")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	memoryGroupID, err := strconv.ParseInt(memoryGroupIDStr, 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(messg.Chat.ID, fmt.Sprintf("Invalid MEMORY_GROUP_ID: %v", err))
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if RANDOM_MESSAGE_PROMPT is set
	if os.Getenv("RANDOM_MESSAGE_PROMPT") == "" {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (RANDOM_MESSAGE_PROMPT not set).")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Extract message from command
	prefix := "/SEND_TO_CHAN "
	if !strings.HasPrefix(strings.ToUpper(messg.Text), "/SEND_TO_CHAN") {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Invalid command format. Usage: /send_to_chan <message>")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	message := strings.TrimSpace(messg.Text[len(prefix):])
	if message == "" {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Please provide a message to send. Usage: /send_to_chan <message>")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Send message to group
	groupMsg := tgbotapi.NewMessage(memoryGroupID, message)
	sentGroupMsg, err := bot.Send(groupMsg)
	if err != nil {
		msg := tgbotapi.NewMessage(messg.Chat.ID, fmt.Sprintf("❌ Failed to send message: %v", err))
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Confirm success
	msg := tgbotapi.NewMessage(messg.Chat.ID, fmt.Sprintf("✅ Message sent to channel!\n\nMessage ID: %d", sentGroupMsg.MessageID))
	msg.ReplyToMessageID = messg.MessageID
	bot.Send(msg)
}

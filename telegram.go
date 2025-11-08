package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	openai "github.com/sashabaranov/go-openai"
)

type class struct {
	Topic     string
	Date      time.Time
	MessageID int
}

var currentClass class

// InteractiveNewsSession stores the state of an interactive news session
type InteractiveNewsSession struct {
	Step             string   // "topic_selection", "message_generation"
	TopicCandidates  []string // 3 topic candidates
	SelectedTopic    string   // The chosen topic
	GeneratedMessage string   // The generated message
	SourceMessages   []string // Original messages for candidates
	MessageID        int      // Bot's message ID for editing
}

var interactiveSessions map[int64]*InteractiveNewsSession // key: user ID

func init() {
	interactiveSessions = make(map[int64]*InteractiveNewsSession)
}

func botGo() {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	me, err := bot.GetMe()
	if err != nil {
		log.Panicf("me: %#v \n", err)
	}
	name := me.UserName

	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Start class scheduler
	startClassScheduler(bot)

	// Start news scheduler
	startNewsScheduler(bot)

	u := tgbotapi.NewUpdate(-1) // Use -1 to get the latest updates and skip old ones
	u.Timeout = 60
	// Enable message_reaction updates to track RSVPs
	u.AllowedUpdates = []string{"message", "edited_message", "callback_query", "message_reaction"}

	log.Printf("[DEBUG] Requesting updates with offset %d and AllowedUpdates: %v", u.Offset, u.AllowedUpdates)
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		log.Printf("[DEBUG] Received update ID %d - Message:%v EditedMsg:%v Callback:%v Reaction:%v",
			update.UpdateID,
			update.Message != nil,
			update.EditedMessage != nil,
			update.CallbackQuery != nil,
			update.MessageReaction != nil)

		// Handle message reactions
		if update.MessageReaction != nil {
			log.Printf("[DEBUG] Processing MessageReaction update")
			handleMessageReactionUpdate(update.MessageReaction)
			continue
		}

		// Handle callback queries (button clicks)
		if update.CallbackQuery != nil {
			handleInteractiveNewsCallback(bot, update.CallbackQuery)
			continue
		}

		if update.Message == nil && update.EditedMessage == nil {
			log.Printf("[DEBUG] Skipping update - no recognized content")
			continue
		}
		var text string
		var messg *tgbotapi.Message
		isNewMessage := false

		if update.Message != nil {
			messg = update.Message
			isNewMessage = true
		}
		if update.EditedMessage != nil {
			messg = update.EditedMessage
		}

		text = messg.Text

		if messg.Chat.IsGroup() || messg.Chat.IsSuperGroup() {
			// Track user activity for all groups
			if messg.From != nil && messg.From.UserName != "" {
				err := trackUserActivity(messg.Chat.ID, messg.From.UserName)
				if err != nil {
					log.Printf("[ERROR] Failed to track activity: %v", err)
				}
			}

			// Memory feature: process and store message if it's from the memory group
			memoryGroupIDStr := os.Getenv("MEMORY_GROUP_ID")
			if memoryGroupIDStr != "" {
				memoryGroupID, err := getEnvInt64("MEMORY_GROUP_ID")
				if err == nil && messg.Chat.ID == memoryGroupID && text != "" && !strings.HasPrefix(text, "/") {
					go processMessageForMemory(messg)
				}
			}

			// English review feature - randomly check NEW messages only (not edits) for major mistakes
			minLength := getReviewMinLength()
			if !isNewMessage && text != "" {
				log.Printf("[DEBUG] Skipping review for edited message from user=%s, userID=%d",
					messg.From.UserName, messg.From.ID)
			}
			if isNewMessage && text != "" && isReviewableMessage(text, minLength, messg.From.ID) && shouldReviewMessage() {
				log.Printf("[REVIEW] Selected for review: user=%s, userID=%d, chatID=%d, msgID=%d, length=%d",
					messg.From.UserName, messg.From.ID, messg.Chat.ID, messg.MessageID, len(text))
				review, err := reviewEnglish(text)
				if err == nil && review != "" {
					log.Printf("[REVIEW] Sending review to chatID=%d, replyTo=%d",
						messg.Chat.ID, messg.MessageID)
					msg := tgbotapi.NewMessage(messg.Chat.ID, review)
					msg.ReplyToMessageID = messg.MessageID
					_, err = bot.Send(msg)
					if err != nil {
						log.Printf("[ERROR] Failed to send review: %v", err)
					} else {
						log.Printf("[REVIEW] Review sent successfully")
					}
				} else if err != nil {
					log.Printf("[REVIEW] Review failed with error: %v", err)
				} else {
					log.Printf("[REVIEW] No review to send (no major mistakes or ambiguous response)")
				}
			}

		}

		if strings.HasPrefix(strings.ToUpper(text), "/HELP") {
			answer := `Commands:
/idiom <term> - Show the definition from idioms.thefreedictionary.com
/stat - Show group activity statistics (admins only, 1/hour)

Mention me @` + name + ` to ask questions (reply to continue conversation)`

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
			continue
		}

		// Handle /stat command
		if strings.HasPrefix(strings.ToUpper(text), "/STAT") {
			handleStatCommand(bot, messg)
			continue
		}

		// Handle /class command (owner only)
		if strings.HasPrefix(strings.ToUpper(text), "/CLASS ") {
			if !isOwner(messg.From.ID) {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can create classes.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			description := strings.TrimSpace(text[7:]) // Remove "/class "
			description = sanitizeClassDescription(description)

			if description == "" {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Please provide a class description. Usage: /class <description>")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if there's already an active class
			existingClass, err := getActiveClass()
			if err != nil {
				log.Printf("[ERROR] Failed to check for active class: %v", err)
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Error checking for existing classes.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			if existingClass != nil && !existingClass.Unpinned {
				msg := tgbotapi.NewMessage(messg.Chat.ID,
					fmt.Sprintf("There's already an active class scheduled: %s\nUse /cancelclass first if you want to replace it.",
						existingClass.Description))
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			class, err := createClass(description)
			if err != nil {
				log.Printf("[ERROR] Failed to create class: %v", err)
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Failed to create class. Please try again.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			berlin, _ := time.LoadLocation("Europe/Berlin")
			msg := tgbotapi.NewMessage(messg.Chat.ID,
				fmt.Sprintf("✅ Class created!\n\nTopic: %s\nScheduled: %s\n\nAnnouncement will be posted soon.",
					class.Description,
					class.ScheduledTime.In(berlin).Format("Monday, January 2 at 15:04 MST")))
			msg.ReplyToMessageID = messg.MessageID
			bot.Send(msg)
			continue
		}

		// Handle /cancelclass command (owner only)
		if strings.HasPrefix(strings.ToUpper(text), "/CANCELCLASS") {
			if !isOwner(messg.From.ID) {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can cancel classes.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			class, err := getActiveClass()
			if err != nil {
				log.Printf("[ERROR] Failed to get active class: %v", err)
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Error checking for active classes.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			if class == nil {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "No active class to cancel.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			err = cancelClass(class.ID)
			if err != nil {
				log.Printf("[ERROR] Failed to cancel class: %v", err)
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Failed to cancel class.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
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
			continue
		}
		if strings.HasPrefix(strings.ToUpper(text), "/IDIOM") {
			split := strings.Split(text, " ")
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
			continue
		}

		// Handle /inews command (owner only, DM only) - Interactive news with confirmations
		if strings.HasPrefix(strings.ToUpper(text), "/INEWS") {
			// Check if it's a DM (private chat)
			if !messg.Chat.IsPrivate() {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "This command only works in direct messages.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if user is owner
			if !isOwner(messg.From.ID) {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can use this command.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if memory group is set
			if os.Getenv("MEMORY_GROUP_ID") == "" {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (MEMORY_GROUP_ID not set).")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if RANDOM_MESSAGE_PROMPT is set
			if os.Getenv("RANDOM_MESSAGE_PROMPT") == "" {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (RANDOM_MESSAGE_PROMPT not set).")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Start interactive session
			startInteractiveNewsSession(bot, messg.From.ID, messg.Chat.ID)
			continue
		}

		// Handle /news command (owner only, DM only)
		if strings.HasPrefix(strings.ToUpper(text), "/NEWS") {
			// Check if it's a DM (private chat)
			if !messg.Chat.IsPrivate() {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "This command only works in direct messages.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if user is owner
			if !isOwner(messg.From.ID) {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can use this command.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if memory group is set
			if os.Getenv("MEMORY_GROUP_ID") == "" {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (MEMORY_GROUP_ID not set).")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if RANDOM_MESSAGE_PROMPT is set
			if os.Getenv("RANDOM_MESSAGE_PROMPT") == "" {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (RANDOM_MESSAGE_PROMPT not set).")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
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
				continue
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
				continue
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
				continue
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

			continue
		}

		// Handle /send_to_chan command (owner only, DM only) - Force send message to group
		if strings.HasPrefix(strings.ToUpper(text), "/SEND_TO_CHAN") {
			// Check if it's a DM (private chat)
			if !messg.Chat.IsPrivate() {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "This command only works in direct messages.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if user is owner
			if !isOwner(messg.From.ID) {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Only the bot owner can use this command.")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if memory group is set
			memoryGroupIDStr := os.Getenv("MEMORY_GROUP_ID")
			if memoryGroupIDStr == "" {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (MEMORY_GROUP_ID not set).")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			memoryGroupID, err := strconv.ParseInt(memoryGroupIDStr, 10, 64)
			if err != nil {
				msg := tgbotapi.NewMessage(messg.Chat.ID, fmt.Sprintf("Invalid MEMORY_GROUP_ID: %v", err))
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Check if RANDOM_MESSAGE_PROMPT is set
			if os.Getenv("RANDOM_MESSAGE_PROMPT") == "" {
				msg := tgbotapi.NewMessage(messg.Chat.ID, "Random message feature is not configured (RANDOM_MESSAGE_PROMPT not set).")
				msg.ReplyToMessageID = messg.MessageID
				bot.Send(msg)
				continue
			}

			// Send "working on it" message
			workingMsg := tgbotapi.NewMessage(messg.Chat.ID, "🔄 Generating and sending message to group...\n\n1. Getting random message from last 20 hours...")
			sentWorking, _ := bot.Send(workingMsg)

			// Step 1: Get random message
			randomMessage, err := getRandomRecentMessage()
			if err != nil {
				editMsg := tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
					fmt.Sprintf("❌ Failed to get random message: %v", err))
				bot.Send(editMsg)
				continue
			}

			editMsg := tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
				fmt.Sprintf("✅ Got message\n\n2. Extracting topic..."))
			bot.Send(editMsg)

			// Step 2: Extract topic
			topic, err := extractTopicFromMessage(randomMessage)
			if err != nil {
				editMsg := tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
					fmt.Sprintf("❌ Failed to extract topic: %v", err))
				bot.Send(editMsg)
				continue
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
				continue
			}

			editMsg = tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
				fmt.Sprintf("✅ Generated message\n\n4. Sending to group..."))
			bot.Send(editMsg)

			// Step 4: Send to group
			groupMsg := tgbotapi.NewMessage(memoryGroupID, generatedMessage)
			sentGroupMsg, err := bot.Send(groupMsg)
			if err != nil {
				editMsg := tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
					fmt.Sprintf("❌ Failed to send message to group: %v", err))
				bot.Send(editMsg)
				continue
			}

			// Step 5: Record the post
			err = recordNewsPost(topic, "", sentGroupMsg.MessageID)
			if err != nil {
				log.Printf("[NEWS] Error recording news post: %v", err)
				// Don't fail if database recording fails
			}

			// Send final success message
			editMsg = tgbotapi.NewEditMessageText(messg.Chat.ID, sentWorking.MessageID,
				fmt.Sprintf("✅ Message sent successfully!\n\n• Message ID: %d\n• Topic: %s", sentGroupMsg.MessageID, topic))
			bot.Send(editMsg)

			// Send a copy of what was sent
			resultMsg := tgbotapi.NewMessage(messg.Chat.ID, fmt.Sprintf("Message sent to group:\n\n%s", generatedMessage))
			bot.Send(resultMsg)

			continue
		}

		if messg.From.ID == bot.Self.ID ||
			update.EditedMessage != nil {
			continue
		}

		// Handle custom topic input for interactive news session
		if messg.Chat.IsPrivate() && text != "" && !strings.HasPrefix(text, "/") {
			if session, exists := interactiveSessions[messg.From.ID]; exists && session.Step == "topic_selection" {
				// User is providing a custom topic
				customTopic := strings.TrimSpace(text)
				if customTopic == "" {
					msg := tgbotapi.NewMessage(messg.Chat.ID, "❌ Topic cannot be empty. Please try again or use /inews to restart.")
					msg.ReplyToMessageID = messg.MessageID
					bot.Send(msg)
					continue
				}

				session.SelectedTopic = customTopic
				session.Step = "message_generation"

				// Acknowledge custom topic
				ackMsg := tgbotapi.NewMessage(messg.Chat.ID, fmt.Sprintf("✅ Custom topic received: \"%s\"\n\n🔄 Generating message...", customTopic))
				bot.Send(ackMsg)

				// Generate message
				generatedMessage, err := generateRandomMessage(customTopic)
				if err != nil {
					errorMsg := tgbotapi.NewMessage(messg.Chat.ID, fmt.Sprintf("❌ Failed to generate message: %v", err))
					bot.Send(errorMsg)
					delete(interactiveSessions, messg.From.ID)
					continue
				}

				session.GeneratedMessage = generatedMessage

				// Show generated message with options
				messageText := fmt.Sprintf("✅ Generated message:\n\n%s\n\n━━━━━━━━━━━━━━━━\nWhat would you like to do?", generatedMessage)

				keyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("📤 Send to Channel", "inews:send"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("🔄 Regenerate", "inews:regenerate"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "inews:cancel"),
					),
				)

				msgWithKeyboard := tgbotapi.NewMessage(messg.Chat.ID, messageText)
				msgWithKeyboard.ReplyMarkup = keyboard
				bot.Send(msgWithKeyboard)

				log.Printf("[INEWS] Custom topic used by user %d: %s", messg.From.ID, customTopic)
				continue
			}
		}

		// Handle replies to bot messages (conversation threading)
		if client != nil && messg.ReplyToMessage != nil && messg.ReplyToMessage.From.ID == bot.Self.ID {
			// User is replying to a bot message - check if it's part of a conversation
			parentMessageID := messg.ReplyToMessage.MessageID

			if _, exists := conversationCache.GetMessage(parentMessageID); exists {
				// This is a conversation continuation
				log.Printf("[INFO] Conversation continuation detected: user %d replying to message %d",
					messg.From.ID, parentMessageID)

				// Get system prompt from conversation root
				systemPrompt := conversationCache.GetSystemPrompt(parentMessageID)
				if systemPrompt == "" {
					systemPrompt = "You are a helpful assistant. Provide clear and concise answers."
				}

				// Build conversation history (last 5 exchanges)
				history := conversationCache.BuildConversationHistory(parentMessageID, 5)

				// Get GPT answer with conversation context
				txt, err := getGPTAnswerWithHistory(text, systemPrompt, history)
				if err != nil {
					log.Printf("Conversation GPT error: %v", err)
					txt = "Sorry, I couldn't process your message."
				}

				// Send response
				msg := tgbotapi.NewMessage(messg.Chat.ID, txt)
				msg.ReplyToMessageID = messg.MessageID
				sentMsg, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				} else {
					// Store user message in conversation tree
					conversationCache.AddMessage(&ConversationNode{
						MessageID:    messg.MessageID,
						ParentID:     parentMessageID,
						ChatID:       messg.Chat.ID,
						UserID:       int(messg.From.ID),
						Text:         text,
						Role:         "user",
						SystemPrompt: systemPrompt,
						Timestamp:    time.Now(),
					})

					// Store bot response in conversation tree
					conversationCache.AddMessage(&ConversationNode{
						MessageID:    sentMsg.MessageID,
						ParentID:     messg.MessageID,
						ChatID:       messg.Chat.ID,
						UserID:       int(bot.Self.ID),
						Text:         txt,
						Role:         "assistant",
						SystemPrompt: systemPrompt,
						Timestamp:    time.Now(),
					})
				}
				continue
			}
		}

		// Handle GPT questions when bot is mentioned (but not "read:" or "image:" prefix)
		if client != nil && strings.Contains(strings.ToUpper(text), strings.ToUpper(name)) {
			upperText := strings.ToUpper(text)
			// Skip if this is a "read:" or "image:" request
			isRead := strings.HasPrefix(upperText, "READ:") || strings.Contains(upperText, " READ:")
			isImage := strings.HasPrefix(upperText, "IMAGE:") || strings.Contains(upperText, " IMAGE:")
			if !isRead && !isImage {
				// Extract question and remove bot mention
				question := regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(text, "")
				question = strings.TrimSpace(question)

				if question != "" {
					log.Printf("GPT request: %s", question)
					systemPrompt := os.Getenv("GPT_SYSTEM_PROMPT")
					if systemPrompt == "" {
						systemPrompt = "You are a helpful assistant. Provide clear and concise answers."
					}

					// Check if we should enable memory tool for this chat/user
					var txt string
					var err error
					if shouldEnableMemoryTool(messg.Chat.ID, messg.From.ID) {
						log.Printf("[INFO] Enabling memory search tool for chat %d, user %d", messg.Chat.ID, messg.From.ID)
						tools := []openai.Tool{getSearchChatHistoryTool()}
						txt, err = getGPTAnswerWithSystemAndTools(question, systemPrompt, tools)
					} else {
						txt, err = getGPTAnswerWithSystem(question, systemPrompt)
					}
					if err != nil {
						log.Printf("GPT error: %v", err)
						txt = "Sorry, I couldn't process your question."
					}

					msg := tgbotapi.NewMessage(messg.Chat.ID, txt)
					msg.ReplyToMessageID = messg.MessageID
					sentMsg, err := bot.Send(msg)
					if err != nil {
						log.Printf("Send: %v ", err)
					} else {
						// Store initial question and answer in conversation tree
						conversationCache.AddMessage(&ConversationNode{
							MessageID:    messg.MessageID,
							ParentID:     0, // Root message
							ChatID:       messg.Chat.ID,
							UserID:       int(messg.From.ID),
							Text:         question,
							Role:         "user",
							SystemPrompt: systemPrompt,
							Timestamp:    time.Now(),
						})

						conversationCache.AddMessage(&ConversationNode{
							MessageID:    sentMsg.MessageID,
							ParentID:     messg.MessageID,
							ChatID:       messg.Chat.ID,
							UserID:       int(bot.Self.ID),
							Text:         txt,
							Role:         "assistant",
							SystemPrompt: systemPrompt,
							Timestamp:    time.Now(),
						})
					}
					continue
				}
			}
		}

		// Handle "image:" prefix for image generation (only if Gemini is enabled)
		upperText := strings.ToUpper(text)
		if geminiClient != nil && (strings.HasPrefix(upperText, "IMAGE:") || strings.Contains(upperText, " IMAGE:")) {
			// Extract prompt after "image:"
			imageIdx := strings.Index(upperText, "IMAGE:")
			if imageIdx != -1 {
				prompt := strings.TrimSpace(text[imageIdx+6:]) // Skip "image:"
				// Remove bot mention if present
				prompt = regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(prompt, "")
				prompt = strings.TrimSpace(prompt)

				if prompt != "" {
					log.Printf("[%s] Image generation request: %s\n", messg.From.UserName, prompt)

					imageData, err := generateImage(prompt)
					if err != nil {
						log.Printf("[ERROR] Image generation failed: %v", err)
						errorMsg := tgbotapi.NewMessage(messg.Chat.ID, "Sorry, I couldn't generate the image. "+err.Error())
						errorMsg.ReplyToMessageID = messg.MessageID
						bot.Send(errorMsg)
					} else {
						photoMsg := tgbotapi.NewPhoto(messg.Chat.ID, tgbotapi.FileBytes{
							Name:  "generated_image.png",
							Bytes: imageData.Bytes(),
						})
						photoMsg.ReplyToMessageID = messg.MessageID
						photoMsg.Caption = "Generated: " + prompt
						_, err = bot.Send(photoMsg)
						if err != nil {
							log.Printf("[ERROR] Failed to send image: %v", err)
						}
					}
				}
			}
			continue
		}

		// Handle "read:" prefix for text-to-speech
		if strings.HasPrefix(upperText, "READ:") || strings.Contains(upperText, " READ:") {
			// Extract text after "read:"
			readIdx := strings.Index(upperText, "READ:")
			if readIdx != -1 {
				textToRead := strings.TrimSpace(text[readIdx+5:]) // Skip "read:"
				// Remove bot mention if present
				textToRead = regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(textToRead, "")
				textToRead = strings.TrimSpace(textToRead)

				if textToRead != "" {
					log.Printf("[%s] TTS request: %s\n", messg.From.UserName, textToRead)

					res := makeSpeech(textToRead)
					if res != nil {
						file := tgbotapi.FileReader{
							Name:   "filename",
							Reader: res,
						}
						msg := tgbotapi.NewVoice(messg.Chat.ID, file)
						msg.ReplyToMessageID = messg.MessageID
						_, err = bot.Send(msg)

						if err != nil {
							log.Printf("Send: %v ", err)
						}
					}
				}
			}
			continue
		}
	}
}

// startInteractiveNewsSession starts an interactive news generation session
func startInteractiveNewsSession(bot *tgbotapi.BotAPI, userID int64, chatID int64) {
	// Send initial message
	msg := tgbotapi.NewMessage(chatID, "🔄 Starting interactive news session...\n\nGetting 3 random messages from last 20 hours...")
	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("[INEWS] Failed to send initial message: %v", err)
		return
	}

	// Get 3 random messages and extract topics
	var topicCandidates []string
	var sourceMessages []string

	for i := 0; i < 3; i++ {
		randomMessage, err := getRandomRecentMessage()
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID,
				fmt.Sprintf("❌ Failed to get random message: %v", err))
			bot.Send(editMsg)
			return
		}

		topic, err := extractTopicFromMessage(randomMessage)
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID,
				fmt.Sprintf("❌ Failed to extract topic from message %d: %v", i+1, err))
			bot.Send(editMsg)
			return
		}

		topicCandidates = append(topicCandidates, topic)
		sourceMessages = append(sourceMessages, randomMessage)

		// Update progress
		editMsg := tgbotapi.NewEditMessageText(chatID, sentMsg.MessageID,
			fmt.Sprintf("🔄 Processing... (%d/3 topics extracted)", i+1))
		bot.Send(editMsg)
	}

	// Create session
	session := &InteractiveNewsSession{
		Step:            "topic_selection",
		TopicCandidates: topicCandidates,
		SourceMessages:  sourceMessages,
		MessageID:       sentMsg.MessageID,
	}
	interactiveSessions[userID] = session

	// Show topic selection with buttons
	messageText := "✅ Found 3 topic candidates!\n\nPlease select a topic:\n\n"
	for i, topic := range topicCandidates {
		messageText += fmt.Sprintf("%d. %s\n", i+1, topic)
	}
	messageText += "\nOr send me your own custom topic as a text message."

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1️⃣ "+topicCandidates[0], "inews:topic:0"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("2️⃣ "+topicCandidates[1], "inews:topic:1"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("3️⃣ "+topicCandidates[2], "inews:topic:2"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "inews:cancel"),
		),
	)

	editMsg := tgbotapi.NewEditMessageTextAndMarkup(chatID, sentMsg.MessageID, messageText, keyboard)
	bot.Send(editMsg)

	log.Printf("[INEWS] Started interactive session for user %d with topics: %v", userID, topicCandidates)
}

// handleInteractiveNewsCallback handles button clicks in interactive news session
func handleInteractiveNewsCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	// Answer callback query immediately to remove loading state
	callback := tgbotapi.NewCallback(query.ID, "")
	bot.Request(callback)

	userID := query.From.ID
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID

	// Check if there's an active session
	session, exists := interactiveSessions[userID]
	if !exists {
		msg := tgbotapi.NewMessage(chatID, "❌ No active session. Use /inews to start a new one.")
		bot.Send(msg)
		return
	}

	data := query.Data

	// Handle cancel
	if data == "inews:cancel" {
		delete(interactiveSessions, userID)
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "❌ Interactive session cancelled.")
		bot.Send(editMsg)
		log.Printf("[INEWS] Session cancelled by user %d", userID)
		return
	}

	// Handle topic selection
	if strings.HasPrefix(data, "inews:topic:") {
		if session.Step != "topic_selection" {
			msg := tgbotapi.NewMessage(chatID, "❌ Invalid session state.")
			bot.Send(msg)
			return
		}

		// Parse topic index
		indexStr := strings.TrimPrefix(data, "inews:topic:")
		index, err := strconv.Atoi(indexStr)
		if err != nil || index < 0 || index >= len(session.TopicCandidates) {
			msg := tgbotapi.NewMessage(chatID, "❌ Invalid topic selection.")
			bot.Send(msg)
			return
		}

		selectedTopic := session.TopicCandidates[index]
		session.SelectedTopic = selectedTopic
		session.Step = "message_generation"

		// Update message to show selection
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID,
			fmt.Sprintf("✅ Selected topic: \"%s\"\n\n🔄 Generating message...", selectedTopic))
		bot.Send(editMsg)

		// Generate message
		generatedMessage, err := generateRandomMessage(selectedTopic)
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID,
				fmt.Sprintf("❌ Failed to generate message: %v", err))
			bot.Send(editMsg)
			delete(interactiveSessions, userID)
			return
		}

		session.GeneratedMessage = generatedMessage

		// Show generated message with options
		messageText := fmt.Sprintf("✅ Generated message:\n\n%s\n\n━━━━━━━━━━━━━━━━\nWhat would you like to do?", generatedMessage)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📤 Send to Channel", "inews:send"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Regenerate", "inews:regenerate"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "inews:cancel"),
			),
		)

		editMsg = tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, messageText, keyboard)
		bot.Send(editMsg)

		log.Printf("[INEWS] Generated message for user %d with topic: %s", userID, selectedTopic)
		return
	}

	// Handle regenerate
	if data == "inews:regenerate" {
		if session.Step != "message_generation" || session.SelectedTopic == "" {
			msg := tgbotapi.NewMessage(chatID, "❌ Invalid session state.")
			bot.Send(msg)
			return
		}

		// Update message to show regeneration
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID,
			fmt.Sprintf("🔄 Regenerating message for topic: \"%s\"...", session.SelectedTopic))
		bot.Send(editMsg)

		// Regenerate message
		generatedMessage, err := generateRandomMessage(session.SelectedTopic)
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID,
				fmt.Sprintf("❌ Failed to regenerate message: %v", err))
			bot.Send(editMsg)
			return
		}

		session.GeneratedMessage = generatedMessage

		// Show regenerated message with options
		messageText := fmt.Sprintf("✅ Regenerated message:\n\n%s\n\n━━━━━━━━━━━━━━━━\nWhat would you like to do?", generatedMessage)

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📤 Send to Channel", "inews:send"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Regenerate", "inews:regenerate"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "inews:cancel"),
			),
		)

		editMsg = tgbotapi.NewEditMessageTextAndMarkup(chatID, messageID, messageText, keyboard)
		bot.Send(editMsg)

		log.Printf("[INEWS] Regenerated message for user %d", userID)
		return
	}

	// Handle send to channel
	if data == "inews:send" {
		if session.Step != "message_generation" || session.GeneratedMessage == "" {
			msg := tgbotapi.NewMessage(chatID, "❌ Invalid session state.")
			bot.Send(msg)
			return
		}

		// Get memory group ID
		memoryGroupID, err := getEnvInt64("MEMORY_GROUP_ID")
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID,
				fmt.Sprintf("❌ Failed to get MEMORY_GROUP_ID: %v", err))
			bot.Send(editMsg)
			return
		}

		// Update message to show sending
		editMsg := tgbotapi.NewEditMessageText(chatID, messageID,
			"📤 Sending message to channel...")
		bot.Send(editMsg)

		// Send to group
		groupMsg := tgbotapi.NewMessage(memoryGroupID, session.GeneratedMessage)
		sentGroupMsg, err := bot.Send(groupMsg)
		if err != nil {
			editMsg := tgbotapi.NewEditMessageText(chatID, messageID,
				fmt.Sprintf("❌ Failed to send message to group: %v", err))
			bot.Send(editMsg)
			return
		}

		// Record the post
		err = recordNewsPost(session.SelectedTopic, "", sentGroupMsg.MessageID)
		if err != nil {
			log.Printf("[INEWS] Error recording news post: %v", err)
		}

		// Send success message
		editMsg = tgbotapi.NewEditMessageText(chatID, messageID,
			fmt.Sprintf("✅ Message sent successfully!\n\n• Message ID: %d\n• Topic: %s\n\nMessage:\n%s",
				sentGroupMsg.MessageID, session.SelectedTopic, session.GeneratedMessage))
		bot.Send(editMsg)

		// Clean up session
		delete(interactiveSessions, userID)
		log.Printf("[INEWS] Message sent to group by user %d (message ID: %d)", userID, sentGroupMsg.MessageID)
		return
	}
}

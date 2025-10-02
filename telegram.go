package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type class struct {
	Topic     string
	Date      time.Time
	MessageID int
}

var currentClass class

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

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Printf("[ERROR] getting update channel %v\n", err)
	}

	for update := range updates {
		if update.Message == nil && update.EditedMessage == nil && update.CallbackQuery == nil {
			continue
		}
		var text string
		var messg *tgbotapi.Message

		if update.Message != nil {
			messg = update.Message
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

			// Get class group ID
			classGroupID, err := getClassGroupID()
			if err == nil && int64(messg.Chat.ID) == classGroupID {
				// Check if message contains a suggestion
				suggestedURL := ExtractSuggestionFromMessage(text)
				if suggestedURL != "" {
					// Add to database
					_, err := AddMediaSuggestion(suggestedURL, messg.From.UserName)
					if err != nil {
						log.Printf("Error adding media suggestion: %v", err)
					} else {
						// Acknowledge the suggestion
						reply := fmt.Sprintf("Thanks for your suggestion, @%s! I've added it to our collection.",
							messg.From.UserName)
						msg := tgbotapi.NewMessage(messg.Chat.ID, reply)
						msg.ReplyToMessageID = messg.MessageID
						_, err = bot.Send(msg)
						if err != nil {
							log.Printf("Error sending suggestion acknowledgment: %v", err)
						}
					}
				}
			}
		}

		if strings.HasPrefix(strings.ToUpper(text), "/HELP") {
			answer := `Commands:
/idiom <term> - Show the definition from idioms.thefreedictionary.com
/media or /list - Show current list of media suggestions
/del <number> - Delete a media suggestion
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

		if messg.From.ID == bot.Self.ID ||
			update.EditedMessage != nil {
			continue
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
						UserID:       messg.From.ID,
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
						UserID:       bot.Self.ID,
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

					txt, err := getGPTAnswerWithSystem(question, systemPrompt)
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
							UserID:       messg.From.ID,
							Text:         question,
							Role:         "user",
							SystemPrompt: systemPrompt,
							Timestamp:    time.Now(),
						})

						conversationCache.AddMessage(&ConversationNode{
							MessageID:    sentMsg.MessageID,
							ParentID:     messg.MessageID,
							ChatID:       messg.Chat.ID,
							UserID:       bot.Self.ID,
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

		// Handle media commands
		if strings.HasPrefix(strings.ToUpper(text), "/MEDIA") || strings.HasPrefix(strings.ToUpper(text), "/LIST") {
			showCurrentMedia(bot, messg)
			continue
		}

		// Handle delete commands
		if strings.HasPrefix(strings.ToUpper(text), "/DEL ") {
			handleDeleteSuggestion(bot, messg)
			continue
		}

		// Handle selection by owner (if message is just a number)
		if handled := handleMediaSelection(bot, messg); handled {
			continue
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
						photoMsg := tgbotapi.NewPhotoUpload(messg.Chat.ID, tgbotapi.FileBytes{
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
							Size:   -1,
						}
						msg := tgbotapi.NewVoiceUpload(messg.Chat.ID, file)
						msg.ReplyToMessageID = messg.MessageID
						_, err = bot.Send(msg)

						if err != nil {
							log.Printf("Send: %v ", err)
						}
					}
				}
			}
		}
	}
}

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

func botGo(filter filterFunc) {
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

	fsms := make(map[int]*dialogue)

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Printf("[ERROR] getting update channel %v\n", err)
	}

	for update := range updates {
		if update.Message == nil && update.EditedMessage == nil && update.CallbackQuery == nil {
			continue
		}
		var text string
		var from int
		var messg *tgbotapi.Message

		if update.Message != nil {
			messg = update.Message
		}
		if update.EditedMessage != nil {
			messg = update.EditedMessage
		}

		text = messg.Text
		from = messg.From.ID

		if messg.Chat.IsGroup() || messg.Chat.IsSuperGroup() {
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

		if _, ok := fsms[from]; !ok {
			fsms[from] = newDialogue(from)
		}
		fsm := fsms[from]

		if strings.HasPrefix(strings.ToUpper(text), "/CANCEL") {
			_ = fsm.state.Event("cancel")

			msg := tgbotapi.NewMessage(messg.Chat.ID, "Command canceled.")
			msg.ReplyToMessageID = messg.MessageID

			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}

		switch fsm.state.Current() {
		case "waitidiom":
			split := strings.Split(text, " ")
			answer := getIdiom(strings.Join(split, "+"))
			if answer == "" {
				answer = "Sorry, nothing about " + text
			}
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			_ = fsm.state.Event("cancel")
			continue

		case "idle":
			log.Printf("[INFO] idle state for %s is  %s  \n", messg.From.UserName, fsm.state.Current())

		default:
			log.Printf("[INFO] uncovered state for %s is  %s  \n", messg.From.UserName, fsm.state.Current())
		}

		if strings.HasPrefix(strings.ToUpper(text), "/HELP") {
			answer := `You can send me any text to read aloud, but please mention me by @` + name +
				` /oxford term :  show the definition from Oxford dictionary
			   /idiom term  :  show the definition from idioms.thefreedictionary.com`

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			continue
		}
		if strings.HasPrefix(strings.ToUpper(text), "/IDIOM") {
			split := strings.Split(text, " ")
			answer := ""
			if len(split) < 2 {
				answer = " Please send me any text to search in idioms.thefreedictionary.com"
				_ = fsm.state.Event("waitidiom")
			} else {
				answer = getIdiom(strings.Join(split[1:], "+"))
				if answer == "" {
					answer = "Sorry, nothing about " + strings.Join(split[1:], " ")
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

		// Handle "answer:" mode - bot mentioned + message starts with "answer:"
		if client != nil && strings.Contains(strings.ToUpper(text), strings.ToUpper(name)) {
			upperText := strings.ToUpper(text)
			if strings.HasPrefix(upperText, "ANSWER:") || strings.Contains(upperText, " ANSWER:") {
				// Extract the question after "answer:"
				answerIdx := strings.Index(upperText, "ANSWER:")
				if answerIdx != -1 {
					question := strings.TrimSpace(text[answerIdx+7:]) // Skip "answer:"
					// Remove bot mention from question
					question = regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(question, "")
					question = strings.TrimSpace(question)

					if question != "" {
						log.Printf("Answer mode GPT request: %s", question)
						systemPrompt := os.Getenv("GPT_SYSTEM_PROMPT")
						if systemPrompt == "" {
							systemPrompt = "You are a helpful assistant. Provide clear and concise answers."
						}

						txt, err := getGPTAnswerWithSystem(question, systemPrompt)
						if err != nil {
							log.Printf("Answer mode GPT error: %v", err)
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
		}

		if client != nil {
			if filter != nil &&
				filter(strings.ToUpper(text)) { // || messg.ReplyToMessage != nil &&
				// messg.ReplyToMessage.From.ID == bot.Self.ID) {
				log.Printf("GPT request: %b %s", filter(strings.ToUpper(text)), text)
				txt, err := getGPTAnswer(text)
				if err != nil {
					log.Printf("chat: %v ", err)
				}
				msg := tgbotapi.NewMessage(messg.Chat.ID, txt)
				msg.ReplyToMessageID = messg.MessageID
				_, err = bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
				continue
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

		if !strings.Contains(strings.ToUpper(text), strings.ToUpper(name)) {
			continue
		}

		log.Printf("[%s] %s \n", messg.From.UserName, text)

		text = regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(text, "")

		// sendAudio(text, voice, messg.From.ID, messg.Chat.ID, messg.MessageID)
		res := makeSpeech(text)
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

type filterFunc func(string) bool

func generatorOfContainFuncs(keywords []string) filterFunc {
	if len(keywords) == 1 {
		return func(msg string) bool {
			return false
		}
	}
	return func(msg string) bool {
		for _, keyword := range keywords {
			if strings.Contains(strings.ToUpper(msg), strings.ToUpper(keyword)) {
				log.Printf("keyword: %s <- %s", keyword, msg)
				return true
			}
		}
		return false
	}
}

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

var removeVoice = regexp.MustCompile(`\[(\w+)\]`)

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
		case "waitvoice":
			answer := ""
			if !voices[text] {
				answer = "Sorry,I don't have this voice: '" + text + "', command canceled"
				_ = fsm.state.Event("cancel")
				msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
				msg.ReplyToMessageID = messg.MessageID

				msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true, Selective: true}

				_, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
			} else {
				answer = "Okay, Dear " + messg.From.FirstName + ", I will use the voice " + text + " for your messages! \n Wheneve you want me to make speech just mention me in your message"

				res := makeSpeech(answer, text)
				if res != nil {
					file := tgbotapi.FileReader{
						Name:   "filename",
						Reader: res,
						Size:   -1,
					}
					msg := tgbotapi.NewVoiceUpload(messg.Chat.ID, file)
					msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true, Selective: true}
					msg.ReplyToMessageID = messg.MessageID
					_, err = bot.Send(msg)

					if err != nil {
						log.Printf("Send: %v ", err)
					}
				}

				SetVoice(messg.From.ID, text)
				prefs[messg.From.ID] = text

				_ = fsm.state.Event("cancel")
			}
			continue
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

		if strings.HasPrefix(strings.ToUpper(text), "/SETVOICE") {
			_ = fsm.state.Event("waitvoice")

			var buttons []tgbotapi.KeyboardButton

			for k := range voices {
				buttons = append(buttons, tgbotapi.KeyboardButton{Text: k})
			}
			answer := "Please choose a voice from the list and send to me."
			markup := tgbotapi.ReplyKeyboardMarkup{
				Keyboard: [][]tgbotapi.KeyboardButton{
					buttons,
				},
				OneTimeKeyboard: true,
				ResizeKeyboard:  true,
				Selective:       true,
			}

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			msg.ReplyMarkup = markup
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
		text = removeVoice.ReplaceAllLiteralString(text, "")

		voice := "Raveena"
		if v, ok := prefs[messg.From.ID]; ok {
			voice = v
		}

		// sendAudio(text, voice, messg.From.ID, messg.Chat.ID, messg.MessageID)
		res := makeSpeech(text, voice)
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

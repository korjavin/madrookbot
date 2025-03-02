package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type class struct {
	Topic     string
	Date      time.Time
	MessageID int
}

var currentClass class

var removeVoice = regexp.MustCompile(`\[(\w+)\]`)

func botGo(filter filterFunc) {
	ctx := context.Background()
	b, err := bot.New(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		log.Panicf("me: %#v \n", err)
	}
	name := me.Username

	log.Printf("Authorized on account %s", me.Username)

	fsms := make(map[int]*dialogue)

	b.Start(ctx)

	handler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil && update.EditedMessage == nil && update.CallbackQuery == nil {
			return
		}
		var text string
		var from int
		var messg *models.Message

		if update.Message != nil {
			messg = update.Message
		}
		if update.EditedMessage != nil {
			messg = update.EditedMessage
		}

		text = messg.Text
		from = int(messg.From.ID)

		if messg.Chat.Type == "group" || messg.Chat.Type == "supergroup" {
			classGroupID, err := getClassGroupID()
			if err == nil && int64(messg.Chat.ID) == classGroupID {
				suggestedURL := ExtractSuggestionFromMessage(text)
				if suggestedURL != "" {
					_, err := AddMediaSuggestion(suggestedURL, messg.From.Username)
					if err != nil {
						log.Printf("Error adding media suggestion: %v", err)
					} else {
						reply := fmt.Sprintf("Thanks for your suggestion, @%s! I've added it to our collection.",
							messg.From.Username)
						params := &bot.SendMessageParams{
							ChatID:           messg.Chat.ID,
							Text:             reply,
							ReplyToMessageID: messg.ID,
						}
						_, err = b.SendMessage(ctx, params)
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

			params := &bot.SendMessageParams{
				ChatID:           messg.Chat.ID,
				Text:             "Command canceled.",
				ReplyToMessageID: messg.ID,
			}
			_, err := b.SendMessage(ctx, params)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			return
		}

		switch fsm.state.Current() {
		case "waitvoice":
			answer := ""
			if !voices[text] {
				answer = "Sorry,I don't have this voice: '" + text + "', command canceled"
				_ = fsm.state.Event("cancel")
				params := &bot.SendMessageParams{
					ChatID:           messg.Chat.ID,
					Text:             answer,
					ReplyToMessageID: messg.ID,
					ReplyMarkup: &models.ReplyKeyboardRemove{
						RemoveKeyboard: true,
						Selective:      true,
					},
				}
				_, err := b.SendMessage(ctx, params)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
			} else {
				answer = "Okay, Dear " + messg.From.FirstName + ", I will use the voice " + text + " for your messages! \n Wheneve you want me to make speech just mention me in your message"

				res := makeSpeech(answer, text)
				if res != nil {
					file := &models.InputFileUpload{
						Filename: "voice.mp3",
						Data:     res,
					}
					params := &bot.SendVoiceParams{
						ChatID:           messg.Chat.ID,
						Voice:            file,
						ReplyToMessageID: messg.ID,
						ReplyMarkup: &models.ReplyKeyboardRemove{
							RemoveKeyboard: true,
							Selective:      true,
						},
					}
					_, err = b.SendVoice(ctx, params)

					if err != nil {
						log.Printf("Send: %v ", err)
					}
				}

				SetVoice(int(messg.From.ID), text)
				prefs[int(messg.From.ID)] = text

				_ = fsm.state.Event("cancel")
			}
			return
		case "waitidiom":
			split := strings.Split(text, " ")
			answer := getIdiom(strings.Join(split, "+"))
			if answer == "" {
				answer = "Sorry, nothing about " + text
			}
			params := &bot.SendMessageParams{
				ChatID:           messg.Chat.ID,
				Text:             answer,
				ReplyToMessageID: messg.ID,
			}
			_, err := b.SendMessage(ctx, params)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			_ = fsm.state.Event("cancel")
			return

		case "idle":
			log.Printf("[INFO] idle state for %s is  %s  \n", messg.From.Username, fsm.state.Current())

		default:
			log.Printf("[INFO] uncovered state for %s is  %s  \n", messg.From.Username, fsm.state.Current())
		}

		if strings.HasPrefix(strings.ToUpper(text), "/HELP") {
			answer := `You can send me any text to read aloud, but please mention me by @` + name +
				` /oxford term :  show the definition from Oxford dictionary
			   /idiom term  :  show the definition from idioms.thefreedictionary.com`

			params := &bot.SendMessageParams{
				ChatID:           messg.Chat.ID,
				Text:             answer,
				ReplyToMessageID: messg.ID,
			}
			_, err := b.SendMessage(ctx, params)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			return
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

			params := &bot.SendMessageParams{
				ChatID:           messg.Chat.ID,
				Text:             answer,
				ReplyToMessageID: messg.ID,
			}
			_, err := b.SendMessage(ctx, params)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			return
		}

		if strings.HasPrefix(strings.ToUpper(text), "/SETVOICE") {
			_ = fsm.state.Event("waitvoice")

			var buttons []models.KeyboardButton

			for k := range voices {
				buttons = append(buttons, models.KeyboardButton{Text: k})
			}
			answer := "Please choose a voice from the list and send to me."
			markup := &models.ReplyKeyboardMarkup{
				Keyboard: [][]models.KeyboardButton{
					buttons,
				},
				OneTimeKeyboard: true,
				ResizeKeyboard:  true,
				Selective:       true,
			}

			params := &bot.SendMessageParams{
				ChatID:           messg.Chat.ID,
				Text:             answer,
				ReplyToMessageID: messg.ID,
				ReplyMarkup:      markup,
			}
			_, err := b.SendMessage(ctx, params)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
			return
		}

		if update.Message != nil && update.Message.From.ID == me.ID ||
			update.EditedMessage != nil {
			return
		}
		if client != nil {
			if filter != nil &&
				filter(strings.ToUpper(text)) {
				log.Printf("GPT request: %b %s", filter(strings.ToUpper(text)), text)
				txt, err := getGPTAnswer(text)
				if err != nil {
					log.Printf("chat: %v ", err)
				}
				params := &bot.SendMessageParams{
					ChatID:           messg.Chat.ID,
					Text:             txt,
					ReplyToMessageID: messg.ID,
				}
				_, err = b.SendMessage(ctx, params)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
				return
			}
		}
		if strings.HasPrefix(strings.ToUpper(text), "/MEDIA") || strings.HasPrefix(strings.ToUpper(text), "/LIST") {
			showCurrentMedia(b, ctx, messg)
			return
		}

		if strings.HasPrefix(strings.ToUpper(text), "/DEL ") {
			handleDeleteSuggestion(b, ctx, messg)
			return
		}

		if handled := handleMediaSelection(b, ctx, messg); handled {
			return
		}

		if !strings.Contains(strings.ToUpper(text), strings.ToUpper(name)) {
			return
		}

		log.Printf("[%s] %s \n", messg.From.Username, text)

		text = regexp.MustCompile(`(?i)@`+name).ReplaceAllLiteralString(text, "")
		text = removeVoice.ReplaceAllLiteralString(text, "")

		voice := "Raveena"
		if v, ok := prefs[int(messg.From.ID)]; ok {
			voice = v
		}

		res := makeSpeech(text, voice)
		if res != nil {
			file := &models.InputFileUpload{
				Filename: "voice.mp3",
				Data:     res,
			}
			params := &bot.SendVoiceParams{
				ChatID:           messg.Chat.ID,
				Voice:            file,
				ReplyToMessageID: messg.ID,
			}
			_, err = b.SendVoice(ctx, params)

			if err != nil {
				log.Printf("Send: %v ", err)
			}
		}
	}

	// Register handler with proper types
	b.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, bot.HandlerFunc(handler))

	// Block until context is cancelled
	<-ctx.Done()
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

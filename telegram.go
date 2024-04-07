package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/olekukonko/tablewriter"
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
	sg := make(map[int]*SheetGenerator)

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

		go AddActivity(messg.From.UserName, messg.Chat.Title, time.Now().Unix())

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

		if strings.HasPrefix(strings.ToUpper(text), "/CENSURE") {
			if messg.From.FirstName != "engelbart" {
				log.Printf("User %s tried to use admin command", messg.From.UserName)
			}
			if messg.Chat.IsGroup() {
				name, ts := GetSilentMoreThan14Days(messg.Chat.Title)
				log.Printf("Silent %s for 14 days, since %s", name, time.Unix(ts, 0))
			} else {
				groupname := strings.Split(messg.Text, " ")[1]
				name, ts := GetSilentMoreThan14Days(groupname)
				log.Printf("Silent %s for 14 days, since %s", name, time.Unix(ts, 0))
			}
		}

		if strings.HasPrefix(strings.ToUpper(text), "/NEWBINGO") {
			var words []string
			split := strings.Split(text, " ")
			if len(split) > 1 {
				words = append(words, split[1:]...)
			}

			sg[messg.From.ID] = NewSheetGenerator(words, time.Now().Unix())
			answer := fmt.Sprintf("New bingo generator created. \n There are %d words in it\n You can add new with /ADD word command", len(words))
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}
		}
		if strings.HasPrefix(strings.ToUpper(text), "/ADD") {
			var answer string
			if _, ok := sg[messg.From.ID]; !ok {
				sg[messg.From.ID] = NewSheetGenerator([]string{}, time.Now().Unix())
			}
			var words []string
			split := strings.Split(text, " ")
			if len(split) > 1 {
				words = append(words, split[1:]...)
				for _, w := range words {
					sg[messg.From.ID].AddWord(w)
				}
				answer = fmt.Sprintf("%d words added\n Thank you. You can use /BINGO now", len(words))
			} else {
				answer = "Give me a word or two to add"
			}
			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}

		}
		if strings.HasPrefix(strings.ToUpper(text), "/BINGO") {
			var answer string
			if _, ok := sg[messg.From.ID]; !ok {
				answer = "I don't have words for you. Create own by /NEWBINGO"
			} else {
				split := strings.Split(text, " ")
				var size int
				if len(split) > 1 {
					size, _ = strconv.Atoi(split[1])
				}
				words := sg[messg.From.ID].GenerateOne(size)
				var captions []string
				for range words[0] {
					captions = append(captions, "-")
				}

				buf := bytes.NewBufferString("```")
				table := tablewriter.NewWriter(buf)
				table.SetHeader(captions)
				table.AppendBulk(words)
				table.Render()
				buf.WriteString("```")
				answer = buf.String()
			}

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			msg.ParseMode = "Markdown"
			// msg.ReplyToMessageID = messg.MessageID
			_, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}

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

		if strings.HasPrefix(strings.ToUpper(text), "/CREATE") {
			if messg.From.ID != 59701326 {
				answer := fmt.Sprintf("User %s is not allowed to create new class. Call the support!", messg.From.UserName)
				msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
				msg.ReplyToMessageID = messg.MessageID
				_, err := bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}

			}

			// parse date and topic of the class from message which format is /create 2019-01-01 20:00 topic
			split := strings.Split(text, " ")
			if len(split) < 4 {
				log.Printf("Wrong format of the message. Should be /create 2019-01-01 20:00 topic")
				continue
			}
			date := split[1]
			tm := split[2]
			topic := strings.Join(split[3:], " ")
			// parse date time in berlin datazone
			// time.LoadLocation undefined (type string has no field or method LoadLocation)
			berlin, err := time.LoadLocation("Europe/Berlin")
			if err != nil {
				log.Printf("LoadLocation: %v ", err)
				continue
			}
			// parse date time in berlin datazone
			t, err := time.ParseInLocation("2006-01-02 15:04", date+" "+tm, berlin)
			if err != nil {
				log.Printf("ParseInLocation: %v ", err)
				continue
			}

			// create new class
			currentClass = class{Date: t, Topic: topic}

			timeStr := t.UTC().Format("January 2 3 PM MST")

			answer, err := getGPTAnswerWithSystem(
				fmt.Sprintf("Rewrite message: new class is scheduled \n on  %s at %s.\n Topic: *%s* \n In order to join it put any reaction on this message and you will be reminded 10 minutes before the class with a zoom link. \n\n Please be committed, if you RSVP we do expect you join.", date, timeStr, topic),
				"You are an English Teacher, and you try to use advanced vocabulary",
			)
			if err != nil {
				log.Printf("GPT err: %v ", err)
			}

			msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
			m, err := bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
				continue
			}
			currentClass.MessageID = m.MessageID

			go func() {
				answer, err := getGPTAnswerWithSystem(
					fmt.Sprintf("Rewrite message: class with topic *%s* is starting in ten minutes, to join please use this link: https://us02web.zoom.us/j/7249000123?pwd=azdzRVJtR2lQMmxYU3lzU0R0dDZydz09 \n\n Please remember, we are a small group, everyone attendance is making difference", topic),
					"You are an English Teacher, and you try to use advanced vocabulary",
				)
				if err != nil {
					log.Printf("GPT err: %v ", err)
				}
				log.Printf("I am going to sleep for %s", time.Until(currentClass.Date.Add(-10*time.Minute)))
				// sleep until the class
				time.Sleep(time.Until(currentClass.Date.Add(-10 * time.Minute)))
				// send reminder
				msg := tgbotapi.NewMessage(messg.Chat.ID, answer)
				_, err = bot.Send(msg)
				if err != nil {
					log.Printf("Send: %v ", err)
				}
			}()

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

		if strings.Contains(strings.ToUpper(text), "ЧТО НОВОГО") {
			txt, err := getAndDeleteRandomNews()
			if err != nil {
				log.Printf("db: %v ", err)
				txt = "Все известные мне новости уже показаны"
			}
			msg := tgbotapi.NewMessage(messg.Chat.ID, txt)
			msg.ReplyToMessageID = messg.MessageID
			_, err = bot.Send(msg)
			if err != nil {
				log.Printf("Send: %v ", err)
			}

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

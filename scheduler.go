package main

import (
	"fmt"
	"log"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Constants for scheduler
const (
	// Berlin timezone
	berlinTimezone = "Europe/Berlin"

	// Times
	MondayTime    = "15:00" // Monday 15:00 Berlin - remind about collection
	WednesdayTime = "12:00" // Wednesday 12:00 Berlin - present list for selection
	SundayTime    = "17:00" // Sunday 17:00 Berlin - remind about upcoming call
)

// scheduleMediaTasks runs the scheduled tasks for media management
func scheduleMediaTasks() {
	// Wait for initial suggestions before starting scheduler
	log.Println("[INFO] Scheduler: Waiting for media suggestions before starting...")
	for {
		suggestions, err := GetMediaSuggestions()
		if err == nil && len(suggestions) > 0 {
			log.Printf("[INFO] Scheduler: Found %d suggestions, starting scheduler", len(suggestions))
			break
		}
		// Check every hour if there are suggestions
		time.Sleep(1 * time.Hour)
	}

	loc, err := time.LoadLocation(berlinTimezone)
	if err != nil {
		log.Printf("[ERROR] Error loading timezone: %v", err)
		loc = time.UTC
	}

	// Get the Telegram bot
	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Printf("[ERROR] Error initializing bot for scheduler: %v", err)
		return
	}

	// Get group ID
	groupID, err := getClassGroupID()
	if err != nil {
		log.Printf("[ERROR] Scheduler error: %v", err)
		return
	}

	// Track what we've already sent today to avoid duplicates
	var lastMondayRun, lastWednesdayRun, lastSundayRun time.Time

	for {
		now := time.Now().In(loc)

		// Sleep for 5 minutes between checks (more reliable than 1 minute)
		time.Sleep(5 * time.Minute)

		// Get current time after sleep
		now = time.Now().In(loc)
		weekday := now.Weekday()
		currentHour := now.Hour()
		currentMinute := now.Minute()

		// Check for scheduled events with time windows (not exact match)
		// Monday: 15:00-15:10 window
		if weekday == time.Monday && currentHour == 15 && currentMinute < 10 {
			// Check if we already ran today
			if lastMondayRun.YearDay() != now.YearDay() || lastMondayRun.Year() != now.Year() {
				log.Println("[INFO] Triggering Monday reminder")
				go sendMondayReminder(bot, groupID)
				lastMondayRun = now
			}
		}

		// Wednesday: 12:00-12:10 window
		if weekday == time.Wednesday && currentHour == 12 && currentMinute < 10 {
			if lastWednesdayRun.YearDay() != now.YearDay() || lastWednesdayRun.Year() != now.Year() {
				log.Println("[INFO] Triggering Wednesday list presentation")
				go sendWednesdayList(bot, groupID)
				lastWednesdayRun = now
			}
		}

		// Sunday: 17:00-17:10 window
		if weekday == time.Sunday && currentHour == 17 && currentMinute < 10 {
			if lastSundayRun.YearDay() != now.YearDay() || lastSundayRun.Year() != now.Year() {
				log.Println("[INFO] Triggering Sunday reminder")
				go sendSundayReminder(bot, groupID)
				lastSundayRun = now
			}
		}
	}
}

// sendMondayReminder sends the Monday reminder with the current list
func sendMondayReminder(bot *tgbotapi.BotAPI, groupID int64) {
	log.Printf("Sending Monday reminder")

	suggestions, err := GetMediaSuggestions()
	if err != nil {
		log.Printf("Error getting media suggestions for Monday reminder: %v", err)
		return
	}

	message := "🗓 *Weekly Media Collection Started*\n\n"
	message += "Help shape our next discussion by suggesting articles, videos, or podcasts! Just share a link and mention it's a suggestion.\n\n"
	message += FormatMediaList(suggestions)

	msg := tgbotapi.NewMessage(groupID, message)
	//msg.ParseMode = "Markdown"

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending Monday reminder: %v", err)
	}
}

// sendWednesdayList sends the Wednesday list for selection
func sendWednesdayList(bot *tgbotapi.BotAPI, groupID int64) {
	log.Printf("Sending Wednesday list")

	// Process weekly update first
	err := ProcessWeeklyUpdate()
	if err != nil {
		log.Printf("Error processing weekly update: %v", err)
	}

	suggestions, err := GetMediaSuggestions()
	if err != nil {
		log.Printf("Error getting media suggestions for Wednesday list: %v", err)
		return
	}

	if len(suggestions) == 0 {
		message := "⚠️ *No Media Suggestions Available*\n\n"
		message += "There are no media suggestions for this week. Please suggest some content by sharing a link!"

		msg := tgbotapi.NewMessage(groupID, message)
		msg.ParseMode = "Markdown"

		_, err = bot.Send(msg)
		if err != nil {
			log.Printf("Error sending Wednesday empty list: %v", err)
		}
		return
	}

	message := "🎯 *Media Selection Time*\n\n"
	message += "Here are this week's suggestions. The owner will select one by replying with the number.\n\n"
	message += FormatMediaList(suggestions)

	msg := tgbotapi.NewMessage(groupID, message)
	//msg.ParseMode = "Markdown"

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending Wednesday list: %v", err)
	}
}

// sendSundayReminder sends the Sunday reminder about the upcoming call
func sendSundayReminder(bot *tgbotapi.BotAPI, groupID int64) {
	log.Printf("Sending Sunday reminder")

	selected, err := GetSelectedMedia()
	if err != nil {
		log.Printf("Error getting selected media for Sunday reminder: %v", err)
		return
	}

	if selected.ID == 0 {
		message := "⚠️ *Reminder: Discussion in 1 Hour*\n\n"
		message += "Our weekly call starts in 1 hour, but no media was selected for discussion this week."

		msg := tgbotapi.NewMessage(groupID, message)
		msg.ParseMode = "Markdown"

		_, err = bot.Send(msg)
		if err != nil {
			log.Printf("Error sending Sunday reminder: %v", err)
		}
		return
	}

	message := "⏰ *Reminder: Discussion in 1 Hour*\n\n"
	message += fmt.Sprintf("Our weekly call starts in 1 hour. We'll be discussing:\n\n[%s](%s)\n\nSuggested by: @%s",
		truncateURL(selected.URL),
		selected.URL,
		selected.Suggester)

	msg := tgbotapi.NewMessage(groupID, message)
	//msg.ParseMode = "Markdown"

	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending Sunday reminder: %v", err)
	}
}

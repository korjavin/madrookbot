package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
	
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Constants for scheduler
const (
	// Berlin timezone
	berlinTimezone = "Europe/Berlin"
	
	// Weekdays
	Monday = 1
	Wednesday = 3
	Sunday = 0
	
	// Times
	MondayTime = "15:00"    // Monday 15:00 Berlin - remind about collection
	WednesdayTime = "12:00" // Wednesday 12:00 Berlin - present list for selection
	SundayTime = "17:00"    // Sunday 17:00 Berlin - remind about upcoming call
)

// getClassGroupID returns the Telegram group ID where the class is held
func getClassGroupID() (int64, error) {
	groupIDStr := os.Getenv("CLASS_GROUP_ID")
	if groupIDStr == "" {
		return 0, fmt.Errorf("CLASS_GROUP_ID environment variable not set")
	}
	
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CLASS_GROUP_ID: %v", err)
	}
	
	return groupID, nil
}

// scheduleMediaTasks runs the scheduled tasks for media management
func scheduleMediaTasks() {
	loc, err := time.LoadLocation(berlinTimezone)
	if err != nil {
		log.Printf("Error loading timezone: %v", err)
		loc = time.UTC
	}
	
	// Get the Telegram bot
	bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
	if err != nil {
		log.Printf("Error initializing bot for scheduler: %v", err)
		return
	}
	
	// Get group ID
	groupID, err := getClassGroupID()
	if err != nil {
		log.Printf("Scheduler error: %v", err)
		return
	}
	
	for {
		now := time.Now().In(loc)
		
		// Calculate time until next check (1 minute intervals)
		nextCheck := now.Add(time.Minute)
		nextCheck = time.Date(
			nextCheck.Year(), nextCheck.Month(), nextCheck.Day(),
			nextCheck.Hour(), nextCheck.Minute(), 0, 0, loc,
		)
		
		// Sleep until next check
		time.Sleep(time.Until(nextCheck))
		
		// Current time after sleep
		now = time.Now().In(loc)
		weekday := int(now.Weekday())
		timeStr := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
		
		// Check for scheduled events
		switch {
		case weekday == Monday && timeStr == MondayTime:
			// Monday reminder
			go sendMondayReminder(bot, groupID)
			
		case weekday == Wednesday && timeStr == WednesdayTime:
			// Wednesday list presentation
			go sendWednesdayList(bot, groupID)
			
		case weekday == Sunday && timeStr == SundayTime:
			// Sunday reminder
			go sendSundayReminder(bot, groupID)
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
	msg.ParseMode = "Markdown"
	
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
	msg.ParseMode = "Markdown"
	
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
	msg.ParseMode = "Markdown"
	
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("Error sending Sunday reminder: %v", err)
	}
}
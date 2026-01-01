package main

import (
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	classSchedulerTicker *time.Ticker
	classBot             *tgbotapi.BotAPI
)

// startClassScheduler starts the background scheduler for class management
func startClassScheduler(bot *tgbotapi.BotAPI) {
	classBot = bot

	// Run scheduler every 5 minutes
	classSchedulerTicker = time.NewTicker(5 * time.Minute)

	go func() {
		log.Printf("[CLASS] Class scheduler started")

		// Run once immediately on startup
		processClassSchedule()

		for range classSchedulerTicker.C {
			processClassSchedule()
		}
	}()
}

// stopClassScheduler stops the class scheduler
func stopClassScheduler() {
	if classSchedulerTicker != nil {
		classSchedulerTicker.Stop()
		log.Printf("[CLASS] Class scheduler stopped")
	}
}

// processClassSchedule checks for pending tasks and executes them
func processClassSchedule() {
	class, err := getActiveClass()
	if err != nil {
		log.Printf("[ERROR] Failed to get active class: %v", err)
		return
	}

	if class == nil {
		// No active class
		return
	}

	now := time.Now()
	berlin, _ := time.LoadLocation("Europe/Berlin")
	nowBerlin := now.In(berlin)

	log.Printf("[CLASS] Checking schedule for class ID=%d at %s", class.ID, nowBerlin.Format("2006-01-02 15:04 MST"))

	// Task 1: Post announcement (random delay after creation, 1-6 hours)
	if !class.AnnouncementPosted {
		timeSinceCreation := now.Sub(class.CreatedAt)
		delay := getRandomAnnouncementDelay()

		if timeSinceCreation >= delay {
			log.Printf("[CLASS] Time to post announcement for class ID=%d", class.ID)
			postClassAnnouncement(class)
		} else {
			log.Printf("[CLASS] Announcement scheduled in %v", delay-timeSinceCreation)
		}
		return
	}

	// Task 2: Send 6-hour reminder (Sunday 13:00 Berlin)
	reminder6hTime := class.ScheduledTime.Add(-6 * time.Hour)
	if !class.Reminder6hSent && nowBerlin.After(reminder6hTime) {
		log.Printf("[CLASS] Time to send 6h reminder for class ID=%d", class.ID)
		send6hReminder(class)
		return
	}

	// Task 3: Send 1-hour reminder (Sunday 18:00 Berlin)
	reminder1hTime := class.ScheduledTime.Add(-1 * time.Hour)
	if !class.Reminder1hSent && nowBerlin.After(reminder1hTime) {
		log.Printf("[CLASS] Time to send 1h reminder for class ID=%d", class.ID)
		send1hReminder(class)
		return
	}

	// Task 4: Unpin message (Sunday 21:00 Berlin, 2 hours after class)
	unpinTime := class.ScheduledTime.Add(2 * time.Hour)
	if !class.Unpinned && nowBerlin.After(unpinTime) {
		log.Printf("[CLASS] Time to unpin message for class ID=%d", class.ID)
		unpinClassAnnouncement(class)
		return
	}

	// Task 5: Mark class as conducted (Sunday 22:00 Berlin, 3 hours after class)
	conductedTime := class.ScheduledTime.Add(3 * time.Hour)
	if !class.Conducted && nowBerlin.After(conductedTime) {
		log.Printf("[CLASS] Time to mark class as conducted for class ID=%d", class.ID)
		markClassConducted(class)
		return
	}
}

// postClassAnnouncement posts the class announcement to the group
func postClassAnnouncement(class *Class) {
	groupID, err := getClassGroupID()
	if err != nil {
		log.Printf("[ERROR] Failed to get class group ID: %v", err)
		return
	}

	announcement, err := generateClassAnnouncement(class)
	if err != nil {
		log.Printf("[ERROR] Failed to generate announcement: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(groupID, announcement)
	sentMsg, err := classBot.Send(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to send announcement: %v", err)
		return
	}

	log.Printf("[CLASS] Announcement posted, messageID=%d", sentMsg.MessageID)

	// Pin the message
	err = pinMessage(classBot, groupID, sentMsg.MessageID)
	if err != nil {
		log.Printf("[ERROR] Failed to pin announcement: %v", err)
	}

	// Update database
	err = updateClassAnnouncementPosted(class.ID, sentMsg.MessageID)
	if err != nil {
		log.Printf("[ERROR] Failed to update announcement status: %v", err)
	}
}

// send6hReminder sends the 6-hour reminder
func send6hReminder(class *Class) {
	groupID, err := getClassGroupID()
	if err != nil {
		log.Printf("[ERROR] Failed to get class group ID: %v", err)
		return
	}

	// Count RSVPs (placeholder implementation)
	rsvpCount := countRSVPs(groupID, class.AnnouncementMessageID)
	lowRSVP := shouldSendLowRSVPWarning(rsvpCount)

	reminder, err := generateReminder6h(class, rsvpCount, lowRSVP)
	if err != nil {
		log.Printf("[ERROR] Failed to generate 6h reminder: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(groupID, reminder)
	if class.AnnouncementMessageID > 0 {
		msg.ReplyToMessageID = class.AnnouncementMessageID
	}

	_, err = classBot.Send(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to send 6h reminder: %v", err)
		return
	}

	log.Printf("[CLASS] 6h reminder sent (lowRSVP=%v, count=%d)", lowRSVP, rsvpCount)

	err = updateClassReminder6hSent(class.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to update 6h reminder status: %v", err)
	}
}

// send1hReminder sends the 1-hour reminder
func send1hReminder(class *Class) {
	groupID, err := getClassGroupID()
	if err != nil {
		log.Printf("[ERROR] Failed to get class group ID: %v", err)
		return
	}

	reminder, err := generateReminder1h(class)
	if err != nil {
		log.Printf("[ERROR] Failed to generate 1h reminder: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(groupID, reminder)
	if class.AnnouncementMessageID > 0 {
		msg.ReplyToMessageID = class.AnnouncementMessageID
	}

	_, err = classBot.Send(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to send 1h reminder: %v", err)
		return
	}

	log.Printf("[CLASS] 1h reminder sent")

	err = updateClassReminder1hSent(class.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to update 1h reminder status: %v", err)
	}
}

// unpinClassAnnouncement unpins the class announcement message
func unpinClassAnnouncement(class *Class) {
	groupID, err := getClassGroupID()
	if err != nil {
		log.Printf("[ERROR] Failed to get class group ID: %v", err)
		return
	}

	if class.AnnouncementMessageID == 0 {
		log.Printf("[WARN] No announcement message to unpin")
		return
	}

	err = unpinMessage(classBot, groupID, class.AnnouncementMessageID)
	if err != nil {
		log.Printf("[ERROR] Failed to unpin announcement: %v", err)
		return
	}

	log.Printf("[CLASS] Announcement unpinned")

	err = updateClassUnpinned(class.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to update unpin status: %v", err)
	}
}

// markClassConducted marks the class as conducted (finished)
func markClassConducted(class *Class) {
	log.Printf("[CLASS] Marking class as conducted")

	err := updateClassConducted(class.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to update conducted status: %v", err)
		return
	}

	log.Printf("[CLASS] Class ID=%d marked as conducted", class.ID)

	// Cleanup old classes
	if err := cleanupOldClasses(); err != nil {
		log.Printf("[ERROR] Failed to cleanup old classes: %v", err)
	}
}

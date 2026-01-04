package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

// generateClassAnnouncement generates an announcement message using GPT
func generateClassAnnouncement(class *Class) (string, error) {
	if client == nil {
		return "", fmt.Errorf("GPT client not available")
	}

	timezones := formatClassTimeWithTimezones(class.ScheduledTime)

	systemPrompt := `You are a friendly class coordinator. Create an engaging and warm announcement for an upcoming class.

Requirements:
- Be enthusiastic and inviting
- Clearly mention the class topic
- Ask people to RSVP by reacting to the message
- Mention that minimum 4 people are needed to hold the class
- Keep it concise but friendly
- Include a call-to-action for RSVP

Format the response as a complete announcement message ready to post.`

	userMessage := fmt.Sprintf(`Create a class announcement with this information:

Topic: %s
Date: This Sunday

Include these times in the announcement:
%s

Remember to ask for RSVP reactions and mention the 4-person minimum.`, class.Description, timezones)

	announcement, err := getGPTAnswerWithSystem(userMessage, systemPrompt)
	if err != nil {
		log.Printf("[ERROR] Failed to generate announcement: %v", err)
		// Fallback to simple announcement
		return fmt.Sprintf(`Hi everyone! 📚

This Sunday we have a class on: %s

Times:
%s

Please react to this message to RSVP! We need at least 4 people to hold the class. Looking forward to seeing you there! 🎓`, class.Description, timezones), nil
	}

	return announcement, nil
}

// generateReminder6h generates a 6-hour reminder message using GPT
func generateReminder6h(class *Class, rsvpCount int, lowRSVP bool) (string, error) {
	if client == nil {
		return "", fmt.Errorf("GPT client not available")
	}

	systemPrompt := `You are an enthusiastic class coordinator. Create an exciting reminder message for a class starting in 6 hours.

Requirements:
- Create urgency and excitement
- Strong call-to-action to join
- Keep it brief and energetic
- Use emojis appropriately
- Be motivating and encouraging`

	var userMessage string
	if lowRSVP {
		userMessage = fmt.Sprintf(`Create a 6-hour reminder for class "%s" starting in 6 hours.

IMPORTANT: Only %d people have RSVP'd so far. The class might be postponed if we don't get at least 4 people.
Encourage more people to sign up! Make it urgent but friendly.`, class.Description, rsvpCount)
	} else {
		userMessage = fmt.Sprintf(`Create an exciting 6-hour reminder for class "%s" starting in 6 hours.
Make it engaging with a strong call-to-action to join!`, class.Description)
	}

	reminder, err := getGPTAnswerWithSystem(userMessage, systemPrompt)
	if err != nil {
		log.Printf("[ERROR] Failed to generate 6h reminder: %v", err)
		// Fallback
		if lowRSVP {
			return fmt.Sprintf(`⚠️ Class Update: "%s" starts in 6 hours!

Only %d people have RSVP'd so far. We need at least 4 people to hold the class.

If you're planning to join, please react to the announcement message now! Don't let this class get postponed! 📚`, class.Description, rsvpCount), nil
		}
		return fmt.Sprintf(`⏰ Reminder: Class on "%s" starts in 6 hours!

Don't miss out! See you there! 🎓`, class.Description), nil
	}

	return reminder, nil
}

// generateReminder1h generates a 1-hour reminder message using GPT
func generateReminder1h(class *Class) (string, error) {
	if client == nil {
		return "", fmt.Errorf("GPT client not available")
	}

	systemPrompt := `You are an enthusiastic class coordinator. Create an urgent last-minute reminder for a class starting in 1 hour.

Requirements:
- Create strong urgency - it's the final reminder!
- Very strong call-to-action
- Keep it brief and impactful
- Use emojis for emphasis
- Make people feel they'll miss out if they don't join`

	userMessage := fmt.Sprintf(`Create an urgent 1-hour reminder for class "%s" starting in just 1 hour!
This is the last reminder - make it count!
Include this link: https://teams.live.com/meet/9351227050956?p=U9EFHAtxX9RxPW1REt`, class.Description)

	reminder, err := getGPTAnswerWithSystem(userMessage, systemPrompt)
	if err != nil {
		log.Printf("[ERROR] Failed to generate 1h reminder: %v", err)
		// Fallback
		return fmt.Sprintf(`🚨 FINAL REMINDER: Class on "%s" starts in 1 HOUR!

Link to join: https://teams.live.com/meet/9351227050956?p=U9EFHAtxX9RxPW1REt

This is your last chance to join! See you soon! 🎓⏰`, class.Description), nil
	}

	return reminder, nil
}

// generateQuestionsRequest generates the 3-day notification requesting questions and link
func generateQuestionsRequest(class *Class, ownerID int64) (string, error) {
	// We'll construct a mention for the owner
	// Note: In Telegram Markdown/HTML, mentioning by ID might be tricky if user has no username.
	// But the bot stores username usually. We'll use the ID mention which is reliable.
	// [Name](tg://user?id=123456789)

	msg := fmt.Sprintf("Hey [Owner](tg://user?id=%d)! 👋\n\n"+
		"The class **%s** is coming up in 3 days (Schedule: %s).\n\n"+
		"Please reply to this message with the discussion questions and the call link. 📝🔗\n"+
		"I'll pin your reply so everyone can see it!",
		ownerID,
		class.Description,
		class.ScheduledTime.Format("Monday 15:04"),
	)
	return msg, nil
}

// countRSVPs counts the number of reactions on a message by querying the database
func countRSVPs(chatID int64, messageID int) int {
	// Get the active class with this message ID
	query := `
		SELECT COUNT(DISTINCT r.user_id)
		FROM class_rsvps r
		JOIN classes c ON r.class_id = c.id
		WHERE c.announcement_message_id = ? AND c.cancelled = 0
	`

	var count int
	err := db.QueryRow(query, messageID).Scan(&count)
	if err != nil {
		log.Printf("[ERROR] Failed to count RSVPs for message %d: %v", messageID, err)
		return 0
	}

	log.Printf("[CLASS] Counted %d RSVPs for message %d", count, messageID)
	return count
}

// trackRSVP records an RSVP (reaction) to a class announcement
func trackRSVP(messageID int, userID int64, username string) error {
	// First, find the class with this announcement message ID
	var classID int
	err := db.QueryRow(`
		SELECT id FROM classes
		WHERE announcement_message_id = ? AND cancelled = 0
		LIMIT 1
	`, messageID).Scan(&classID)

	if err == sql.ErrNoRows {
		// Not a class announcement message, ignore
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to find class for message %d: %v", messageID, err)
	}

	// Insert or update RSVP
	_, err = db.Exec(`
		INSERT OR REPLACE INTO class_rsvps (class_id, message_id, user_id, username, reacted_at)
		VALUES (?, ?, ?, ?, datetime('now'))
	`, classID, messageID, userID, username)

	if err != nil {
		return fmt.Errorf("failed to track RSVP: %v", err)
	}

	log.Printf("[CLASS] Tracked RSVP from user %s (ID: %d) for class %d", username, userID, classID)
	return nil
}

// removeRSVP removes an RSVP when a user removes their reaction
func removeRSVP(messageID int, userID int64) error {
	result, err := db.Exec(`
		DELETE FROM class_rsvps
		WHERE message_id = ? AND user_id = ?
	`, messageID, userID)

	if err != nil {
		return fmt.Errorf("failed to remove RSVP: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[CLASS] Removed RSVP from user ID %d for message %d", userID, messageID)
	}

	return nil
}

// shouldSendLowRSVPWarning determines if we should warn about low RSVP count
func shouldSendLowRSVPWarning(rsvpCount int) bool {
	return rsvpCount < 4 && rsvpCount >= 0
}

// getClassGroupID returns the class group ID from environment
func getClassGroupID() (int64, error) {
	return getEnvInt64("CLASS_GROUP_ID")
}

// getOwnerID returns the owner ID from environment
func getOwnerID() (int64, error) {
	ownerID, err := getEnvInt64("OWNER_ID")
	if err != nil {
		return 0, err
	}
	return ownerID, nil
}

// isOwner checks if the user is the bot owner
func isOwner(userID int64) bool {
	ownerID, err := getOwnerID()
	if err != nil {
		log.Printf("[ERROR] Failed to get owner ID: %v", err)
		return false
	}
	return userID == ownerID
}

// cleanupOldClasses removes old cancelled or completed classes from the database
func cleanupOldClasses() error {
	// Keep last 10 classes for history
	query := `
		DELETE FROM classes
		WHERE id NOT IN (
			SELECT id FROM classes
			ORDER BY created_at DESC
			LIMIT 10
		)
	`

	result, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to cleanup old classes: %v", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		log.Printf("[CLASS] Cleaned up %d old classes", rows)
	}

	return nil
}

// getRandomAnnouncementDelay returns a random delay between 1-6 hours for posting announcement
func getRandomAnnouncementDelay() time.Duration {
	// Random delay between 1-6 hours
	hours := 1 + (time.Now().UnixNano() % 6)
	delay := time.Duration(hours) * time.Hour
	log.Printf("[CLASS] Announcement will be posted in %v", delay)
	return delay
}

// sanitizeClassDescription cleans up the class description
func sanitizeClassDescription(description string) string {
	description = strings.TrimSpace(description)
	if len(description) > 500 {
		description = description[:500]
	}
	return description
}

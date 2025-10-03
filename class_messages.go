package main

import (
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
This is the last reminder - make it count!`, class.Description)

	reminder, err := getGPTAnswerWithSystem(userMessage, systemPrompt)
	if err != nil {
		log.Printf("[ERROR] Failed to generate 1h reminder: %v", err)
		// Fallback
		return fmt.Sprintf(`🚨 FINAL REMINDER: Class on "%s" starts in 1 HOUR!

This is your last chance to join! See you soon! 🎓⏰`, class.Description), nil
	}

	return reminder, nil
}

// countRSVPs counts the number of reactions on a message
func countRSVPs(chatID int64, messageID int) int {
	// Note: Telegram Bot API doesn't provide a direct way to count reactions
	// This is a placeholder - in practice, we'd need to track reactions through updates
	// or use a different approach

	// For now, return 0 as we can't easily get this info
	// The bot owner would need to manually check or we'd need to implement
	// reaction tracking in the update handler

	log.Printf("[WARN] RSVP counting not fully implemented - returning 0")
	return 0
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
func getOwnerID() (int, error) {
	ownerID, err := getEnvInt64("OWNER_ID")
	if err != nil {
		return 0, err
	}
	return int(ownerID), nil
}

// isOwner checks if the user is the bot owner
func isOwner(userID int) bool {
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

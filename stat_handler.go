package main

import (
	"fmt"
	"log"
	"sort"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleStatCommand processes the /stat command
func handleStatCommand(bot *tgbotapi.BotAPI, messg *tgbotapi.Message) {
	// Only work in groups
	if !messg.Chat.IsGroup() && !messg.Chat.IsSuperGroup() {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "This command only works in groups.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check if user is admin or owner
	isAuthorized, err := isUserAuthorizedForStats(bot, messg)
	if err != nil {
		log.Printf("[ERROR] Failed to check authorization: %v", err)
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Failed to verify permissions.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	if !isAuthorized {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "This command is only available to group admins and bot owner.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Check rate limit
	allowed, err := checkStatRateLimit(messg.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(messg.Chat.ID, fmt.Sprintf("⏱️ Rate limit: %v", err))
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	if !allowed {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "⏱️ Please wait before running /stat again.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Get activity stats for last 7 days
	stats, err := getGroupActivityStats(messg.Chat.ID, 7)
	if err != nil {
		log.Printf("[ERROR] Failed to get activity stats: %v", err)
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Failed to retrieve statistics.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	if len(stats.UserStats) == 0 {
		msg := tgbotapi.NewMessage(messg.Chat.ID, "No activity data available for the last 7 days.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Generate chart
	chartBuffer, err := generateActivityChart(stats)
	if err != nil {
		log.Printf("[ERROR] Failed to generate chart: %v", err)
		msg := tgbotapi.NewMessage(messg.Chat.ID, "Failed to generate chart.")
		msg.ReplyToMessageID = messg.MessageID
		bot.Send(msg)
		return
	}

	// Send chart as photo
	photoMsg := tgbotapi.NewPhoto(messg.Chat.ID, tgbotapi.FileBytes{
		Name:  "activity_chart.png",
		Bytes: chartBuffer.Bytes(),
	})
	photoMsg.ReplyToMessageID = messg.MessageID
	photoMsg.Caption = "📊 Group Activity Statistics (Last 7 Days)"

	_, err = bot.Send(photoMsg)
	if err != nil {
		log.Printf("[ERROR] Failed to send chart: %v", err)
		return
	}

	// Update rate limit
	err = updateStatRateLimit(messg.Chat.ID)
	if err != nil {
		log.Printf("[ERROR] Failed to update rate limit: %v", err)
	}

	// Generate GPT analysis in background
	go generateStatAnalysis(bot, messg, stats)
}

// isUserAuthorizedForStats checks if user is group admin or bot owner
func isUserAuthorizedForStats(bot *tgbotapi.BotAPI, messg *tgbotapi.Message) (bool, error) {
	// Check if user is bot owner
	ownerID, err := getOwnerID()
	if err == nil && messg.From.ID == ownerID {
		return true, nil
	}

	// Check if user is group admin
	chatConfig := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: messg.Chat.ID,
			UserID: messg.From.ID,
		},
	}

	member, err := bot.GetChatMember(chatConfig)
	if err != nil {
		return false, err
	}

	// Check if user is admin or creator
	if member.IsAdministrator() || member.IsCreator() {
		return true, nil
	}

	return false, nil
}

// generateStatAnalysis uses GPT to analyze activity patterns
func generateStatAnalysis(bot *tgbotapi.BotAPI, messg *tgbotapi.Message, stats *ActivityStats) {
	if client == nil {
		return
	}

	// Prepare data summary for GPT
	summary := buildActivitySummary(stats)

	prompt := fmt.Sprintf(`Analyze this Telegram group activity data:

%s

Provide a SHORT analysis with:
1. Mention top 4 contributors with their message counts in a single sentence
2. ONE interesting fun fact about the activity patterns

Keep it super brief (max 100 words total). Be casual and friendly.`, summary)

	analysis, err := getGPTAnswerWithSystem(prompt, "You are a friendly data analyst who provides brief, fun insights about group chat activity.")
	if err != nil {
		log.Printf("[ERROR] GPT analysis failed: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(messg.Chat.ID, "🔍 *Activity Analysis*\n\n"+analysis)
	msg.ParseMode = "Markdown"
	msg.ReplyToMessageID = messg.MessageID
	_, err = bot.Send(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to send analysis: %v", err)
	}
}

// buildActivitySummary creates a text summary of activity stats for GPT
func buildActivitySummary(stats *ActivityStats) string {
	// Sort users by total messages
	sort.Slice(stats.UserStats, func(i, j int) bool {
		return stats.UserStats[i].TotalMessages > stats.UserStats[j].TotalMessages
	})

	summary := fmt.Sprintf("Group Activity Summary (Last 7 Days)\n")
	summary += fmt.Sprintf("Period: %s to %s\n\n",
		stats.StartTime.Format("Jan 2"),
		stats.EndTime.Format("Jan 2"))

	// Top contributors
	summary += "Top Contributors:\n"
	for i, user := range stats.UserStats {
		if i >= 10 {
			break
		}
		summary += fmt.Sprintf("%d. @%s - %d messages\n", i+1, user.UserName, user.TotalMessages)
	}

	// Calculate peak hours for each user
	summary += "\nActivity Patterns:\n"
	for i, user := range stats.UserStats {
		if i >= 5 {
			break
		}
		peakHour, peakCount := findPeakHour(user)
		if peakCount > 0 {
			summary += fmt.Sprintf("@%s most active at %02d:00 (%d msgs)\n",
				user.UserName, peakHour, peakCount)
		}
	}

	return summary
}

// findPeakHour finds the hour with most activity for a user
func findPeakHour(user UserActivitySummary) (int, int) {
	hourCounts := make(map[int]int)

	for hourBucket, count := range user.HourlyData {
		hour := hourBucket.Hour()
		hourCounts[hour] += count
	}

	maxHour := 0
	maxCount := 0
	for hour, count := range hourCounts {
		if count > maxCount {
			maxCount = count
			maxHour = hour
		}
	}

	return maxHour, maxCount
}

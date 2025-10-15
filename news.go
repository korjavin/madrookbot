package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	newsRand      *rand.Rand
	newsScheduler *time.Ticker
)

func init() {
	newsRand = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// initNewsTable creates the table for tracking news posts
func initNewsTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS news_posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		last_post_time DATETIME NOT NULL,
		topic TEXT NOT NULL,
		news_url TEXT NOT NULL,
		message_id INTEGER NOT NULL
	);`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create news_posts table: %w", err)
	}

	log.Printf("[INFO] News posts table initialized")
	return nil
}

// getLastNewsPostTime returns the timestamp of the last news post
func getLastNewsPostTime() (time.Time, error) {
	var lastPostTime time.Time
	query := `SELECT last_post_time FROM news_posts ORDER BY last_post_time DESC LIMIT 1`

	err := db.QueryRow(query).Scan(&lastPostTime)
	if err == sql.ErrNoRows {
		// No posts yet, return zero time
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last news post time: %w", err)
	}

	return lastPostTime, nil
}

// recordNewsPost saves a news post to the database
func recordNewsPost(topic, newsURL string, messageID int) error {
	query := `INSERT INTO news_posts (last_post_time, topic, news_url, message_id) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, time.Now(), topic, newsURL, messageID)
	if err != nil {
		return fmt.Errorf("failed to record news post: %w", err)
	}
	return nil
}

// isGroupQuiet checks if there have been no messages in the memory group for the specified duration
// Uses the activity table for faster queries
func isGroupQuiet(quietDuration time.Duration) (bool, error) {
	memoryGroupID, err := getEnvInt64("MEMORY_GROUP_ID")
	if err != nil {
		return false, fmt.Errorf("MEMORY_GROUP_ID not set: %w", err)
	}

	// Calculate the timestamp threshold
	threshold := time.Now().Add(-quietDuration)

	// Query the activity table for recent activity
	var count int
	query := `SELECT COUNT(*) FROM user_activity WHERE group_id = ? AND hour_bucket >= ?`
	err = db.QueryRow(query, memoryGroupID, threshold).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to query activity table: %w", err)
	}

	isQuiet := count == 0
	log.Printf("[NEWS] Group quiet check: %v (found %d activity records in last %v)", isQuiet, count, quietDuration)

	return isQuiet, nil
}

// getRandomRecentMessage retrieves a random message from the last 20 hours
// Only selects messages longer than 130 characters for better topic extraction
func getRandomRecentMessage() (string, error) {
	ctx := context.Background()

	// Calculate the timestamp for 20 hours ago
	twentyHoursAgo := time.Now().UTC().Add(-20 * time.Hour)

	// Create filter for messages in the last 20 hours
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: "timestamp_utc",
						DatetimeRange: &qdrant.DatetimeRange{
							Gte: timestamppb.New(twentyHoursAgo),
						},
					},
				},
			},
		},
	}

	// Scroll through messages (fetch up to 200 to have a good pool after filtering)
	limit := uint32(200)
	scrollResp, err := qdrantClient.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: qdrantCollectionName,
		Filter:         filter,
		Limit:          &limit,
		WithPayload:    &qdrant.WithPayloadSelector{SelectorOptions: &qdrant.WithPayloadSelector_Enable{Enable: true}},
	})

	if err != nil {
		return "", fmt.Errorf("failed to query Qdrant: %w", err)
	}

	if len(scrollResp.Result) == 0 {
		return "", fmt.Errorf("no messages found in the last 20 hours")
	}

	// Filter messages by length (min 130 characters)
	const minMessageLength = 130
	var validMessages []string

	for _, point := range scrollResp.Result {
		if textValue, ok := point.Payload["text"]; ok {
			if strValue, ok := textValue.GetKind().(*qdrant.Value_StringValue); ok {
				text := strValue.StringValue
				if len(text) >= minMessageLength {
					validMessages = append(validMessages, text)
				}
			}
		}
	}

	if len(validMessages) == 0 {
		return "", fmt.Errorf("no messages longer than %d characters found in the last 20 hours", minMessageLength)
	}

	// Pick a random message from the filtered results
	randomIndex := newsRand.Intn(len(validMessages))
	selectedText := validMessages[randomIndex]

	log.Printf("[NEWS] Selected random message (index %d of %d valid messages, total %d): %.100s...",
		randomIndex, len(validMessages), len(scrollResp.Result), selectedText)

	return selectedText, nil
}

// retryWithBackoff retries a function with exponential backoff
func retryWithBackoff(operation func() error, maxRetries int, baseDelay time.Duration) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err

		if i < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<uint(i)) // Exponential backoff: 2s, 4s, 8s, 16s, 32s
			log.Printf("[NEWS] Retry %d/%d failed: %v. Retrying in %v...", i+1, maxRetries, err, delay)
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// extractTopicFromMessage uses LLM to extract a topic/theme from a message
// Retries up to 5 times with exponential backoff on failures
func extractTopicFromMessage(message string) (string, error) {
	systemPrompt := `You are a helpful assistant that extracts the main topic or theme from a text message.
Provide a brief topic description (2-5 words) that could be used to search for related news.
Only respond with the topic, nothing else.

Examples:
- Message: "I've been thinking about getting solar panels for my house" -> Topic: "residential solar panels"
- Message: "The new AI model is amazing!" -> Topic: "artificial intelligence"
- Message: "Bitcoin is going crazy today" -> Topic: "cryptocurrency Bitcoin"`

	userMessage := fmt.Sprintf("Extract the main topic from this message:\n\n%s", message)

	var topic string
	err := retryWithBackoff(func() error {
		var err error
		topic, err = getGPTAnswerWithSystem(userMessage, systemPrompt)
		return err
	}, 5, 2*time.Second)

	if err != nil {
		return "", fmt.Errorf("failed to extract topic: %w", err)
	}

	topic = strings.TrimSpace(topic)
	log.Printf("[NEWS] Extracted topic: %s", topic)
	return topic, nil
}

// GoogleNewsRSS represents the XML structure of Google News RSS feed
type GoogleNewsRSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			PubDate     string `xml:"pubDate"`
			Description string `xml:"description"`
		} `xml:"item"`
	} `xml:"channel"`
}

// searchGoogleNews searches Google News for articles related to the topic
// Returns title, description (truncated to 2000 chars), and URL
func searchGoogleNews(topic string) (string, string, string, error) {
	// Google News RSS feed URL with "when:7d" parameter to limit to last 7 days
	// Format: https://news.google.com/rss/search?q=QUERY+when:7d&hl=en-US&gl=US&ceid=US:en
	encodedTopic := url.QueryEscape(topic + " when:7d")
	rssURL := fmt.Sprintf("https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en", encodedTopic)

	log.Printf("[NEWS] Searching Google News for: %s (last 7 days)", topic)

	// Fetch RSS feed
	resp, err := http.Get(rssURL)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch news RSS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("news RSS returned status: %d", resp.StatusCode)
	}

	// Read and parse XML
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read RSS body: %w", err)
	}

	var rss GoogleNewsRSS
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse RSS XML: %w", err)
	}

	if len(rss.Channel.Items) == 0 {
		return "", "", "", fmt.Errorf("no news articles found for topic: %s", topic)
	}

	// Pick a random article from the first 5 results
	maxItems := 5
	if len(rss.Channel.Items) < maxItems {
		maxItems = len(rss.Channel.Items)
	}

	randomIndex := newsRand.Intn(maxItems)
	article := rss.Channel.Items[randomIndex]

	// Truncate description to 2000 chars to avoid context overflow
	description := article.Description
	if len(description) > 2000 {
		description = description[:2000] + "..."
	}

	log.Printf("[NEWS] Found news article: %s (description length: %d)", article.Title, len(description))
	return article.Title, description, article.Link, nil
}

// generateConservativeComment generates a conservative/redneck-style comment about the news
// Retries up to 5 times with exponential backoff on failures
func generateConservativeComment(newsTitle, newsDescription, newsURL string) (string, error) {
	// Get system prompt from environment variable or use default
	systemPrompt := os.Getenv("NEWS_COMMENT_PROMPT")
	if systemPrompt == "" {
		// Default prompt
		systemPrompt = `You are a middle-aged conservative man, almost a redneck, but still polite and well-meaning.
You have traditional values, are skeptical of mainstream media and government, value hard work and self-reliance.
You comment on news with a mix of common sense, traditional wisdom, and slight distrust of "the establishment."

Style guidelines:
- Be polite but opinionated
- Use occasional colloquialisms (but don't overdo it)
- Show skepticism toward big tech, government overreach, and liberal policies
- Value family, community, hard work, and personal responsibility
- Keep it concise (2-4 sentences)
- Sometimes reference "back in my day" or traditional wisdom
- Be genuine and authentic, not a caricature

This is a parody account, so it should be entertaining but not offensive.`
	}

	userMessage := fmt.Sprintf("Comment on this news:\n\nHeadline: %s\n\nSummary: %s\n\nNews URL: %s", newsTitle, newsDescription, newsURL)

	var comment string
	err := retryWithBackoff(func() error {
		var err error
		comment, err = getGPTAnswerWithSystem(userMessage, systemPrompt)
		return err
	}, 5, 2*time.Second)

	if err != nil {
		return "", fmt.Errorf("failed to generate comment: %w", err)
	}

	log.Printf("[NEWS] Generated comment: %s", comment)
	return comment, nil
}

// shouldPostNewsNow checks if it's the right time to post news
func shouldPostNewsNow(bot *tgbotapi.BotAPI) bool {
	// Check if we already posted today
	lastPost, err := getLastNewsPostTime()
	if err != nil {
		log.Printf("[NEWS] Error checking last post time: %v", err)
		return false
	}

	// If we posted in the last 24 hours, don't post again
	if !lastPost.IsZero() && time.Since(lastPost) < 24*time.Hour {
		log.Printf("[NEWS] Already posted in the last 24 hours (last post: %v)", lastPost)
		return false
	}

	// Check if it's within the allowed time window (17:00-23:50 Berlin time)
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.Printf("[NEWS] Error loading Berlin timezone: %v", err)
		return false
	}

	now := time.Now().In(berlin)
	hour := now.Hour()
	minute := now.Minute()

	// Check if between 17:00 and 23:50
	if hour < 17 || (hour == 23 && minute > 50) || hour >= 24 {
		log.Printf("[NEWS] Outside posting window (current time: %02d:%02d Berlin)", hour, minute)
		return false
	}

	// Check if the group has been quiet for at least 3 hours
	isQuiet, err := isGroupQuiet(3 * time.Hour)
	if err != nil {
		log.Printf("[NEWS] Error checking if group is quiet: %v", err)
		return false
	}

	if !isQuiet {
		log.Printf("[NEWS] Group is not quiet (had messages in last 3 hours)")
		return false
	}

	log.Printf("[NEWS] All conditions met for posting news")
	return true
}

// sendOwnerWarning sends a warning message to the bot owner's DM
func sendOwnerWarning(bot *tgbotapi.BotAPI, warning string) {
	ownerIDStr := os.Getenv("OWNER_ID")
	if ownerIDStr == "" {
		log.Printf("[NEWS] Cannot send owner warning: OWNER_ID not set")
		return
	}

	ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64)
	if err != nil {
		log.Printf("[NEWS] Cannot send owner warning: invalid OWNER_ID: %v", err)
		return
	}

	warningMsg := tgbotapi.NewMessage(ownerID, fmt.Sprintf("⚠️ News Feature Warning\n\n%s", warning))
	_, err = bot.Send(warningMsg)
	if err != nil {
		log.Printf("[NEWS] Failed to send warning to owner: %v", err)
	}
}

// tryPostNews attempts to post a news comment to the memory group
// Errors are logged and sent to owner DM, not posted to the group
func tryPostNews(bot *tgbotapi.BotAPI) {
	if !shouldPostNewsNow(bot) {
		return
	}

	memoryGroupID, err := getEnvInt64("MEMORY_GROUP_ID")
	if err != nil {
		log.Printf("[NEWS] Error getting memory group ID: %v", err)
		return
	}

	// Step 1: Get a random recent message
	randomMessage, err := getRandomRecentMessage()
	if err != nil {
		log.Printf("[NEWS] Error getting random message: %v", err)
		sendOwnerWarning(bot, fmt.Sprintf("Failed to get random message:\n%v", err))
		return
	}

	// Step 2: Extract topic from the message
	topic, err := extractTopicFromMessage(randomMessage)
	if err != nil {
		log.Printf("[NEWS] Error extracting topic: %v", err)
		sendOwnerWarning(bot, fmt.Sprintf("Failed to extract topic from message:\n%v\n\nMessage: %.200s...", err, randomMessage))
		return
	}

	// Step 3: Search for news about the topic
	newsTitle, newsDescription, newsURL, err := searchGoogleNews(topic)
	if err != nil {
		log.Printf("[NEWS] Error searching news: %v", err)
		sendOwnerWarning(bot, fmt.Sprintf("Failed to find news for topic '%s':\n%v", topic, err))
		return
	}

	// Step 4: Generate conservative comment
	comment, err := generateConservativeComment(newsTitle, newsDescription, newsURL)
	if err != nil {
		log.Printf("[NEWS] Error generating comment: %v", err)
		sendOwnerWarning(bot, fmt.Sprintf("Failed to generate comment for:\n%s\n\nError: %v", newsTitle, err))
		return
	}

	// Step 5: Post to the group
	message := fmt.Sprintf("%s\n\n%s", comment, newsURL)
	msg := tgbotapi.NewMessage(memoryGroupID, message)

	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("[NEWS] Error sending news comment: %v", err)
		sendOwnerWarning(bot, fmt.Sprintf("Failed to send message to group:\n%v", err))
		return
	}

	// Step 6: Record the post
	err = recordNewsPost(topic, newsURL, sentMsg.MessageID)
	if err != nil {
		log.Printf("[NEWS] Error recording news post: %v", err)
		// Don't send warning for database errors, just log
		return
	}

	log.Printf("[NEWS] Successfully posted news comment (message ID: %d)", sentMsg.MessageID)
}

// startNewsScheduler starts the news posting scheduler
func startNewsScheduler(bot *tgbotapi.BotAPI) {
	if os.Getenv("MEMORY_GROUP_ID") == "" {
		log.Printf("[NEWS] News scheduler disabled (MEMORY_GROUP_ID not set)")
		return
	}

	// Initialize the news table
	err := initNewsTable()
	if err != nil {
		log.Printf("[NEWS] Failed to initialize news table: %v", err)
		return
	}

	log.Printf("[NEWS] Starting news scheduler (checks every 10 minutes)")

	// Check every 10 minutes for the right conditions to post
	newsScheduler = time.NewTicker(10 * time.Minute)

	go func() {
		// Try immediately on startup (for testing, will respect the conditions)
		tryPostNews(bot)

		// Then check every 10 minutes
		for range newsScheduler.C {
			tryPostNews(bot)
		}
	}()
}

// stopNewsScheduler stops the news posting scheduler
func stopNewsScheduler() {
	if newsScheduler != nil {
		newsScheduler.Stop()
		log.Printf("[NEWS] News scheduler stopped")
	}
}

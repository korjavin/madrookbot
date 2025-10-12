package main

import (
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	reviewRand     *rand.Rand
	ignoredUserIDs map[int64]bool
)

func init() {
	// Initialize random generator for review feature
	reviewRand = rand.New(rand.NewSource(time.Now().UnixNano()))

	// Initialize ignored users list
	ignoredUserIDs = parseIgnoredUsers()

	// Log review feature configuration at startup
	percentage := getReviewPercentage()
	minLength := getReviewMinLength()
	hasCustomPrompt := os.Getenv("REVIEW_PROMPT") != ""

	if percentage > 0 {
		log.Printf("[INFO] English review feature enabled: percentage=%.2f%%, minLength=%d, customPrompt=%v, ignoredUsers=%d",
			percentage, minLength, hasCustomPrompt, len(ignoredUserIDs))
	} else {
		log.Printf("[INFO] English review feature disabled (percentage=0)")
	}
}

// parseIgnoredUsers parses the PARTICIPANTS_IGNORE environment variable
// and returns a map of user IDs to ignore
func parseIgnoredUsers() map[int64]bool {
	ignored := make(map[int64]bool)
	ignoreStr := os.Getenv("PARTICIPANTS_IGNORE")
	if ignoreStr == "" {
		return ignored
	}

	// Split by comma and parse each ID
	ids := strings.Split(ignoreStr, ",")
	for _, idStr := range ids {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			log.Printf("[WARN] Invalid user ID in PARTICIPANTS_IGNORE: %s", idStr)
			continue
		}
		ignored[id] = true
		log.Printf("[INFO] User ID %d will be ignored from grammar reviews", id)
	}

	return ignored
}

// getReviewPercentage returns the percentage chance of reviewing a message (0-100)
func getReviewPercentage() float64 {
	percentStr := os.Getenv("REVIEW_PERCENTAGE")
	if percentStr == "" {
		return 1.0 // Default 1%
	}
	percent, err := strconv.ParseFloat(percentStr, 64)
	if err != nil {
		log.Printf("[WARN] Invalid REVIEW_PERCENTAGE value: %s, using default 1.0", percentStr)
		return 1.0
	}
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

// getReviewMinLength returns the minimum message length to review
func getReviewMinLength() int {
	lengthStr := os.Getenv("REVIEW_MIN_LENGTH")
	if lengthStr == "" {
		return 130 // Default 130 characters
	}
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		log.Printf("[WARN] Invalid REVIEW_MIN_LENGTH value: %s, using default 130", lengthStr)
		return 130
	}
	if length < 0 {
		return 0
	}
	return length
}

// getReviewPrompt returns the system prompt for English review
func getReviewPrompt() string {
	prompt := os.Getenv("REVIEW_PROMPT")
	if prompt != "" {
		return prompt
	}

	// Default prompt
	return `You are an advanced English teacher reviewing a student's text message.

Analyze the text for MAJOR English mistakes only, such as:
- Serious grammatical errors that affect meaning
- Significant vocabulary misuse
- Major sentence structure problems
- Critical spelling errors of common words

IGNORE minor issues like:
- Stylistic preferences
- Informal language or slang (unless clearly wrong)
- Minor punctuation variations
- Colloquialisms
- Single-letter typos in otherwise correct text

If you find MAJOR mistakes, respond in this format:
MISTAKES FOUND
{original part with mistake} -->> {corrected version}

{brief explanation of the issue}

If there are NO major mistakes or you're uncertain, respond with exactly:
NO MAJOR MISTAKES

Be strict - only report issues you're absolutely confident about. When in doubt, say NO MAJOR MISTAKES.`
}

// shouldReviewMessage determines if a message should be reviewed based on random chance
func shouldReviewMessage() bool {
	percentage := getReviewPercentage()
	if percentage <= 0 {
		log.Printf("[DEBUG] Review feature disabled (percentage: %.2f)", percentage)
		return false
	}
	randomValue := reviewRand.Float64() * 100.0
	shouldReview := randomValue < percentage
	log.Printf("[DEBUG] Review random check: %.2f < %.2f = %v", randomValue, percentage, shouldReview)
	return shouldReview
}

// isReviewableMessage checks if the message is eligible for review
// (plain text, not a command, not a bot mention, not too short, user not ignored)
func isReviewableMessage(text string, minLength int, userID int64) bool {
	if text == "" {
		log.Printf("[DEBUG] Review skipped: empty message")
		return false
	}

	// Skip if user is in the ignore list
	if ignoredUserIDs[userID] {
		log.Printf("[DEBUG] Review skipped: user ID %d is in ignore list", userID)
		return false
	}

	// Skip if too short
	if len(text) < minLength {
		log.Printf("[DEBUG] Review skipped: message too short (len=%d, min=%d)", len(text), minLength)
		return false
	}

	// Skip if it's a command
	if strings.HasPrefix(text, "/") {
		log.Printf("[DEBUG] Review skipped: command detected")
		return false
	}

	// Skip if it contains common bot interaction patterns
	upperText := strings.ToUpper(text)
	if strings.Contains(upperText, "READ:") ||
		strings.Contains(upperText, "IMAGE:") ||
		strings.HasPrefix(upperText, "@") {
		log.Printf("[DEBUG] Review skipped: bot interaction pattern detected")
		return false
	}

	log.Printf("[DEBUG] Message is reviewable (len=%d, userID=%d)", len(text), userID)
	return true
}

// reviewEnglish sends the text to GPT for English review
// Returns the review text if major mistakes found, empty string otherwise
func reviewEnglish(text string) (string, error) {
	if client == nil {
		log.Printf("[WARN] Review skipped: GPT client not initialized")
		return "", nil
	}

	systemPrompt := getReviewPrompt()
	log.Printf("[DEBUG] Sending text to GPT for review (length: %d)", len(text))

	// Prepare the user message
	userMessage := "Review this text:\n\n" + text

	response, err := getGPTAnswerWithSystem(userMessage, systemPrompt)
	if err != nil {
		log.Printf("[ERROR] English review GPT error: %v", err)
		return "", err
	}

	log.Printf("[DEBUG] GPT review response received (length: %d)", len(response))

	// Check if the response indicates major mistakes were found
	if strings.Contains(response, "NO MAJOR MISTAKES") {
		log.Printf("[INFO] Review complete: no major mistakes found")
		return "", nil
	}

	// Only return the review if it contains "MISTAKES FOUND"
	if strings.Contains(response, "MISTAKES FOUND") {
		log.Printf("[INFO] Review complete: major mistakes found, will send review")
		return response, nil
	}

	// If response is ambiguous, don't send it
	log.Printf("[WARN] Review complete: ambiguous response (neither MISTAKES FOUND nor NO MAJOR MISTAKES), skipping")
	return "", nil
}

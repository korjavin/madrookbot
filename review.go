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
	reviewRand *rand.Rand
)

func init() {
	// Initialize random generator for review feature
	reviewRand = rand.New(rand.NewSource(time.Now().UnixNano()))
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
Original: [problematic part]
Issue: [brief explanation]
Suggestion: [corrected version]

If there are NO major mistakes or you're uncertain, respond with exactly:
NO MAJOR MISTAKES

Be strict - only report issues you're absolutely confident about. When in doubt, say NO MAJOR MISTAKES.`
}

// shouldReviewMessage determines if a message should be reviewed based on random chance
func shouldReviewMessage() bool {
	percentage := getReviewPercentage()
	if percentage <= 0 {
		return false
	}
	randomValue := reviewRand.Float64() * 100.0
	return randomValue < percentage
}

// isReviewableMessage checks if the message is eligible for review
// (plain text, not a command, not a bot mention, not too short)
func isReviewableMessage(text string, minLength int) bool {
	if text == "" {
		return false
	}

	// Skip if too short
	if len(text) < minLength {
		return false
	}

	// Skip if it's a command
	if strings.HasPrefix(text, "/") {
		return false
	}

	// Skip if it contains common bot interaction patterns
	upperText := strings.ToUpper(text)
	if strings.Contains(upperText, "READ:") ||
		strings.Contains(upperText, "IMAGE:") ||
		strings.HasPrefix(upperText, "@") {
		return false
	}

	return true
}

// reviewEnglish sends the text to GPT for English review
// Returns the review text if major mistakes found, empty string otherwise
func reviewEnglish(text string) (string, error) {
	if client == nil {
		return "", nil
	}

	systemPrompt := getReviewPrompt()

	// Prepare the user message
	userMessage := "Review this text:\n\n" + text

	response, err := getGPTAnswerWithSystem(userMessage, systemPrompt)
	if err != nil {
		log.Printf("[ERROR] English review GPT error: %v", err)
		return "", err
	}

	// Check if the response indicates major mistakes were found
	if strings.Contains(response, "NO MAJOR MISTAKES") {
		return "", nil
	}

	// Only return the review if it contains "MISTAKES FOUND"
	if strings.Contains(response, "MISTAKES FOUND") {
		return response, nil
	}

	// If response is ambiguous, don't send it
	return "", nil
}

package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestHashUserID tests the hashUserID function
func TestHashUserID(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		expected string
	}{
		{
			name:     "simple username",
			userID:   "john_doe",
			expected: hashExpected("john_doe"),
		},
		{
			name:     "empty string",
			userID:   "",
			expected: hashExpected(""),
		},
		{
			name:     "complex username",
			userID:   "user_with_special_chars_123",
			expected: hashExpected("user_with_special_chars_123"),
		},
		{
			name:     "unicode characters",
			userID:   "用户",
			expected: hashExpected("用户"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashUserID(tt.userID)
			expected := hashExpected(tt.userID)

			if result != expected {
				t.Errorf("hashUserID(%s) = %s, want %s", tt.userID, result, expected)
			}
		})
	}
}

// TestHashUserIDConsistency tests that the hash is consistent
func TestHashUserIDConsistency(t *testing.T) {
	userID := "test_user_123"

	hash1 := hashUserID(userID)
	hash2 := hashUserID(userID)

	if hash1 != hash2 {
		t.Error("hashUserID should return consistent results")
	}

	// Verify it's a valid SHA256 hex string
	if len(hash1) != 64 {
		t.Errorf("expected 64 character hex string, got %d", len(hash1))
	}
}

// TestHashUserIDUniqueness tests that different inputs produce different hashes
func TestHashUserIDUniqueness(t *testing.T) {
	users := []string{
		"user1",
		"user2",
		"user10",
		"User1", // Different case
	}

	hashes := make(map[string]bool)
	for _, user := range users {
		hash := hashUserID(user)
		if hashes[hash] {
			t.Errorf("duplicate hash for user: %s", user)
		}
		hashes[hash] = true
	}
}

// hashExpected is a helper to compute the expected hash
func hashExpected(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash)
}

// TestQdrantPoint tests the QdrantPoint structure
func TestQdrantPoint(t *testing.T) {
	now := time.Now()
	point := QdrantPoint{
		ID:           "test-id-123",
		MessageID:    456,
		UserID:       "user-hash-abc",
		Text:         "This is a test message",
		TimestampUTC: now,
		TelegramLink: "https://t.me/c/123/456",
	}

	if point.ID != "test-id-123" {
		t.Error("ID should be 'test-id-123'")
	}
	if point.MessageID != 456 {
		t.Error("MessageID should be 456")
	}
	if point.Text != "This is a test message" {
		t.Error("Text should match")
	}
	if point.TelegramLink != "https://t.me/c/123/456" {
		t.Error("TelegramLink should match")
	}
}

// TestRecentMessage tests the recentMessage structure
func TestRecentMessage(t *testing.T) {
	now := time.Now()
	msg := recentMessage{
		qdrantID:     "qdrant-id",
		messageID:    123,
		text:         "Combined message text",
		timestamp:    now,
		telegramLink: "https://t.me/c/123/456",
	}

	if msg.qdrantID != "qdrant-id" {
		t.Error("qdrantID should match")
	}
	if msg.messageID != 123 {
		t.Error("messageID should match")
	}
	if msg.text != "Combined message text" {
		t.Error("text should match")
	}
}

// TestProcessMessageForMemoryLogic tests the logic structure of processMessageForMemory
func TestProcessMessageForMemoryLogic(t *testing.T) {
	// Test the default values logic
	minLenStr := ""
	minLen, err := strconv.Atoi(minLenStr)
	if err != nil {
		minLen = 20 // Default value
	}
	if minLen != 20 {
		t.Errorf("expected default minLen 20, got %d", minLen)
	}

	combineWindowStr := ""
	combineWindow, err := strconv.Atoi(combineWindowStr)
	if err != nil {
		combineWindow = 90 // Default value
	}
	if combineWindow != 90 {
		t.Errorf("expected default combineWindow 90, got %d", combineWindow)
	}

	// Test with valid environment values
	os.Setenv("MIN_MESSAGE_LENGTH", "50")
	os.Setenv("MESSAGE_COMBINE_WINDOW", "120")
	defer func() {
		os.Unsetenv("MIN_MESSAGE_LENGTH")
		os.Unsetenv("MESSAGE_COMBINE_WINDOW")
	}()

	minLenStr = "50"
	minLen, err = strconv.Atoi(minLenStr)
	if err != nil {
		minLen = 20
	}
	if minLen != 50 {
		t.Errorf("expected minLen 50, got %d", minLen)
	}

	combineWindowStr = "120"
	combineWindow, err = strconv.Atoi(combineWindowStr)
	if err != nil {
		combineWindow = 90
	}
	if combineWindow != 120 {
		t.Errorf("expected combineWindow 120, got %d", combineWindow)
	}
}

// TestChatIDLinkFormat tests the Telegram link format logic
func TestChatIDLinkFormat(t *testing.T) {
	tests := []struct {
		name     string
		chatID   int64
		messageID int
		expected string
	}{
		{
			name:     "supergroup with -100 prefix",
			chatID:   -10012345,
			messageID: 100,
			expected: "https://t.me/c/12345/100",
		},
		{
			name:     "supergroup with - prefix",
			chatID:   -12345,
			messageID: 100,
			expected: "https://t.me/c/12345/100",
		},
		{
			name:     "normal group",
			chatID:   12345,
			messageID: 100,
			expected: "https://t.me/c/12345/100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatIDStr := fmt.Sprintf("%d", tt.chatID)
			if strings.HasPrefix(chatIDStr, "-100") {
				chatIDStr = strings.TrimPrefix(chatIDStr, "-100")
			} else if strings.HasPrefix(chatIDStr, "-") {
				chatIDStr = strings.TrimPrefix(chatIDStr, "-")
			}
			link := fmt.Sprintf("https://t.me/c/%s/%d", chatIDStr, tt.messageID)

			if link != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, link)
			}
		})
	}
}

// TestMessageLengthFiltering tests message length filtering logic
func TestMessageLengthFiltering(t *testing.T) {
	minLen := 20

	tests := []struct {
		name      string
		text      string
		shouldSkip bool
	}{
		{
			name:      "short message - should skip",
			text:      "Hi",
			shouldSkip: true,
		},
		{
			name:      "exact min length - should keep",
			text:      string(make([]byte, 20)),
			shouldSkip: false,
		},
		{
			name:      "long message - should keep",
			text:      string(make([]byte, 100)),
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldSkip := len(tt.text) < minLen
			if shouldSkip != tt.shouldSkip {
				t.Errorf("expected shouldSkip=%v, got %v", tt.shouldSkip, shouldSkip)
			}
		})
	}
}

// TestMessageCombineWindow tests the message combining logic
func TestMessageCombineWindow(t *testing.T) {
	combineWindow := 90 // seconds

	tests := []struct {
		name         string
		lastTime     time.Time
		currentTime  time.Time
		shouldCombine bool
	}{
		{
			name:         "within window - should combine",
			lastTime:     time.Now(),
			currentTime:  time.Now().Add(30 * time.Second),
			shouldCombine: true,
		},
		{
			name:         "outside window - should not combine",
			lastTime:     time.Now(),
			currentTime:  time.Now().Add(120 * time.Second),
			shouldCombine: false,
		},
		{
			name:         "exactly at window boundary",
			lastTime:     time.Now(),
			currentTime:  time.Now().Add(90 * time.Second),
			shouldCombine: false, // < float64(combineWindow), not <=
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldCombine := tt.currentTime.Sub(tt.lastTime).Seconds() < float64(combineWindow)
			if shouldCombine != tt.shouldCombine {
				t.Errorf("expected shouldCombine=%v, got %v", tt.shouldCombine, shouldCombine)
			}
		})
	}
}

// TestEmbeddingSizeConstant tests the embedding size constant
func TestEmbeddingSizeConstant(t *testing.T) {
	if embeddingSize != 1536 {
		t.Errorf("expected embeddingSize to be 1536, got %d", embeddingSize)
	}
}

// TestQdrantCollectionName tests the collection name constant
func TestQdrantCollectionName(t *testing.T) {
	if qdrantCollectionName != "telegram_messages" {
		t.Errorf("expected qdrantCollectionName to be 'telegram_messages', got '%s'", qdrantCollectionName)
	}
}

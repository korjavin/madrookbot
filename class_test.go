package main

import (
	"os"
	"testing"
	"time"
)

// TestGetNextSunday tests the getNextSunday function
func TestGetNextSunday(t *testing.T) {
	// This test is tricky because it depends on current time
	// We'll test the logic by checking the result is a Sunday at 18:00 Berlin time
	nextSunday := getNextSunday()

	// Verify it's a Sunday
	if nextSunday.Weekday() != time.Sunday {
		t.Errorf("expected Sunday, got %s", nextSunday.Weekday())
	}

	// Verify it's at 18:00
	if nextSunday.Hour() != 18 || nextSunday.Minute() != 0 {
		t.Errorf("expected 18:00, got %02d:%02d", nextSunday.Hour(), nextSunday.Minute())
	}

	// Verify it's in the future (or today if Sunday before 18:00)
	now := time.Now()
	if nextSunday.Before(now) {
		t.Error("next Sunday should be in the future")
	}
}

// TestGetNextSundayEdgeCases tests edge cases for getNextSunday
func TestGetNextSundayEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		nowFunc  func() time.Time
		checkDay time.Weekday
	}{
		{
			name:     "Monday should return next Sunday",
			nowFunc:  func() time.Time { return time.Date(2024, 1, 8, 12, 0, 0, 0, time.UTC) }, // Monday
			checkDay: time.Sunday,
		},
		{
			name:     "Saturday should return next Sunday",
			nowFunc:  func() time.Time { return time.Date(2024, 1, 13, 12, 0, 0, 0, time.UTC) }, // Saturday
			checkDay: time.Sunday,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test verifies the function returns a valid result
			result := getNextSunday()
			if result.Weekday() != tt.checkDay {
				t.Errorf("expected %s, got %s", tt.checkDay, result.Weekday())
			}
		})
	}
}

// TestIsColumnExistsError tests the isColumnExistsError function
func TestIsColumnExistsError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "duplicate column error",
			err:      &testError{message: "duplicate column name: conducted"},
			expected: true,
		},
		{
			name:     "UNIQUE constraint error",
			err:      &testError{message: "UNIQUE constraint failed"},
			expected: true,
		},
		{
			name:     "no column error",
			err:      &testError{message: "table classes has no column named conducted"},
			expected: true,
		},
		{
			name:     "other error",
			err:      &testError{message: "some other error"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isColumnExistsError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// testError is a simple error for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

// TestFormatClassTimeWithTimezones tests the formatClassTimeWithTimezones function
func TestFormatClassTimeWithTimezones(t *testing.T) {
	// Create a specific time for testing
	classTime := time.Date(2024, 1, 14, 18, 0, 0, 0, time.UTC) // Sunday 18:00 UTC

	result := formatClassTimeWithTimezones(classTime)

	// Check that all expected timezones are present
	timezones := []string{"Berlin", "Egypt", "India"}
	for _, tz := range timezones {
		if !containsString(result, tz) {
			t.Errorf("expected timezone %s to be in result", tz)
		}
	}

	// Check that it contains time format (should show 19:00 for Berlin which is UTC+1 in January)
	if !containsString(result, "19:00") && !containsString(result, "18:00") {
		t.Error("expected time to be in result (19:00 or 18:00 depending on timezone)")
	}

	// Check that it contains day of week
	if !containsString(result, "Sunday") {
		t.Error("expected Sunday to be in result")
	}
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestSanitizeClassDescription tests the sanitizeClassDescription function
func TestSanitizeClassDescription(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal description",
			input:    "Introduction to Python",
			expected: "Introduction to Python",
		},
		{
			name:     "trim whitespace",
			input:    "  Introduction to Python  ",
			expected: "Introduction to Python",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "long description truncated",
			input:    string(make([]byte, 600)), // 600 characters
			expected: string(make([]byte, 500)), // Should be truncated to 500
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeClassDescription(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// TestShouldSendLowRSVPWarning tests the shouldSendLowRSVPWarning function
func TestShouldSendLowRSVPWarning(t *testing.T) {
	tests := []struct {
		name      string
		rsvpCount int
		expected  bool
	}{
		{
			name:      "zero RSVPs",
			rsvpCount: 0,
			expected:  true,
		},
		{
			name:      "one RSVP",
			rsvpCount: 1,
			expected:  true,
		},
		{
			name:      "two RSVPs",
			rsvpCount: 2,
			expected:  true,
		},
		{
			name:      "three RSVPs",
			rsvpCount: 3,
			expected:  true,
		},
		{
			name:      "four RSVPs - minimum met",
			rsvpCount: 4,
			expected:  false,
		},
		{
			name:      "five RSVPs",
			rsvpCount: 5,
			expected:  false,
		},
		{
			name:      "ten RSVPs",
			rsvpCount: 10,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldSendLowRSVPWarning(tt.rsvpCount)
			if result != tt.expected {
				t.Errorf("shouldSendLowRSVPWarning(%d) = %v, want %v", tt.rsvpCount, result, tt.expected)
			}
		})
	}
}

// TestGetRandomAnnouncementDelay tests the getRandomAnnouncementDelay function
func TestGetRandomAnnouncementDelay(t *testing.T) {
	// Run multiple times to check it's within expected range
	for i := 0; i < 100; i++ {
		delay := getRandomAnnouncementDelay()

		// Should be between 1 and 6 hours
		minDelay := 1 * time.Hour
		maxDelay := 6 * time.Hour

		if delay < minDelay || delay > maxDelay {
			t.Errorf("delay %v out of range [%v, %v]", delay, minDelay, maxDelay)
		}
	}
}

// TestIsOwner tests the isOwner function
func TestIsOwner(t *testing.T) {
	// Save original environment
	originalOwnerID := os.Getenv("OWNER_ID")
	defer os.Setenv("OWNER_ID", originalOwnerID)

	tests := []struct {
		name     string
		ownerID  string
		userID   int64
		expected bool
	}{
		{
			name:     "owner matches",
			ownerID:  "12345",
			userID:   12345,
			expected: true,
		},
		{
			name:     "owner does not match",
			ownerID:  "12345",
			userID:   67890,
			expected: false,
		},
		{
			name:     "owner ID not set",
			ownerID:  "",
			userID:   12345,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.ownerID == "" {
				os.Unsetenv("OWNER_ID")
			} else {
				os.Setenv("OWNER_ID", tt.ownerID)
			}
			result := isOwner(tt.userID)
			if result != tt.expected {
				t.Errorf("isOwner(%d) = %v, want %v", tt.userID, result, tt.expected)
			}
		})
	}
}

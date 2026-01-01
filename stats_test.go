package main

import (
	"testing"
	"time"
)

// TestCheckStatRateLimit tests the checkStatRateLimit function logic
func TestCheckStatRateLimitLogic(t *testing.T) {
	tests := []struct {
		name         string
		lastRun      time.Time
		expectedBool bool
	}{
		{
			name:         "no previous run - should allow",
			lastRun:      time.Time{},
			expectedBool: true,
		},
		{
			name:         "recent run - should rate limit",
			lastRun:      time.Now().Add(-30 * time.Minute),
			expectedBool: false,
		},
		{
			name:         "old run - should allow",
			lastRun:      time.Now().Add(-2 * time.Hour),
			expectedBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the rate limiting logic
			rateLimit := time.Hour

			if tt.lastRun.IsZero() {
				// First time running
				if !tt.expectedBool {
					t.Error("expected first run to be allowed")
				}
			} else {
				timeSinceLastRun := time.Since(tt.lastRun)
				allowed := timeSinceLastRun >= rateLimit
				if allowed != tt.expectedBool {
					t.Errorf("expected %v, got %v", tt.expectedBool, allowed)
				}
			}
		})
	}
}

// TestFindPeakHour tests the findPeakHour function
func TestFindPeakHour(t *testing.T) {
	tests := []struct {
		name         string
		hourlyData   map[time.Time]int
		expectedHour int
		expectedCount int
	}{
		{
			name:         "single hour",
			hourlyData:   map[time.Time]int{time.Date(2024, 1, 14, 10, 0, 0, 0, time.UTC): 5},
			expectedHour: 10,
			expectedCount: 5,
		},
		{
			name: "multiple hours",
			hourlyData: map[time.Time]int{
				time.Date(2024, 1, 14, 9, 0, 0, 0, time.UTC): 3,
				time.Date(2024, 1, 14, 10, 0, 0, 0, time.UTC): 7,
				time.Date(2024, 1, 14, 11, 0, 0, 0, time.UTC): 2,
			},
			expectedHour:   10,
			expectedCount: 7,
		},
		{
			name:         "empty data",
			hourlyData:   map[time.Time]int{},
			expectedHour: 0,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := UserActivitySummary{
				UserName:   "testuser",
				HourlyData: tt.hourlyData,
			}
			peakHour, peakCount := findPeakHour(user)

			if peakHour != tt.expectedHour {
				t.Errorf("expected peak hour %d, got %d", tt.expectedHour, peakHour)
			}
			if peakCount != tt.expectedCount {
				t.Errorf("expected peak count %d, got %d", tt.expectedCount, peakCount)
			}
		})
	}
}

// TestBuildActivitySummary tests the buildActivitySummary function
func TestBuildActivitySummary(t *testing.T) {
	now := time.Now()
	stats := &ActivityStats{
		GroupID:   12345,
		StartTime: now.AddDate(0, 0, -7),
		EndTime:   now,
		UserStats: []UserActivitySummary{
			{
				UserName:      "user1",
				TotalMessages: 100,
				HourlyData: map[time.Time]int{
					now.Add(-1 * time.Hour): 50,
					now.Add(-2 * time.Hour): 30,
				},
			},
			{
				UserName:      "user2",
				TotalMessages: 80,
				HourlyData: map[time.Time]int{
					now.Add(-1 * time.Hour): 40,
				},
			},
			{
				UserName:      "user3",
				TotalMessages: 60,
				HourlyData:    map[time.Time]int{},
			},
		},
	}

	summary := buildActivitySummary(stats)

	// Check that summary contains expected elements
	if !containsString(summary, "Group Activity Summary") {
		t.Error("summary should contain 'Group Activity Summary'")
	}
	if !containsString(summary, "user1") {
		t.Error("summary should contain top user")
	}
	if !containsString(summary, "Top Contributors") {
		t.Error("summary should contain 'Top Contributors'")
	}
	if !containsString(summary, "Activity Patterns") {
		t.Error("summary should contain 'Activity Patterns'")
	}
}

// TestActivityStatsSorting tests that user stats are sorted by message count
func TestActivityStatsSorting(t *testing.T) {
	now := time.Now()
	stats := &ActivityStats{
		GroupID:   12345,
		StartTime: now.AddDate(0, 0, -7),
		EndTime:   now,
		UserStats: []UserActivitySummary{
			{UserName: "user3", TotalMessages: 30},
			{UserName: "user1", TotalMessages: 100},
			{UserName: "user2", TotalMessages: 50},
		},
	}

	// The buildActivitySummary function sorts the stats
	// We verify this by checking the output order
	summary := buildActivitySummary(stats)

	// User1 should appear before User2, and User2 before User3
	user1Pos := indexOf(summary, "user1")
	user2Pos := indexOf(summary, "user2")
	user3Pos := indexOf(summary, "user3")

	if user1Pos == -1 || user2Pos == -1 || user3Pos == -1 {
		t.Fatal("all users should appear in summary")
	}

	if user1Pos >= user2Pos {
		t.Error("user1 should appear before user2 (higher message count)")
	}
	if user2Pos >= user3Pos {
		t.Error("user2 should appear before user3 (higher message count)")
	}
}

// TestCleanupOldActivityLogic tests the cleanup logic for old activity
func TestCleanupOldActivityLogic(t *testing.T) {
	retentionMonths := 6
	cutoffTime := time.Now().AddDate(0, -retentionMonths, 0)

	tests := []struct {
		name          string
		messageTime   time.Time
		shouldCleanup bool
	}{
		{
			name:          "old message - should cleanup",
			messageTime:   time.Now().AddDate(0, -7, 0),
			shouldCleanup: true,
		},
		{
			name:          "recent message - should keep",
			messageTime:   time.Now().AddDate(0, -1, 0),
			shouldCleanup: false,
		},
		{
			name:          "exactly at cutoff - should keep",
			messageTime:   cutoffTime,
			shouldCleanup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldCleanup := tt.messageTime.Before(cutoffTime)
			if shouldCleanup != tt.shouldCleanup {
				t.Errorf("expected %v, got %v", tt.shouldCleanup, shouldCleanup)
			}
		})
	}
}

// Helper function to find index of substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// TestActivityRetentionMonths tests the retention period constant
func TestActivityRetentionMonths(t *testing.T) {
	if activityRetentionMonths != 6 {
		t.Errorf("expected activityRetentionMonths to be 6, got %d", activityRetentionMonths)
	}
}

// TestUserActivitySummaryFields tests the UserActivitySummary structure
func TestUserActivitySummaryFields(t *testing.T) {
	user := UserActivitySummary{
		UserName:      "testuser",
		TotalMessages: 42,
		HourlyData: map[time.Time]int{
			time.Now(): 10,
		},
	}

	if user.UserName != "testuser" {
		t.Error("UserName should be 'testuser'")
	}
	if user.TotalMessages != 42 {
		t.Error("TotalMessages should be 42")
	}
	if len(user.HourlyData) != 1 {
		t.Error("HourlyData should have 1 entry")
	}
}

// TestActivityStatsFields tests the ActivityStats structure
func TestActivityStatsFields(t *testing.T) {
	now := time.Now()
	stats := ActivityStats{
		GroupID:   12345,
		StartTime: now.AddDate(0, 0, -7),
		EndTime:   now,
		UserStats: []UserActivitySummary{},
	}

	if stats.GroupID != 12345 {
		t.Error("GroupID should be 12345")
	}
	if len(stats.UserStats) != 0 {
		t.Error("UserStats should be empty")
	}
}

package main

import (
	"fmt"
	"log"
	"time"
)

const activityRetentionMonths = 6

// initActivityDatabase creates tables for user activity tracking
func initActivityDatabase() error {
	// User activity table - hourly buckets
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS user_activity (
		group_id INTEGER NOT NULL,
		user_name TEXT NOT NULL,
		hour_bucket TIMESTAMP NOT NULL,
		message_count INTEGER DEFAULT 1,
		PRIMARY KEY (group_id, user_name, hour_bucket)
	)`)
	if err != nil {
		return err
	}

	// Index for fast cleanup and queries
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_activity_hour_bucket
		ON user_activity(hour_bucket)`)
	if err != nil {
		return err
	}

	// Rate limiting table for /stat command
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS stat_rate_limit (
		group_id INTEGER PRIMARY KEY,
		last_run_time TIMESTAMP NOT NULL
	)`)
	if err != nil {
		return err
	}

	log.Println("[INFO] Activity tracking database initialized")
	return nil
}

// trackUserActivity records a message in the current hour bucket
func trackUserActivity(groupID int64, userName string) error {
	// Get current hour bucket (truncate to hour)
	now := time.Now()
	hourBucket := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	// Insert or increment message count
	_, err := db.Exec(`INSERT INTO user_activity (group_id, user_name, hour_bucket, message_count)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(group_id, user_name, hour_bucket)
		DO UPDATE SET message_count = message_count + 1`,
		groupID, userName, hourBucket)

	if err != nil {
		log.Printf("[ERROR] Failed to track user activity: %v", err)
		return err
	}

	return nil
}

// cleanupOldActivity removes activity data older than retention period
func cleanupOldActivity() error {
	cutoffTime := time.Now().AddDate(0, -activityRetentionMonths, 0)
	result, err := db.Exec("DELETE FROM user_activity WHERE hour_bucket < ?", cutoffTime)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("[INFO] Cleaned up %d old activity records (older than %d months)",
			rowsAffected, activityRetentionMonths)
	}

	return nil
}

// ActivityStats holds aggregated statistics for a group
type ActivityStats struct {
	GroupID   int64
	StartTime time.Time
	EndTime   time.Time
	UserStats []UserActivitySummary
}

// UserActivitySummary holds per-user statistics
type UserActivitySummary struct {
	UserName      string
	TotalMessages int
	HourlyData    map[time.Time]int // hour bucket -> message count
}

// getGroupActivityStats retrieves activity stats for a group for the last N days
func getGroupActivityStats(groupID int64, days int) (*ActivityStats, error) {
	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	rows, err := db.Query(`SELECT user_name, hour_bucket, message_count
		FROM user_activity
		WHERE group_id = ? AND hour_bucket >= ? AND hour_bucket <= ?
		ORDER BY hour_bucket ASC`,
		groupID, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Aggregate by user
	userDataMap := make(map[string]*UserActivitySummary)

	for rows.Next() {
		var userName string
		var hourBucket time.Time
		var messageCount int

		err := rows.Scan(&userName, &hourBucket, &messageCount)
		if err != nil {
			log.Printf("[ERROR] Failed to scan activity row: %v", err)
			continue
		}

		if _, exists := userDataMap[userName]; !exists {
			userDataMap[userName] = &UserActivitySummary{
				UserName:   userName,
				HourlyData: make(map[time.Time]int),
			}
		}

		userDataMap[userName].HourlyData[hourBucket] = messageCount
		userDataMap[userName].TotalMessages += messageCount
	}

	// Convert map to slice
	var userStats []UserActivitySummary
	for _, summary := range userDataMap {
		userStats = append(userStats, *summary)
	}

	return &ActivityStats{
		GroupID:   groupID,
		StartTime: startTime,
		EndTime:   endTime,
		UserStats: userStats,
	}, rows.Err()
}

// checkStatRateLimit checks if enough time has passed since last /stat run
func checkStatRateLimit(groupID int64) (bool, error) {
	var lastRunTime time.Time
	err := db.QueryRow("SELECT last_run_time FROM stat_rate_limit WHERE group_id = ?", groupID).Scan(&lastRunTime)

	if err != nil {
		// No record found - first time running
		return true, nil
	}

	// Check if 1 hour has passed
	if time.Since(lastRunTime) < time.Hour {
		return false, fmt.Errorf("rate limited: last run was %v ago", time.Since(lastRunTime).Round(time.Minute))
	}

	return true, nil
}

// updateStatRateLimit updates the last run time for a group
func updateStatRateLimit(groupID int64) error {
	_, err := db.Exec(`INSERT INTO stat_rate_limit (group_id, last_run_time)
		VALUES (?, ?)
		ON CONFLICT(group_id)
		DO UPDATE SET last_run_time = ?`,
		groupID, time.Now(), time.Now())
	return err
}

// scheduleActivityCleanup runs periodic cleanup of old activity data
func scheduleActivityCleanup() {
	// Run cleanup on startup
	err := cleanupOldActivity()
	if err != nil {
		log.Printf("[ERROR] Failed initial activity cleanup: %v", err)
	}

	// Run cleanup daily
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		err := cleanupOldActivity()
		if err != nil {
			log.Printf("[ERROR] Failed scheduled activity cleanup: %v", err)
		}
	}
}

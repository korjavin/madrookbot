package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// Class represents a scheduled class
type Class struct {
	ID                        int
	Description               string
	ScheduledTime             time.Time
	AnnouncementMessageID     int
	AnnouncementPosted        bool
	Reminder6hSent            bool
	Reminder1hSent            bool
	Unpinned                  bool
	Conducted                 bool
	CreatedAt                 time.Time
	Cancelled                 bool
	QuestionsRequested        bool
	QuestionsRequestMessageID int
}

// isColumnExistsError checks if the error is due to column already existing
func isColumnExistsError(err error) bool {
	return err != nil && (err.Error() == "duplicate column name: conducted" ||
		err.Error() == "UNIQUE constraint failed" ||
		err.Error() == "table classes has no column named conducted")
}

// initClassesTable creates the classes table if it doesn't exist
func initClassesTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS classes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		description TEXT NOT NULL,
		scheduled_time DATETIME NOT NULL,
		announcement_message_id INTEGER DEFAULT 0,
		announcement_posted BOOLEAN DEFAULT 0,
		reminder_6h_sent BOOLEAN DEFAULT 0,
		reminder_1h_sent BOOLEAN DEFAULT 0,
		unpinned BOOLEAN DEFAULT 0,
		conducted BOOLEAN DEFAULT 0,
		created_at DATETIME NOT NULL,
		cancelled BOOLEAN DEFAULT 0
	)`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create classes table: %v", err)
	}

	// Add conducted column if it doesn't exist (migration)
	migrationQuery := `
		ALTER TABLE classes ADD COLUMN conducted BOOLEAN DEFAULT 0
	`
	_, err = db.Exec(migrationQuery)
	if err != nil && !isColumnExistsError(err) {
		return fmt.Errorf("failed to add conducted column: %v", err)
	}

	// Add questions_requested column if it doesn't exist
	migrationQuery2 := `
		ALTER TABLE classes ADD COLUMN questions_requested BOOLEAN DEFAULT 0
	`
	_, err = db.Exec(migrationQuery2)
	if err != nil && !isColumnExistsError(err) {
		return fmt.Errorf("failed to add questions_requested column: %v", err)
	}

	// Add questions_request_message_id column if it doesn't exist
	migrationQuery3 := `
		ALTER TABLE classes ADD COLUMN questions_request_message_id INTEGER DEFAULT 0
	`
	_, err = db.Exec(migrationQuery3)
	if err != nil && !isColumnExistsError(err) {
		return fmt.Errorf("failed to add questions_request_message_id column: %v", err)
	}

	// Create RSVPs table
	rsvpQuery := `
	CREATE TABLE IF NOT EXISTS class_rsvps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		class_id INTEGER NOT NULL,
		message_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		username TEXT,
		reacted_at DATETIME NOT NULL,
		UNIQUE(class_id, message_id, user_id),
		FOREIGN KEY(class_id) REFERENCES classes(id) ON DELETE CASCADE
	)`

	_, err = db.Exec(rsvpQuery)
	if err != nil {
		return fmt.Errorf("failed to create class_rsvps table: %v", err)
	}

	log.Printf("[INFO] Classes and RSVPs tables initialized")
	return nil
}

// getNextSunday returns the next Sunday at 18:00 Berlin time
// If today is Sunday and it's before 18:00, returns today at 18:00
func getNextSunday() time.Time {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.Printf("[ERROR] Failed to load Berlin timezone: %v", err)
		berlin = time.UTC
	}

	now := time.Now().In(berlin)

	// Set to 18:00 today
	classTime := time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, berlin)

	// If today is Sunday and it's before 18:00, use today
	if now.Weekday() == time.Sunday && now.Before(classTime) {
		return classTime
	}

	// Otherwise, find next Sunday
	daysUntilSunday := (7 - int(now.Weekday())) % 7
	if daysUntilSunday == 0 {
		daysUntilSunday = 7
	}

	nextSunday := now.AddDate(0, 0, daysUntilSunday)
	return time.Date(nextSunday.Year(), nextSunday.Month(), nextSunday.Day(), 18, 0, 0, 0, berlin)
}

// createClass creates a new class and stores it in the database
func createClass(description string) (*Class, error) {
	scheduledTime := getNextSunday()
	now := time.Now()

	query := `
		INSERT INTO classes (description, scheduled_time, created_at, cancelled)
		VALUES (?, ?, ?, 0)
	`

	result, err := db.Exec(query, description, scheduledTime, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert class: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get class ID: %v", err)
	}

	class := &Class{
		ID:            int(id),
		Description:   description,
		ScheduledTime: scheduledTime,
		CreatedAt:     now,
		Cancelled:     false,
	}

	log.Printf("[CLASS] Created class ID=%d, description='%s', scheduled=%s",
		id, description, scheduledTime.Format("2006-01-02 15:04 MST"))

	return class, nil
}

// getActiveClass returns the current active (non-cancelled, non-conducted) class
func getActiveClass() (*Class, error) {
	query := `
		SELECT id, description, scheduled_time, announcement_message_id,
		       announcement_posted, reminder_6h_sent, reminder_1h_sent,
		       unpinned, conducted, created_at, cancelled,
		       questions_requested, questions_request_message_id
		FROM classes
		WHERE cancelled = 0 AND conducted = 0
		ORDER BY scheduled_time ASC
		LIMIT 1
	`

	var class Class
	err := db.QueryRow(query).Scan(
		&class.ID,
		&class.Description,
		&class.ScheduledTime,
		&class.AnnouncementMessageID,
		&class.AnnouncementPosted,
		&class.Reminder6hSent,
		&class.Reminder1hSent,
		&class.Unpinned,
		&class.Conducted,
		&class.CreatedAt,
		&class.Cancelled,
		&class.QuestionsRequested,
		&class.QuestionsRequestMessageID,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get active class: %v", err)
	}

	return &class, nil
}

// cancelClass marks a class as cancelled
func cancelClass(classID int) error {
	query := `UPDATE classes SET cancelled = 1 WHERE id = ?`
	_, err := db.Exec(query, classID)
	if err != nil {
		return fmt.Errorf("failed to cancel class: %v", err)
	}

	log.Printf("[CLASS] Cancelled class ID=%d", classID)
	return nil
}

// updateClassAnnouncementPosted marks the announcement as posted
func updateClassAnnouncementPosted(classID int, messageID int) error {
	query := `UPDATE classes SET announcement_posted = 1, announcement_message_id = ? WHERE id = ?`
	_, err := db.Exec(query, messageID, classID)
	if err != nil {
		return fmt.Errorf("failed to update announcement posted: %v", err)
	}

	log.Printf("[CLASS] Marked announcement posted for class ID=%d, messageID=%d", classID, messageID)
	return nil
}

// updateClassReminder6hSent marks the 6h reminder as sent
func updateClassReminder6hSent(classID int) error {
	query := `UPDATE classes SET reminder_6h_sent = 1 WHERE id = ?`
	_, err := db.Exec(query, classID)
	if err != nil {
		return fmt.Errorf("failed to update 6h reminder: %v", err)
	}

	log.Printf("[CLASS] Marked 6h reminder sent for class ID=%d", classID)
	return nil
}

// updateClassReminder1hSent marks the 1h reminder as sent
func updateClassReminder1hSent(classID int) error {
	query := `UPDATE classes SET reminder_1h_sent = 1 WHERE id = ?`
	_, err := db.Exec(query, classID)
	if err != nil {
		return fmt.Errorf("failed to update 1h reminder: %v", err)
	}

	log.Printf("[CLASS] Marked 1h reminder sent for class ID=%d", classID)
	return nil
}

// updateClassUnpinned marks the class message as unpinned
func updateClassUnpinned(classID int) error {
	query := `UPDATE classes SET unpinned = 1 WHERE id = ?`
	_, err := db.Exec(query, classID)
	if err != nil {
		return fmt.Errorf("failed to update unpinned status: %v", err)
	}

	log.Printf("[CLASS] Marked class unpinned for ID=%d", classID)
	return nil
}

// updateClassConducted marks the class as conducted (finished)
func updateClassConducted(classID int) error {
	query := `UPDATE classes SET conducted = 1 WHERE id = ?`
	_, err := db.Exec(query, classID)
	if err != nil {
		return fmt.Errorf("failed to update conducted status: %v", err)
	}

	log.Printf("[CLASS] Marked class conducted for ID=%d", classID)
	return nil
}

// updateClassQuestionsRequestSent marks the questions request as sent
func updateClassQuestionsRequestSent(classID int, messageID int) error {
	query := `UPDATE classes SET questions_requested = 1, questions_request_message_id = ? WHERE id = ?`
	_, err := db.Exec(query, messageID, classID)
	if err != nil {
		return fmt.Errorf("failed to update questions request status: %v", err)
	}

	log.Printf("[CLASS] Marked questions request sent for class ID=%d, messageID=%d", classID, messageID)
	return nil
}

// getClassByQuestionsRequestMessageID returns the class associated with a questions request message ID
func getClassByQuestionsRequestMessageID(messageID int) (*Class, error) {
	query := `
		SELECT id, description, scheduled_time, announcement_message_id,
		       announcement_posted, reminder_6h_sent, reminder_1h_sent,
		       unpinned, conducted, created_at, cancelled,
		       questions_requested, questions_request_message_id
		FROM classes
		WHERE questions_request_message_id = ?
	`

	var class Class
	err := db.QueryRow(query, messageID).Scan(
		&class.ID,
		&class.Description,
		&class.ScheduledTime,
		&class.AnnouncementMessageID,
		&class.AnnouncementPosted,
		&class.Reminder6hSent,
		&class.Reminder1hSent,
		&class.Unpinned,
		&class.Conducted,
		&class.CreatedAt,
		&class.Cancelled,
		&class.QuestionsRequested,
		&class.QuestionsRequestMessageID,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get class by questions request message ID: %v", err)
	}

	return &class, nil
}

// formatClassTimeWithTimezones formats the class time in multiple timezones
func formatClassTimeWithTimezones(classTime time.Time) string {
	timezones := map[string]string{
		"Berlin": "Europe/Berlin",
		"Egypt":  "Africa/Cairo",
		"India":  "Asia/Kolkata",
	}

	var result string
	for name, tz := range timezones {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			log.Printf("[WARN] Failed to load timezone %s: %v", tz, err)
			continue
		}
		localTime := classTime.In(loc)
		result += fmt.Sprintf("🕐 %s: %s\n", name, localTime.Format("Monday, January 2 at 15:04 MST"))
	}

	return result
}

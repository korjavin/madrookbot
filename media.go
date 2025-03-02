package main

import (
	"database/sql"

	"fmt"

	"regexp"
	"strings"
	"time"
)

// MediaSuggestion represents a suggested media item
type MediaSuggestion struct {
	ID          int64     `json:"id"`
	URL         string    `json:"url"`
	Suggester   string    `json:"suggester"`
	SuggestedAt time.Time `json:"suggested_at"`
	WeeksActive int       `json:"weeks_active"`
	Selected    bool      `json:"selected"`
}

// Initialize media suggestions table
func initMediaSuggestions() error {
	// Create media suggestions table if it doesn't exist
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS media_suggestions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		suggester TEXT NOT NULL,
		suggested_at TIMESTAMP NOT NULL,
		weeks_active INTEGER DEFAULT 1,
		selected BOOLEAN DEFAULT FALSE
	)`)

	return err
}

// AddMediaSuggestion adds a new media suggestion to the database
func AddMediaSuggestion(url, suggester string) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO media_suggestions (url, suggester, suggested_at) VALUES (?, ?, ?)",
		url, suggester, time.Now())
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// GetMediaSuggestions returns all media suggestions that haven't been selected yet
func GetMediaSuggestions() ([]MediaSuggestion, error) {
	rows, err := db.Query(`
		SELECT id, url, suggester, suggested_at, weeks_active, selected 
		FROM media_suggestions 
		WHERE selected = FALSE
		ORDER BY suggested_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	suggestions := []MediaSuggestion{}
	for rows.Next() {
		var suggestion MediaSuggestion
		var suggestedAt string
		err = rows.Scan(
			&suggestion.ID,
			&suggestion.URL,
			&suggestion.Suggester,
			&suggestedAt,
			&suggestion.WeeksActive,
			&suggestion.Selected,
		)
		if err != nil {
			return nil, err
		}

		// Parse the timestamp
		suggestion.SuggestedAt, _ = time.Parse(time.RFC3339, suggestedAt)
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

// GetSelectedMedia returns the currently selected media suggestion
func GetSelectedMedia() (MediaSuggestion, error) {
	var suggestion MediaSuggestion
	var suggestedAt string

	err := db.QueryRow(`
		SELECT id, url, suggester, suggested_at, weeks_active, selected 
		FROM media_suggestions 
		WHERE selected = TRUE
		ORDER BY id DESC
		LIMIT 1
	`).Scan(
		&suggestion.ID,
		&suggestion.URL,
		&suggestion.Suggester,
		&suggestedAt,
		&suggestion.WeeksActive,
		&suggestion.Selected,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return MediaSuggestion{}, nil
		}
		return MediaSuggestion{}, err
	}

	// Parse the timestamp
	suggestion.SuggestedAt, _ = time.Parse(time.RFC3339, suggestedAt)

	return suggestion, nil
}

// SelectMedia marks a media suggestion as selected
func SelectMedia(id int64) error {
	_, err := db.Exec("UPDATE media_suggestions SET selected = TRUE WHERE id = ?", id)
	return err
}

// ProcessWeeklyUpdate processes weekly updates for media suggestions
// - Increments weeks_active counter for unselected items
// - Removes suggestions older than 6 weeks
// - Resets selected status for the next week
func ProcessWeeklyUpdate() error {
	// First, increment weeks_active for all unselected items
	_, err := db.Exec("UPDATE media_suggestions SET weeks_active = weeks_active + 1 WHERE selected = FALSE")
	if err != nil {
		return err
	}

	// Remove suggestions older than 6 weeks
	_, err = db.Exec("DELETE FROM media_suggestions WHERE weeks_active > 6 AND selected = FALSE")
	if err != nil {
		return err
	}

	return nil
}

// Constants for suggestion detection
var (
	// Keywords that might indicate a suggestion
	suggestionKeywords = []string{
		"suggest", "suggestion", "what about", "how about", "consider", "check out",
		"look at", "watch", "read", "listen to", "worth", "interesting", "media",
	}

	// URL extraction regex
	urlRegex = regexp.MustCompile(`https?://\S+`)
)

// ExtractSuggestionFromMessage attempts to extract a URL suggestion from a message
func ExtractSuggestionFromMessage(message string) string {
	// Convert to lowercase for case-insensitive matching
	messageLower := strings.ToLower(message)

	// Check if message contains any suggestion keywords
	hasSuggestionKeyword := false
	for _, keyword := range suggestionKeywords {
		if strings.Contains(messageLower, keyword) {
			hasSuggestionKeyword = true
			break
		}
	}

	if !hasSuggestionKeyword {
		return ""
	}

	// Extract URL
	matches := urlRegex.FindStringSubmatch(message)
	if len(matches) > 0 {
		return matches[0]
	}

	return ""
}

// FormatMediaList formats a list of media suggestions as a markdown message
func FormatMediaList(suggestions []MediaSuggestion) string {
	if len(suggestions) == 0 {
		return "No media suggestions available. Feel free to suggest something by sharing a link and mentioning it as a suggestion!"
	}

	var sb strings.Builder
	sb.WriteString("📋 *Current Media Suggestions*\n\n")

	for i, suggestion := range suggestions {
		sb.WriteString(fmt.Sprintf("%d. [%s](%s) (by @%s, %d week(s))\n",
			i+1,
			truncateURL(suggestion.URL),
			suggestion.URL,
			suggestion.Suggester,
			suggestion.WeeksActive))
	}

	return sb.String()
}

// Helper function to make URLs more readable in the list
func truncateURL(url string) string {
	// Remove protocol
	url = strings.Replace(url, "https://", "", 1)
	url = strings.Replace(url, "http://", "", 1)

	// Remove www. if present
	url = strings.Replace(url, "www.", "", 1)

	// If URL is too long, truncate it
	if len(url) > 40 {
		return url[:37] + "..."
	}

	return url
}

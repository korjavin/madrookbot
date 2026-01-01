package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/qdrant/go-client/qdrant"
)

// ConversationService handles conversation caching and persistence
type ConversationService struct {
	db         *sql.DB
	cache      *ConversationCache
	maxCacheMB int
}

// NewConversationService creates a new ConversationService
func NewConversationService(database *sql.DB, maxCacheMB int) *ConversationService {
	if maxCacheMB <= 0 {
		maxCacheMB = 20 // Default 20MB
	}

	svc := &ConversationService{
		db: database,
		cache: &ConversationCache{
			nodes:         make(map[int]*ConversationNode),
			maxCacheBytes: maxCacheMB * 1024 * 1024,
			currentBytes:  0,
			mu:            sync.RWMutex{},
		},
		maxCacheMB: maxCacheMB,
	}

	// Initialize database table
	if err := svc.initDatabase(); err != nil {
		log.Printf("[ERROR] Failed to initialize conversation database: %v", err)
	}

	return svc
}

// initDatabase creates the conversation_messages table
func (s *ConversationService) initDatabase() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS conversation_messages (
		message_id INTEGER PRIMARY KEY,
		parent_id INTEGER NOT NULL,
		chat_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		text TEXT NOT NULL,
		role TEXT NOT NULL,
		system_prompt TEXT,
		timestamp TIMESTAMP NOT NULL
	)`)
	if err != nil {
		return err
	}

	// Create index on timestamp for faster cleanup
	_, err = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_conversation_timestamp
		ON conversation_messages(timestamp)`)

	return err
}

// AddMessage adds a message to the conversation tree
func (s *ConversationService) AddMessage(node *ConversationNode) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	// Calculate approximate size
	node.ApproxSize = len(node.Text) + len(node.SystemPrompt) + 100

	// Add to cache
	s.cache.nodes[node.MessageID] = node
	s.cache.currentBytes += node.ApproxSize

	// Check if we need to evict old messages
	s.cache.evictIfNeeded()
}

// GetMessage retrieves a message from the cache
func (s *ConversationService) GetMessage(messageID int) (*ConversationNode, bool) {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	node, exists := s.cache.nodes[messageID]
	return node, exists
}

// BuildConversationHistory walks up the tree to collect the last N exchanges
func (s *ConversationService) BuildConversationHistory(messageID int, maxExchanges int) []ConversationNode {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	var history []ConversationNode
	currentID := messageID

	for currentID != 0 && len(history) < maxExchanges*2 {
		node, exists := s.cache.nodes[currentID]
		if !exists {
			break
		}

		// Prepend to maintain chronological order
		history = append([]ConversationNode{*node}, history...)
		currentID = node.ParentID
	}

	if len(history) > maxExchanges*2 {
		history = history[len(history)-maxExchanges*2:]
	}

	return history
}

// GetSystemPrompt retrieves the system prompt from the conversation root
func (s *ConversationService) GetSystemPrompt(messageID int) string {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	currentID := messageID
	for currentID != 0 {
		node, exists := s.cache.nodes[currentID]
		if !exists {
			break
		}
		if node.SystemPrompt != "" {
			return node.SystemPrompt
		}
		currentID = node.ParentID
	}

	return ""
}

// Size returns the current cache size in bytes
func (s *ConversationService) Size() int {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	return s.cache.currentBytes
}

// Count returns the number of messages in cache
func (s *ConversationService) Count() int {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	return len(s.cache.nodes)
}

// SaveToDatabase saves current cache to database
func (s *ConversationService) SaveToDatabase() error {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing data
	_, err = tx.Exec("DELETE FROM conversation_messages")
	if err != nil {
		return err
	}

	// Insert all current nodes
	stmt, err := tx.Prepare(`INSERT INTO conversation_messages
		(message_id, parent_id, chat_id, user_id, text, role, system_prompt, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	count := 0
	for _, node := range s.cache.nodes {
		_, err = stmt.Exec(
			node.MessageID,
			node.ParentID,
			node.ChatID,
			node.UserID,
			node.Text,
			node.Role,
			node.SystemPrompt,
			node.Timestamp,
		)
		if err != nil {
			return err
		}
		count++
	}

	return tx.Commit()
}

// ActivityService handles user activity tracking
type ActivityService struct {
	db              *sql.DB
	retentionMonths int
}

// NewActivityService creates a new ActivityService
func NewActivityService(database *sql.DB, retentionMonths int) *ActivityService {
	if retentionMonths <= 0 {
		retentionMonths = 3 // Default 3 months
	}

	return &ActivityService{
		db:              database,
		retentionMonths: retentionMonths,
	}
}

// TrackActivity records user activity
func (s *ActivityService) TrackActivity(groupID int64, username string) error {
	now := time.Now()
	hourBucket := now.Truncate(time.Hour)

	query := `INSERT OR IGNORE INTO user_activity (group_id, username, hour_bucket, message_count)
	          VALUES (?, ?, ?, 0)`
	_, err := s.db.Exec(query, groupID, username, hourBucket)
	if err != nil {
		return err
	}

	query = `UPDATE user_activity SET message_count = message_count + 1
	         WHERE group_id = ? AND username = ? AND hour_bucket = ?`
	_, err = s.db.Exec(query, groupID, username, hourBucket)
	return err
}

// GetTopUsers returns top active users for a group
func (s *ActivityService) GetTopUsers(groupID int64, limit int) ([]TopUser, error) {
	query := `SELECT user_name, SUM(message_count) as total_messages
	          FROM user_activity
	          WHERE group_id = ?
	          GROUP BY user_name
	          ORDER BY total_messages DESC
	          LIMIT ?`

	rows, err := s.db.Query(query, groupID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []TopUser
	for rows.Next() {
		var user TopUser
		if err := rows.Scan(&user.UserName, &user.MessageCount); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// TopUser represents a user with their message count
type TopUser struct {
	UserName    string
	MessageCount int
}

// GetPeakActivityHour returns the hour with most activity
func (s *ActivityService) GetPeakActivityHour(groupID int64) (int, error) {
	query := `SELECT hour_bucket, SUM(message_count) as total
	          FROM user_activity
	          WHERE group_id = ?
	          GROUP BY hour_bucket
	          ORDER BY total DESC
	          LIMIT 1`

	rows, err := s.db.Query(query, groupID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return 0, nil
	}

	var hourBucket time.Time
	var total int
	if err := rows.Scan(&hourBucket, &total); err != nil {
		return 0, err
	}

	return hourBucket.Hour(), nil
}

// CleanupOldActivity removes activity records older than retention period
func (s *ActivityService) CleanupOldActivity() error {
	cutoff := time.Now().AddDate(0, -s.retentionMonths, 0)
	query := `DELETE FROM user_activity WHERE hour_bucket < ?`
	result, err := s.db.Exec(query, cutoff)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected > 0 {
		log.Printf("[INFO] Cleaned up %d old activity records", rowsAffected)
	}

	return nil
}

// ClassService handles class scheduling and management
type ClassService struct {
	db     *sql.DB
	berlin *time.Location
}

// NewClassService creates a new ClassService
func NewClassService(database *sql.DB) *ClassService {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	return &ClassService{
		db:     database,
		berlin: berlin,
	}
}

// CreateClass creates a new class
func (s *ClassService) CreateClass(description string) (*Class, error) {
	scheduledTime := getNextSunday().In(s.berlin)

	query := `INSERT INTO classes (description, scheduled_time, created_at) VALUES (?, ?, ?)`
	result, err := s.db.Exec(query, description, scheduledTime, time.Now())
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Class{
		ID:            int(id),
		Description:   description,
		ScheduledTime: scheduledTime,
	}, nil
}

// GetActiveClass returns the most recent unpinned class
func (s *ClassService) GetActiveClass() (*Class, error) {
	query := `SELECT id, description, scheduled_time, announcement_message_id, unpinned
	          FROM classes WHERE unpinned = 0 ORDER BY created_at DESC LIMIT 1`

	var class Class
	err := s.db.QueryRow(query).Scan(
		&class.ID,
		&class.Description,
		&class.ScheduledTime,
		&class.AnnouncementMessageID,
		&class.Unpinned,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &class, nil
}

// CancelClass marks a class as cancelled
func (s *ClassService) CancelClass(id int) error {
	query := `UPDATE classes SET cancelled = 1, unpinned = 1 WHERE id = ?`
	_, err := s.db.Exec(query, id)
	return err
}

// NewsConfig holds configuration for the news service
type NewsConfig struct {
	Rand          *rand.Rand
	AlertCooldown time.Duration
}

// NewsService handles news posting operations
type NewsService struct {
	gpt           *openai.Client
	qdrant        qdrant.PointsClient
	db            *sql.DB
	alertCooldown time.Duration
	rand          *rand.Rand
}

// NewNewsService creates a new NewsService
func NewNewsService(gptClient *openai.Client, qdrantClient qdrant.PointsClient, database *sql.DB, cfg NewsConfig) *NewsService {
	if cfg.AlertCooldown == 0 {
		cfg.AlertCooldown = 31 * 24 * time.Hour
	}
	if cfg.Rand == nil {
		cfg.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	return &NewsService{
		gpt:           gptClient,
		qdrant:        qdrantClient,
		db:            database,
		alertCooldown: cfg.AlertCooldown,
		rand:          cfg.Rand,
	}
}

// GetLastPostTime returns the timestamp of the last news post
func (s *NewsService) GetLastPostTime() (time.Time, error) {
	var lastPostTime time.Time
	query := `SELECT last_post_time FROM news_posts ORDER BY last_post_time DESC LIMIT 1`

	err := s.db.QueryRow(query).Scan(&lastPostTime)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last news post time: %w", err)
	}

	return lastPostTime, nil
}

// RecordPost saves a news post to the database
func (s *NewsService) RecordPost(topic, newsURL string, messageID int) error {
	query := `INSERT INTO news_posts (last_post_time, topic, news_url, message_id) VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, time.Now(), topic, newsURL, messageID)
	if err != nil {
		return fmt.Errorf("failed to record news post: %w", err)
	}
	return nil
}

// IsOwner checks if a user is the bot owner
func IsOwner(userID int64) bool {
	ownerIDStr := os.Getenv("OWNER_ID")
	if ownerIDStr == "" {
		return false
	}

	ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64)
	if err != nil {
		return false
	}

	return userID == ownerID
}

// MemoryService handles message memory storage in Qdrant
type MemoryService struct {
	qdrantClient   qdrant.PointsClient
	openaiClient   *openai.Client
	recentMessages map[string]recentMessage
}

// NewMemoryService creates a new MemoryService
func NewMemoryService(qdrantClient qdrant.PointsClient, openaiClient *openai.Client) *MemoryService {
	return &MemoryService{
		qdrantClient:   qdrantClient,
		openaiClient:   openaiClient,
		recentMessages: make(map[string]recentMessage),
	}
}

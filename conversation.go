package main

import (
	"log"
	"sync"
	"time"
)

// ConversationNode represents a single message in the conversation tree
type ConversationNode struct {
	MessageID     int       // Telegram message ID
	ParentID      int       // Parent message ID (0 for root messages)
	ChatID        int64     // Chat ID where message was sent
	UserID        int       // User who sent the message
	Text          string    // Message text
	Role          string    // "user" or "assistant"
	SystemPrompt  string    // System prompt used for this conversation branch
	Timestamp     time.Time // When the message was created
	ApproxSize    int       // Approximate size in bytes for cache management
}

// ConversationCache manages the conversation tree
type ConversationCache struct {
	nodes         map[int]*ConversationNode // Map of messageID -> node
	maxCacheBytes int                       // Maximum cache size in bytes
	currentBytes  int                       // Current cache size in bytes
	mu            sync.RWMutex              // Thread-safe access
}

var conversationCache *ConversationCache

func initConversationCache() {
	conversationCache = &ConversationCache{
		nodes:         make(map[int]*ConversationNode),
		maxCacheBytes: 20 * 1024 * 1024, // 20MB
		currentBytes:  0,
	}
	log.Println("[INFO] Conversation cache initialized with 20MB limit")

	// Initialize database table
	err := initConversationDatabase()
	if err != nil {
		log.Printf("[ERROR] Failed to initialize conversation database: %v", err)
	}

	// Clean up old messages on startup
	err = cleanupOldConversations()
	if err != nil {
		log.Printf("[ERROR] Failed to cleanup old conversations: %v", err)
	}

	// Load conversations from database
	err = loadConversationsFromDatabase()
	if err != nil {
		log.Printf("[ERROR] Failed to load conversations from database: %v", err)
	}

	// Start periodic save every 5 minutes
	go periodicSaveConversations()
}

// AddMessage adds a message to the conversation tree
func (cc *ConversationCache) AddMessage(node *ConversationNode) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Calculate approximate size
	node.ApproxSize = len(node.Text) + len(node.SystemPrompt) + 100 // +100 for metadata

	// Add to cache
	cc.nodes[node.MessageID] = node
	cc.currentBytes += node.ApproxSize

	// Check if we need to evict old messages
	cc.evictIfNeeded()

	log.Printf("[DEBUG] Added message %d to conversation cache (parent: %d, size: %d bytes, total: %d bytes)",
		node.MessageID, node.ParentID, node.ApproxSize, cc.currentBytes)
}

// GetMessage retrieves a message from the cache
func (cc *ConversationCache) GetMessage(messageID int) (*ConversationNode, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	node, exists := cc.nodes[messageID]
	return node, exists
}

// BuildConversationHistory walks up the tree to collect the last N exchanges
// Returns messages in chronological order (oldest first)
func (cc *ConversationCache) BuildConversationHistory(messageID int, maxExchanges int) []ConversationNode {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	var history []ConversationNode
	currentID := messageID

	// Walk up the tree collecting messages
	for currentID != 0 && len(history) < maxExchanges*2 {
		node, exists := cc.nodes[currentID]
		if !exists {
			break
		}

		// Prepend to maintain chronological order
		history = append([]ConversationNode{*node}, history...)
		currentID = node.ParentID
	}

	// Limit to last maxExchanges exchanges (user+assistant pairs)
	if len(history) > maxExchanges*2 {
		history = history[len(history)-maxExchanges*2:]
	}

	log.Printf("[DEBUG] Built conversation history: %d messages for messageID %d", len(history), messageID)
	return history
}

// evictIfNeeded removes oldest messages when cache exceeds limit
func (cc *ConversationCache) evictIfNeeded() {
	if cc.currentBytes <= cc.maxCacheBytes {
		return
	}

	// Find oldest messages to evict
	type nodeTime struct {
		id   int
		time time.Time
		size int
	}

	var nodes []nodeTime
	for id, node := range cc.nodes {
		nodes = append(nodes, nodeTime{id: id, time: node.Timestamp, size: node.ApproxSize})
	}

	// Sort by timestamp (oldest first)
	for i := 0; i < len(nodes)-1; i++ {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[i].time.After(nodes[j].time) {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			}
		}
	}

	// Evict oldest until we're under limit
	bytesToFree := cc.currentBytes - (cc.maxCacheBytes * 3 / 4) // Free to 75% capacity
	freed := 0
	evicted := 0

	for _, nt := range nodes {
		if freed >= bytesToFree {
			break
		}
		delete(cc.nodes, nt.id)
		freed += nt.size
		evicted++
	}

	cc.currentBytes -= freed
	log.Printf("[INFO] Evicted %d old messages, freed %d bytes (current: %d bytes)",
		evicted, freed, cc.currentBytes)
}

// GetSystemPrompt retrieves the system prompt from the conversation root
func (cc *ConversationCache) GetSystemPrompt(messageID int) string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	// Walk up to find the root or first message with system prompt
	currentID := messageID
	for currentID != 0 {
		node, exists := cc.nodes[currentID]
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

// Database persistence functions

const conversationRetentionDays = 7

// initConversationDatabase creates the conversation_messages table
func initConversationDatabase() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS conversation_messages (
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
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_conversation_timestamp
		ON conversation_messages(timestamp)`)

	return err
}

// cleanupOldConversations removes messages older than retention period
func cleanupOldConversations() error {
	cutoffTime := time.Now().Add(-conversationRetentionDays * 24 * time.Hour)
	result, err := db.Exec("DELETE FROM conversation_messages WHERE timestamp < ?", cutoffTime)
	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("[INFO] Cleaned up %d old conversation messages (older than %d days)",
			rowsAffected, conversationRetentionDays)
	}

	return nil
}

// saveConversationsToDatabase saves current cache to database
func saveConversationsToDatabase() error {
	if conversationCache == nil {
		return nil
	}

	conversationCache.mu.RLock()
	defer conversationCache.mu.RUnlock()

	// Begin transaction for better performance
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing data (we'll save the entire current state)
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
	for _, node := range conversationCache.nodes {
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

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Saved %d conversation messages to database", count)

	// Cleanup old messages after save
	return cleanupOldConversations()
}

// loadConversationsFromDatabase loads conversations from database into cache
func loadConversationsFromDatabase() error {
	if conversationCache == nil {
		return nil
	}

	rows, err := db.Query(`SELECT message_id, parent_id, chat_id, user_id, text, role,
		system_prompt, timestamp FROM conversation_messages ORDER BY timestamp ASC`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	count := 0
	totalBytes := 0
	uniqueChats := make(map[int64]bool)
	uniqueUsers := make(map[int]bool)
	oldestTime := time.Now()
	newestTime := time.Time{}

	for rows.Next() {
		var node ConversationNode
		err = rows.Scan(
			&node.MessageID,
			&node.ParentID,
			&node.ChatID,
			&node.UserID,
			&node.Text,
			&node.Role,
			&node.SystemPrompt,
			&node.Timestamp,
		)
		if err != nil {
			log.Printf("[ERROR] Failed to scan conversation row: %v", err)
			continue
		}

		// Calculate size
		node.ApproxSize = len(node.Text) + len(node.SystemPrompt) + 100

		// Track stats
		totalBytes += node.ApproxSize
		uniqueChats[node.ChatID] = true
		uniqueUsers[node.UserID] = true
		if node.Timestamp.Before(oldestTime) {
			oldestTime = node.Timestamp
		}
		if node.Timestamp.After(newestTime) {
			newestTime = node.Timestamp
		}

		// Add to cache (without locking since we're in init)
		conversationCache.mu.Lock()
		conversationCache.nodes[node.MessageID] = &node
		conversationCache.currentBytes += node.ApproxSize
		conversationCache.mu.Unlock()

		count++
	}

	if count > 0 {
		cacheUsagePercent := float64(totalBytes) / float64(conversationCache.maxCacheBytes) * 100
		log.Printf("[INFO] Loaded %d conversation messages from database", count)
		log.Printf("[INFO] - Cache usage: %d bytes (%.1f%% of 20MB limit)", totalBytes, cacheUsagePercent)
		log.Printf("[INFO] - Unique chats: %d, unique users: %d", len(uniqueChats), len(uniqueUsers))
		log.Printf("[INFO] - Time range: %s to %s",
			oldestTime.Format("2006-01-02 15:04"),
			newestTime.Format("2006-01-02 15:04"))
	}

	return rows.Err()
}

// periodicSaveConversations saves cache to database every 5 minutes
func periodicSaveConversations() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		err := saveConversationsToDatabase()
		if err != nil {
			log.Printf("[ERROR] Failed to save conversations to database: %v", err)
		}
	}
}

// SaveConversationsOnShutdown should be called before the bot exits
func SaveConversationsOnShutdown() {
	log.Println("[INFO] Saving conversations to database before shutdown...")
	err := saveConversationsToDatabase()
	if err != nil {
		log.Printf("[ERROR] Failed to save conversations on shutdown: %v", err)
	} else {
		log.Println("[INFO] Conversations saved successfully")
	}
}

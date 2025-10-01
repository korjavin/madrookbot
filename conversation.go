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

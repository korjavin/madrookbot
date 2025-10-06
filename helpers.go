package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// getEnvInt64 is a helper function to get int64 from environment variable
func getEnvInt64(key string) (int64, error) {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return 0, fmt.Errorf("%s environment variable not set", key)
	}

	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %v", key, err)
	}

	return value, nil
}
// shouldEnableMemoryTool checks if the memory search tool should be enabled
// for the given chat and user. Tool is enabled for:
// - Messages in MEMORY_GROUP_ID
// - Direct messages from OWNER_ID
func shouldEnableMemoryTool(chatID int64, userID int64) bool {
	// Check if in memory group
	memoryGroupIDStr := os.Getenv("MEMORY_GROUP_ID")
	if memoryGroupIDStr != "" {
		memoryGroupID, err := strconv.ParseInt(memoryGroupIDStr, 10, 64)
		if err == nil && chatID == memoryGroupID {
			return true
		}
	}

	// Check if DM from owner
	ownerIDStr := os.Getenv("OWNER_ID")
	if ownerIDStr != "" {
		ownerID, err := strconv.ParseInt(ownerIDStr, 10, 64)
		if err == nil && userID == ownerID {
			// Check if it's a DM (chat ID == user ID for DMs)
			if userID == chatID {
				log.Printf("[DEBUG] Memory tool enabled for owner DM")
				return true
			}
		}
	}

	return false
}

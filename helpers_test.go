package main

import (
	"os"
	"testing"
)

// TestGetEnvInt64 tests the getEnvInt64 function
func TestGetEnvInt64(t *testing.T) {
	// Save original environment
	originalValue := os.Getenv("TEST_INT64_ENV")
	defer os.Setenv("TEST_INT64_ENV", originalValue)

	tests := []struct {
		name        string
		setEnvValue string
		expectedVal int64
		expectedErr bool
	}{
		{
			name:        "valid integer",
			setEnvValue: "12345",
			expectedVal: 12345,
			expectedErr: false,
		},
		{
			name:        "zero value",
			setEnvValue: "0",
			expectedVal: 0,
			expectedErr: false,
		},
		{
			name:        "negative value",
			setEnvValue: "-100",
			expectedVal: -100,
			expectedErr: false,
		},
		{
			name:        "large value",
			setEnvValue: "9223372036854775807",
			expectedVal: 9223372036854775807,
			expectedErr: false,
		},
		{
			name:        "not set",
			setEnvValue: "",
			expectedVal: 0,
			expectedErr: true,
		},
		{
			name:        "invalid value",
			setEnvValue: "not-a-number",
			expectedVal: 0,
			expectedErr: true,
		},
		{
			name:        "float value",
			setEnvValue: "3.14",
			expectedVal: 0,
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnvValue == "" {
				os.Unsetenv("TEST_INT64_ENV")
			} else {
				os.Setenv("TEST_INT64_ENV", tt.setEnvValue)
			}

			val, err := getEnvInt64("TEST_INT64_ENV")

			if tt.expectedErr && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
			if !tt.expectedErr && val != tt.expectedVal {
				t.Errorf("expected %d, got %d", tt.expectedVal, val)
			}
		})
	}
}

// TestShouldEnableMemoryTool tests the shouldEnableMemoryTool function
func TestShouldEnableMemoryTool(t *testing.T) {
	// Save original environment
	originalMemoryGroupID := os.Getenv("MEMORY_GROUP_ID")
	originalOwnerID := os.Getenv("OWNER_ID")
	defer func() {
		os.Setenv("MEMORY_GROUP_ID", originalMemoryGroupID)
		os.Setenv("OWNER_ID", originalOwnerID)
	}()

	tests := []struct {
		name      string
		memoryGroupID string
		ownerID   string
		chatID    int64
		userID    int64
		expected  bool
	}{
		{
			name:           "in memory group",
			memoryGroupID:  "12345",
			ownerID:        "67890",
			chatID:         12345,
			userID:         11111,
			expected:       true,
		},
		{
			name:           "owner DM",
			memoryGroupID:  "12345",
			ownerID:        "67890",
			chatID:         67890, // Same as userID (DM)
			userID:         67890,
			expected:       true,
		},
		{
			name:           "not in memory group and not owner DM",
			memoryGroupID:  "12345",
			ownerID:        "67890",
			chatID:         99999,
			userID:         11111,
			expected:       false,
		},
		{
			name:           "in group but wrong user",
			memoryGroupID:  "12345",
			ownerID:        "67890",
			chatID:         12345,
			userID:         67890, // Owner but not in memory group
			expected:       true,  // Should still be true because in memory group
		},
		{
			name:           "memory group ID not set",
			memoryGroupID:  "",
			ownerID:        "67890",
			chatID:         12345,
			userID:         11111,
			expected:       false,
		},
		{
			name:           "owner ID not set",
			memoryGroupID:  "12345",
			ownerID:        "",
			chatID:         67890,
			userID:         67890,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.memoryGroupID == "" {
				os.Unsetenv("MEMORY_GROUP_ID")
			} else {
				os.Setenv("MEMORY_GROUP_ID", tt.memoryGroupID)
			}
			if tt.ownerID == "" {
				os.Unsetenv("OWNER_ID")
			} else {
				os.Setenv("OWNER_ID", tt.ownerID)
			}

			result := shouldEnableMemoryTool(tt.chatID, tt.userID)
			if result != tt.expected {
				t.Errorf("shouldEnableMemoryTool(%d, %d) = %v, want %v",
					tt.chatID, tt.userID, result, tt.expected)
			}
		})
	}
}

// TestShouldEnableMemoryToolEdgeCases tests edge cases
func TestShouldEnableMemoryToolEdgeCases(t *testing.T) {
	// Save original environment
	originalMemoryGroupID := os.Getenv("MEMORY_GROUP_ID")
	originalOwnerID := os.Getenv("OWNER_ID")
	defer func() {
		os.Setenv("MEMORY_GROUP_ID", originalMemoryGroupID)
		os.Setenv("OWNER_ID", originalOwnerID)
	}()

	// Test with invalid memory group ID
	os.Setenv("MEMORY_GROUP_ID", "invalid")
	os.Setenv("OWNER_ID", "12345")
	result := shouldEnableMemoryTool(12345, 67890)
	if result {
		t.Error("expected false with invalid memory group ID")
	}

	// Reset to valid
	os.Setenv("MEMORY_GROUP_ID", "12345")
	os.Setenv("OWNER_ID", "67890")

	// Test with matching memory group ID (positive number)
	result = shouldEnableMemoryTool(12345, 67890)
	if !result {
		t.Error("expected true for matching memory group ID")
	}
}

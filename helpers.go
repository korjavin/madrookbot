package main

import (
	"fmt"
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
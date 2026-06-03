package utils

import "os"

// GetEnv retrieves the value of the environment variable named by key.
// If the variable is empty, it returns the defaultValue.
func GetEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

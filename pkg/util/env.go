package util

import (
	"fmt"
	"os"
)

// GetClickUpPAT retrieves the CLICKUP_PAT environment variable.
// Returns an error if the variable is not set or is empty.
func GetClickUpPAT() (string, error) {
	pat := os.Getenv("CLICKUP_PAT")
	if pat == "" {
		return "", fmt.Errorf("CLICKUP_PAT environment variable not set. Get your token from: https://app.clickup.com/settings/apps")
	}
	return pat, nil
}

package filter

import (
	"strings"

	"clickup-tui/pkg/clickup"
)

// ShouldIncludeTask determines if a task should be included based on its status and optionally its assignee.
// If showAll is false, only includes tasks in "in progress" or "in review" status.
// If showAll is true, includes all tasks except "completed" or "closed".
// If mineOnly is true, only includes tasks where the given userID is an assignee.
func ShouldIncludeTask(task clickup.Task, userID string, showAll bool, mineOnly bool) bool {
	if mineOnly {
		isAssigned := false
		for _, assignee := range task.Assignees {
			if assignee.ID.String() == userID {
				isAssigned = true
				break
			}
		}
		if !isAssigned {
			return false
		}
	}

	status := strings.ToLower(task.Status.Status)

	if showAll {
		return status != "completed" && status != "closed"
	}
	return status == "in progress" || status == "in review"
}

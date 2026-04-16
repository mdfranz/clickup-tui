package filter

import (
	"strings"

	"clickup-tui/pkg/clickup"
)

// ShouldIncludeTask determines if a task should be included based on its status.
// If showAll is false, only includes tasks in "in progress" or "in review" status.
// If showAll is true, includes all tasks except "completed" or "closed".
func ShouldIncludeTask(task clickup.Task, showAll bool) bool {
	status := strings.ToLower(task.Status.Status)

	if showAll {
		return status != "completed" && status != "closed"
	}
	return status == "in progress" || status == "in review"
}

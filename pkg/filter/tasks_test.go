package filter

import (
	"testing"

	"clickup-tui/pkg/clickup"
)

func TestShouldIncludeTask_Active(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		showAll  bool
		expected bool
	}{
		// Active statuses
		{"in progress - active", "in progress", false, true},
		{"in review - active", "in review", false, true},
		{"In Progress - case insensitive", "In Progress", false, true},
		{"IN REVIEW - case insensitive", "IN REVIEW", false, true},

		// Non-active statuses (excluded when showAll=false)
		{"backlog - not shown", "backlog", false, false},
		{"scoping - not shown", "scoping", false, false},
		{"todo - not shown", "todo", false, false},
		{"done - not shown", "done", false, false},

		// Show all cases
		{"in progress - show all", "in progress", true, true},
		{"in review - show all", "in review", true, true},
		{"backlog - show all", "backlog", true, true},
		{"scoping - show all", "scoping", true, true},
		{"todo - show all", "todo", true, true},

		// Excluded even with show all
		{"completed - excluded", "completed", true, false},
		{"closed - excluded", "closed", true, false},
		{"Completed - case insensitive", "Completed", true, false},
		{"CLOSED - case insensitive", "CLOSED", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := clickup.Task{
				ID:   "test-id",
				Name: "Test Task",
			}
			task.Status.Status = tt.status

			result := ShouldIncludeTask(task, tt.showAll)
			if result != tt.expected {
				t.Errorf("ShouldIncludeTask(%q, %v) = %v, want %v",
					tt.status, tt.showAll, result, tt.expected)
			}
		})
	}
}

func TestShouldIncludeTask_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		showAll  bool
		expected bool
	}{
		{"empty status", "", false, false},
		{"empty status with show all", "", true, true},
		{"whitespace status", "   ", false, false},
		{"unknown status", "unknown", false, false},
		{"unknown status with show all", "unknown", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := clickup.Task{
				ID:   "test-id",
				Name: "Test Task",
			}
			task.Status.Status = tt.status

			result := ShouldIncludeTask(task, tt.showAll)
			if result != tt.expected {
				t.Errorf("ShouldIncludeTask(%q, %v) = %v, want %v",
					tt.status, tt.showAll, result, tt.expected)
			}
		})
	}
}

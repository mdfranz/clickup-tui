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
		{"blocked - active", "blocked", false, true},
		{"In Progress - case insensitive", "In Progress", false, true},
		{"IN REVIEW - case insensitive", "IN REVIEW", false, true},
		{"Scoping - case insensitive", "Scoping", false, true},

		// Non-active statuses (excluded when showAll=false)
		{"backlog - not shown", "backlog", false, false},
		{"scoping - active", "scoping", false, true},
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

			result := ShouldIncludeTask(task, "123", tt.showAll, false)
			if result != tt.expected {
				t.Errorf("ShouldIncludeTask(%q, %v, false) = %v, want %v",
					tt.status, tt.showAll, result, tt.expected)
			}
		})
	}
}

func TestShouldIncludeTask_MineOnly(t *testing.T) {
	myID := "123"
	otherID := "456"

	tests := []struct {
		name      string
		assignees []string
		mineOnly  bool
		expected  bool
	}{
		{"assigned to me - mineOnly=true", []string{myID}, true, true},
		{"assigned to others - mineOnly=true", []string{otherID}, true, false},
		{"assigned to me and others - mineOnly=true", []string{otherID, myID}, true, true},
		{"not assigned - mineOnly=true", []string{}, true, false},
		{"assigned to me - mineOnly=false", []string{myID}, false, true},
		{"assigned to others - mineOnly=false", []string{otherID}, false, true},
		{"not assigned - mineOnly=false", []string{}, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var assignees []clickup.User
			for _, id := range tt.assignees {
				var userID clickup.UserID
				if id == "123" {
					userID = 123
				} else {
					userID = 456
				}
				assignees = append(assignees, clickup.User{ID: userID})
			}

			task := clickup.Task{
				ID:        "test-id",
				Name:      "Test Task",
				Assignees: assignees,
			}
			task.Status.Status = "in progress"

			result := ShouldIncludeTask(task, myID, false, tt.mineOnly)
			if result != tt.expected {
				t.Errorf("ShouldIncludeTask(mineOnly=%v, assignees=%v) = %v, want %v",
					tt.mineOnly, tt.assignees, result, tt.expected)
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

			result := ShouldIncludeTask(task, "123", tt.showAll, false)
			if result != tt.expected {
				t.Errorf("ShouldIncludeTask(%q, %v, false) = %v, want %v",
					tt.status, tt.showAll, result, tt.expected)
			}
		})
	}
}

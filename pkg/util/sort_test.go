package util

import (
	"testing"

	"clickup-tui/pkg/clickup"
)

func TestSortTasksByDateDesc(t *testing.T) {
	tasks := []clickup.Task{
		{ID: "1", DateUpdated: "1000"},
		{ID: "2", DateUpdated: "3000"},
		{ID: "3", DateUpdated: "2000"},
	}

	SortTasksByDateDesc(tasks)

	if tasks[0].ID != "2" || tasks[1].ID != "3" || tasks[2].ID != "1" {
		t.Errorf("SortTasksByDateDesc failed. Got order: %s, %s, %s", tasks[0].ID, tasks[1].ID, tasks[2].ID)
	}
}

func TestSortCommentsByDateDesc(t *testing.T) {
	comments := []clickup.Comment{
		{ID: "1", Date: "1000"},
		{ID: "2", Date: "3000"},
		{ID: "3", Date: "2000"},
	}

	SortCommentsByDateDesc(comments)

	if comments[0].ID != "2" || comments[1].ID != "3" || comments[2].ID != "1" {
		t.Errorf("SortCommentsByDateDesc failed. Got order: %s, %s, %s", comments[0].ID, comments[1].ID, comments[2].ID)
	}
}

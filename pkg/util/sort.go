package util

import (
	"sort"
	"strconv"

	"clickup-tui/pkg/clickup"
)

// SortTasksByDateDesc sorts a slice of tasks in place, by DateUpdated descending (newest first)
func SortTasksByDateDesc(tasks []clickup.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		timeI, _ := strconv.ParseInt(tasks[i].DateUpdated, 10, 64)
		timeJ, _ := strconv.ParseInt(tasks[j].DateUpdated, 10, 64)
		return timeI > timeJ // Descending
	})
}

// SortCommentsByDateDesc sorts a slice of comments in place, by Date descending (newest first)
func SortCommentsByDateDesc(comments []clickup.Comment) {
	sort.Slice(comments, func(i, j int) bool {
		timeI, _ := strconv.ParseInt(comments[i].Date, 10, 64)
		timeJ, _ := strconv.ParseInt(comments[j].Date, 10, 64)
		return timeI > timeJ // Descending
	})
}

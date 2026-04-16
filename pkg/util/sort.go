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

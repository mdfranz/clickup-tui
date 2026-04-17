package main

import (
	"fmt"
	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/util"
)

func main() {
	pat, _ := util.GetClickUpPAT()
	client := clickup.NewClient(pat)
	lists, _ := client.GetLists("901314371471")
	for _, l := range lists {
		tasks, _ := client.GetTasks(l.ID)
		for _, t := range tasks {
			fmt.Printf("ID: %s, Name: %s\n", t.ID, t.Name)
		}
	}
}

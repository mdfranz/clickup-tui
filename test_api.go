package main

import (
	"fmt"
	"clickup-tui/pkg/util"
	"clickup-tui/pkg/clickup"
)

func main() {
	pat, _ := util.GetClickUpPAT()
	client := clickup.NewClient(pat)
	teams, _ := client.GetTeams()
	if len(teams) > 0 {
		fmt.Println("Team ID:", teams[0].ID)
	}
}

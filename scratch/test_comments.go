package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"clickup-tui/pkg/util"
)

func main() {
	pat, err := util.GetClickUpPAT()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	req, _ := http.NewRequest("GET", "https://api.clickup.com/api/v2/task/86agxvzt9/activity?team_id=90131842483", nil) // I need a valid task ID, let me find one in the user's output
	req.Header.Add("Authorization", pat)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}

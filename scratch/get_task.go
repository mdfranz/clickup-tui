package main

import (
	"fmt"
	"io"
	"net/http"
	"clickup-tui/pkg/util"
)

func main() {
	pat, _ := util.GetClickUpPAT()
	req, _ := http.NewRequest("GET", "https://api.clickup.com/api/v2/task/86agxvzt9", nil)
	req.Header.Add("Authorization", pat)
	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}

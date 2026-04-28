package clickup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"clickup-tui/pkg/logger"
)

const APIURL = "https://api.clickup.com/api/v2/"

type Member struct {
	User User `json:"user"`
}

type Team struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []Member `json:"members"`
}

type TeamsResponse struct {
	Teams []Team `json:"teams"`
}

type Space struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SpacesResponse struct {
	Spaces []Space `json:"spaces"`
}

type Folder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FoldersResponse struct {
	Folders []Folder `json:"folders"`
}

type Status struct {
	Status string `json:"status"`
	Color  string `json:"color"`
	Type   string `json:"type"`
}

type List struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Statuses []Status `json:"statuses"`
}

type ListsResponse struct {
	Lists []List `json:"lists"`
}

type User struct {
	ID       UserID `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// UserID handles both numeric and string ID values from the API
type UserID int64

func (u UserID) String() string {
	return fmt.Sprintf("%d", u)
}

type Task struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status struct {
		Status string `json:"status"`
	} `json:"status"`
	ParentID    string `json:"parent"`
	Assignees   []User `json:"assignees"`
	Creator     User   `json:"creator"`
	DateCreated string `json:"date_created"` // Unix timestamp in milliseconds as string
	DateUpdated string `json:"date_updated"` // Unix timestamp in milliseconds as string
	DateDone    string `json:"date_done"`    // Unix timestamp in milliseconds as string
	DateClosed  string `json:"date_closed"`  // Unix timestamp in milliseconds as string
	TextContent string `json:"text_content"` // Task description
}

func (t *Task) UnmarshalJSON(data []byte) error {
	type Alias Task
	aux := &struct {
		Parent interface{} `json:"parent"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Parent != nil {
		switch v := aux.Parent.(type) {
		case string:
			t.ParentID = v
		case map[string]interface{}:
			if id, ok := v["id"].(string); ok {
				t.ParentID = id
			}
		}
	}

	return nil
}

type TasksResponse struct {
	Tasks []Task `json:"tasks"`
}

type Comment struct {
	ID          string `json:"id"`
	CommentText string `json:"comment_text"`
	User        User   `json:"user"`
	Date        string `json:"date"` // Unix timestamp in milliseconds as string
}

type CommentsResponse struct {
	Comments []Comment `json:"comments"`
}

type Client struct {
	HTTPClient *http.Client
	PAT        string
}

func NewClient(pat string) *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		PAT:        pat,
	}
}

// doRequest is a helper method that handles the standard HTTP request/response pattern.
// It creates a request, adds authorization, executes it, checks the status,
// reads the body, and unmarshals the JSON response into the target.
func (c *Client) doRequest(method, url string, target interface{}) error {
	return c.doRequestWithBody(method, url, nil, target)
}

func (c *Client) doRequestWithBody(method, url string, body io.Reader, target interface{}) error {
	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = io.ReadAll(body)
		if err != nil {
			return err
		}
	}

	slog.Info("API Request", "method", method, "url", url, "body", logger.TruncateBody(string(reqBody)))

	var bodyReader io.Reader
	if len(reqBody) > 0 {
		bodyReader = bytes.NewReader(reqBody)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		slog.Error("Failed to create request", "error", err)
		return err
	}

	req.Header.Add("Authorization", c.PAT)
	if bodyReader != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.HTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		slog.Error("API Request Failed", "error", err, "duration", duration)
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response body", "error", err)
		return err
	}

	slog.Info("API Response",
		"status", resp.StatusCode,
		"duration", duration,
		"body", logger.TruncateBody(string(respBody)),
	)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(respBody, target); err != nil {
		slog.Error("Failed to unmarshal response", "error", err, "body", logger.TruncateBody(string(respBody)))
		return err
	}

	return nil
}

type Activity struct {
	ID     string `json:"id"`
	User   User   `json:"user"`
	Type   string `json:"type"`
	Date   string `json:"date"` // Unix timestamp in milliseconds as string
	TaskID string `json:"task_id"`
	Source string `json:"source"`
	Detail string `json:"detail,omitempty"`
}

type ActivityResponse struct {
	History []Activity `json:"history"`
}

func (c *Client) GetRecentTasks(listID string, dateUpdatedGt int64) ([]Task, error) {
	url := fmt.Sprintf("%slist/%s/task?archived=false&include_closed=true&subtasks=true&date_updated_gt=%d", APIURL, listID, dateUpdatedGt)
	var tasksResp TasksResponse
	if err := c.doRequest("GET", url, &tasksResp); err != nil {
		return nil, err
	}
	return tasksResp.Tasks, nil
}

func (c *Client) GetWorkspaceUsers(workspaceID string) ([]User, error) {
	teams, err := c.GetTeams()
	if err != nil {
		return nil, err
	}
	for _, t := range teams {
		if t.ID == workspaceID {
			var users []User
			for _, m := range t.Members {
				users = append(users, m.User)
			}
			return users, nil
		}
	}
	return nil, fmt.Errorf("workspace %s not found", workspaceID)
}

func (c *Client) GetTeams() ([]Team, error) {
	var teamsResp TeamsResponse
	if err := c.doRequest("GET", APIURL+"team", &teamsResp); err != nil {
		return nil, err
	}
	return teamsResp.Teams, nil
}

func (c *Client) GetUser() (User, error) {
	var userResp struct {
		User User `json:"user"`
	}
	if err := c.doRequest("GET", APIURL+"user", &userResp); err != nil {
		return User{}, err
	}
	return userResp.User, nil
}

func (c *Client) GetSpaces(teamID string) ([]Space, error) {
	url := fmt.Sprintf("%steam/%s/space?archived=false", APIURL, teamID)
	var spacesResp SpacesResponse
	if err := c.doRequest("GET", url, &spacesResp); err != nil {
		return nil, err
	}
	return spacesResp.Spaces, nil
}

func (c *Client) GetFolders(spaceID string) ([]Folder, error) {
	url := fmt.Sprintf("%sspace/%s/folder?archived=false", APIURL, spaceID)
	var foldersResp FoldersResponse
	if err := c.doRequest("GET", url, &foldersResp); err != nil {
		return nil, err
	}
	return foldersResp.Folders, nil
}

func (c *Client) GetLists(folderID string) ([]List, error) {
	url := fmt.Sprintf("%sfolder/%s/list?archived=false", APIURL, folderID)
	var listsResp ListsResponse
	if err := c.doRequest("GET", url, &listsResp); err != nil {
		return nil, err
	}
	return listsResp.Lists, nil
}

func (c *Client) GetTasks(listID string, includeClosed bool) ([]Task, error) {
	url := fmt.Sprintf("%slist/%s/task?archived=false&include_closed=%t&subtasks=true", APIURL, listID, includeClosed)
	var tasksResp TasksResponse
	if err := c.doRequest("GET", url, &tasksResp); err != nil {
		return nil, err
	}
	return tasksResp.Tasks, nil
}

func (c *Client) GetTask(taskID string) (Task, error) {
	url := fmt.Sprintf("%stask/%s", APIURL, taskID)
	var task Task
	if err := c.doRequest("GET", url, &task); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (c *Client) GetTaskComments(taskID string) ([]Comment, error) {
	url := fmt.Sprintf("%stask/%s/comment", APIURL, taskID)
	var commentsResp CommentsResponse
	if err := c.doRequest("GET", url, &commentsResp); err != nil {
		return nil, err
	}
	return commentsResp.Comments, nil
}

func (c *Client) UpdateTaskStatus(taskID string, status string) error {
	url := fmt.Sprintf("%stask/%s", APIURL, taskID)
	payload, err := json.Marshal(map[string]interface{}{
		"status": status,
	})
	if err != nil {
		return err
	}
	var result map[string]interface{}
	return c.doRequestWithBody("PUT", url, bytes.NewReader(payload), &result)
}

func (c *Client) GetList(listID string) (List, error) {
	url := fmt.Sprintf("%slist/%s", APIURL, listID)
	var list List
	if err := c.doRequest("GET", url, &list); err != nil {
		return List{}, err
	}
	return list, nil
}

func (c *Client) CreateTaskComment(taskID string, commentText string) error {
	url := fmt.Sprintf("%stask/%s/comment", APIURL, taskID)
	payload, err := json.Marshal(map[string]string{"comment_text": commentText})
	if err != nil {
		return err
	}
	var result map[string]interface{}
	return c.doRequestWithBody("POST", url, bytes.NewReader(payload), &result)
}

func (c *Client) CreateTask(listID string, name string, description string, status string, assignees []int64) (Task, error) {
	url := fmt.Sprintf("%slist/%s/task", APIURL, listID)
	payload := map[string]interface{}{
		"name": name,
	}
	if description != "" {
		payload["description"] = description
	}
	if status != "" {
		payload["status"] = status
	}
	if len(assignees) > 0 {
		payload["assignees"] = assignees
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Task{}, err
	}
	var task Task
	if err := c.doRequestWithBody("POST", url, bytes.NewReader(body), &task); err != nil {
		return Task{}, err
	}
	return task, nil
}

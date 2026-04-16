package clickup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const APIURL = "https://api.clickup.com/api/v2/"

type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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

type List struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	Assignees   []User `json:"assignees"`
	DateUpdated string `json:"date_updated"` // Unix timestamp in milliseconds as string
	TextContent string `json:"text_content"` // Task description
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
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return err
	}

	req.Header.Add("Authorization", c.PAT)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, target); err != nil {
		return err
	}

	return nil
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

func (c *Client) GetTasks(listID string) ([]Task, error) {
	url := fmt.Sprintf("%slist/%s/task?archived=false&include_closed=false", APIURL, listID)
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

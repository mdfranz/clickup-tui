package clickup

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Task struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status struct {
		Status string `json:"status"`
	} `json:"status"`
	Assignees   []User `json:"assignees"`
	DateUpdated string `json:"date_updated"` // Unix timestamp in milliseconds as string
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
		HTTPClient: &http.Client{},
		PAT:        pat,
	}
}

func (c *Client) GetTeams() ([]Team, error) {
	req, err := http.NewRequest("GET", APIURL+"team", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", c.PAT)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get teams: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var teamsResp TeamsResponse
	if err := json.Unmarshal(body, &teamsResp); err != nil {
		return nil, err
	}

	return teamsResp.Teams, nil
}

func (c *Client) GetUser() (User, error) {
	req, err := http.NewRequest("GET", APIURL+"user", nil)
	if err != nil {
		return User{}, err
	}

	req.Header.Add("Authorization", c.PAT)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return User{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("failed to get current user: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return User{}, err
	}

	var userResp struct {
		User User `json:"user"`
	}
	if err := json.Unmarshal(body, &userResp); err != nil {
		return User{}, err
	}

	return userResp.User, nil
}

func (c *Client) GetSpaces(teamID string) ([]Space, error) {
	url := fmt.Sprintf("%steam/%s/space?archived=false", APIURL, teamID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", c.PAT)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get spaces for team %s: status %d", teamID, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var spacesResp SpacesResponse
	if err := json.Unmarshal(body, &spacesResp); err != nil {
		return nil, err
	}

	return spacesResp.Spaces, nil
}

func (c *Client) GetFolders(spaceID string) ([]Folder, error) {
	url := fmt.Sprintf("%sspace/%s/folder?archived=false", APIURL, spaceID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", c.PAT)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get folders for space %s: status %d", spaceID, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var foldersResp FoldersResponse
	if err := json.Unmarshal(body, &foldersResp); err != nil {
		return nil, err
	}

	return foldersResp.Folders, nil
}

func (c *Client) GetLists(folderID string) ([]List, error) {
	url := fmt.Sprintf("%sfolder/%s/list?archived=false", APIURL, folderID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", c.PAT)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get lists for folder %s: status %d", folderID, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var listsResp ListsResponse
	if err := json.Unmarshal(body, &listsResp); err != nil {
		return nil, err
	}

	return listsResp.Lists, nil
}

func (c *Client) GetTasks(listID string) ([]Task, error) {
	url := fmt.Sprintf("%slist/%s/task?archived=false&include_closed=false", APIURL, listID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", c.PAT)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get tasks for list %s: status %d", listID, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tasksResp TasksResponse
	if err := json.Unmarshal(body, &tasksResp); err != nil {
		return nil, err
	}

	return tasksResp.Tasks, nil
}

func (c *Client) GetTaskComments(taskID string) ([]Comment, error) {
	url := fmt.Sprintf("%stask/%s/comment", APIURL, taskID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", c.PAT)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get comments for task %s: status %d", taskID, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var commentsResp CommentsResponse
	if err := json.Unmarshal(body, &commentsResp); err != nil {
		return nil, err
	}

	return commentsResp.Comments, nil
}

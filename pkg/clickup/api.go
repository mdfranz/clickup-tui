package clickup

// API defines all operations on the ClickUp API.
// Both Client and cache.CachedClient implement this interface.
type API interface {
	GetTeams() ([]Team, error)
	GetUser() (User, error)
	GetSpaces(teamID string) ([]Space, error)
	GetFolders(spaceID string) ([]Folder, error)
	GetLists(folderID string) ([]List, error)
	GetList(listID string) (List, error)
	GetTasks(listID string) ([]Task, error)
	GetRecentTasks(listID string, dateUpdatedGt int64) ([]Task, error)
	GetTask(taskID string) (Task, error)
	GetTaskComments(taskID string) ([]Comment, error)
	GetWorkspaceUsers(workspaceID string) ([]User, error)
	UpdateTaskStatus(taskID string, status string) error
	CreateTaskComment(taskID string, commentText string) error
	CreateTask(listID string, name string, description string, status string, assignees []int64) (Task, error)
}

// Compile-time check that *Client satisfies API.
var _ API = (*Client)(nil)

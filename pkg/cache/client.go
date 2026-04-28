package cache

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"clickup-tui/pkg/clickup"
)

// CachedClient wraps a clickup.Client and caches API responses to disk.
// It implements clickup.API.
type CachedClient struct {
	inner     clickup.API
	store     *Store
	cachePath string
	noCache   bool
	dirty     bool
}

// Compile-time check that *CachedClient satisfies clickup.API.
var _ clickup.API = (*CachedClient)(nil)

// NewCachedClient creates a new CachedClient wrapping the given client.
// If noCache is true, all reads bypass the cache (but writes still invalidate).
func NewCachedClient(inner clickup.API, noCache bool) (*CachedClient, error) {
	cachePath := CachePath()
	store := loadOrCreate(cachePath)
	return &CachedClient{
		inner:     inner,
		store:     store,
		cachePath: cachePath,
		noCache:   noCache,
	}, nil
}

// Flush writes the cache to disk if it has been modified.
func (c *CachedClient) Flush() error {
	if !c.dirty {
		return nil
	}
	slog.Info("Flushing cache to disk", "path", c.cachePath)
	return flush(c.store, c.cachePath)
}

// --- Org data (TTL-based) ---

func (c *CachedClient) GetUser() (clickup.User, error) {
	if !c.noCache && c.store.User != nil && time.Since(c.store.User.FetchedAt) < TTLUser {
		slog.Debug("Cache hit", "method", "GetUser")
		return c.store.User.Data, nil
	}
	user, err := c.inner.GetUser()
	if err != nil {
		if c.store.User != nil {
			slog.Warn("API error, returning stale cache", "method", "GetUser", "error", err)
			return c.store.User.Data, nil
		}
		return clickup.User{}, err
	}
	c.store.User = &CachedValue[clickup.User]{Data: user, FetchedAt: time.Now()}
	c.dirty = true
	return user, nil
}

func (c *CachedClient) GetTeams() ([]clickup.Team, error) {
	if !c.noCache && c.store.Teams != nil && time.Since(c.store.Teams.FetchedAt) < TTLTeams {
		slog.Debug("Cache hit", "method", "GetTeams")
		return c.store.Teams.Data, nil
	}
	teams, err := c.inner.GetTeams()
	if err != nil {
		if c.store.Teams != nil {
			slog.Warn("API error, returning stale cache", "method", "GetTeams", "error", err)
			return c.store.Teams.Data, nil
		}
		return nil, err
	}
	c.store.Teams = &CachedValue[[]clickup.Team]{Data: teams, FetchedAt: time.Now()}
	c.dirty = true
	return teams, nil
}

func (c *CachedClient) GetSpaces(teamID string) ([]clickup.Space, error) {
	if !c.noCache {
		if cached, ok := c.store.Spaces[teamID]; ok && time.Since(cached.FetchedAt) < TTLSpaces {
			slog.Debug("Cache hit", "method", "GetSpaces", "teamID", teamID)
			return cached.Data, nil
		}
	}
	spaces, err := c.inner.GetSpaces(teamID)
	if err != nil {
		if cached, ok := c.store.Spaces[teamID]; ok {
			slog.Warn("API error, returning stale cache", "method", "GetSpaces", "error", err)
			return cached.Data, nil
		}
		return nil, err
	}
	c.store.Spaces[teamID] = &CachedValue[[]clickup.Space]{Data: spaces, FetchedAt: time.Now()}
	c.dirty = true
	return spaces, nil
}

func (c *CachedClient) GetFolders(spaceID string) ([]clickup.Folder, error) {
	if !c.noCache {
		if cached, ok := c.store.Folders[spaceID]; ok && time.Since(cached.FetchedAt) < TTLFolders {
			slog.Debug("Cache hit", "method", "GetFolders", "spaceID", spaceID)
			return cached.Data, nil
		}
	}
	folders, err := c.inner.GetFolders(spaceID)
	if err != nil {
		if cached, ok := c.store.Folders[spaceID]; ok {
			slog.Warn("API error, returning stale cache", "method", "GetFolders", "error", err)
			return cached.Data, nil
		}
		return nil, err
	}
	c.store.Folders[spaceID] = &CachedValue[[]clickup.Folder]{Data: folders, FetchedAt: time.Now()}
	c.dirty = true
	return folders, nil
}

func (c *CachedClient) GetLists(folderID string) ([]clickup.List, error) {
	if !c.noCache {
		if cached, ok := c.store.Lists[folderID]; ok && time.Since(cached.FetchedAt) < TTLLists {
			slog.Debug("Cache hit", "method", "GetLists", "folderID", folderID)
			return cached.Data, nil
		}
	}
	lists, err := c.inner.GetLists(folderID)
	if err != nil {
		if cached, ok := c.store.Lists[folderID]; ok {
			slog.Warn("API error, returning stale cache", "method", "GetLists", "error", err)
			return cached.Data, nil
		}
		return nil, err
	}
	c.store.Lists[folderID] = &CachedValue[[]clickup.List]{Data: lists, FetchedAt: time.Now()}
	c.dirty = true
	return lists, nil
}

func (c *CachedClient) GetList(listID string) (clickup.List, error) {
	if !c.noCache {
		if cached, ok := c.store.ListDetail[listID]; ok && time.Since(cached.FetchedAt) < TTLListDetail {
			slog.Debug("Cache hit", "method", "GetList", "listID", listID)
			return cached.Data, nil
		}
	}
	list, err := c.inner.GetList(listID)
	if err != nil {
		if cached, ok := c.store.ListDetail[listID]; ok {
			slog.Warn("API error, returning stale cache", "method", "GetList", "error", err)
			return cached.Data, nil
		}
		return clickup.List{}, err
	}
	c.store.ListDetail[listID] = &CachedValue[clickup.List]{Data: list, FetchedAt: time.Now()}
	c.dirty = true
	return list, nil
}

func (c *CachedClient) GetWorkspaceUsers(workspaceID string) ([]clickup.User, error) {
	if !c.noCache {
		if cached, ok := c.store.WsUsers[workspaceID]; ok && time.Since(cached.FetchedAt) < TTLWsUsers {
			slog.Debug("Cache hit", "method", "GetWorkspaceUsers", "workspaceID", workspaceID)
			return cached.Data, nil
		}
	}
	users, err := c.inner.GetWorkspaceUsers(workspaceID)
	if err != nil {
		if cached, ok := c.store.WsUsers[workspaceID]; ok {
			slog.Warn("API error, returning stale cache", "method", "GetWorkspaceUsers", "error", err)
			return cached.Data, nil
		}
		return nil, err
	}
	c.store.WsUsers[workspaceID] = &CachedValue[[]clickup.User]{Data: users, FetchedAt: time.Now()}
	c.dirty = true
	return users, nil
}

// --- Task data (incremental updates) ---

func (c *CachedClient) GetTasks(listID string, includeClosed bool) ([]clickup.Task, error) {
	cached := c.store.Tasks[listID]

	// If we need closed tasks but cache doesn't have them, we must do a full fetch
	// (unless it's an incremental update, but GetRecentTasks with include_closed=true
	// will only return RECENTLY updated closed tasks, not ALL closed tasks).
	// For simplicity, if includeClosed=true and cache.IncludesClosed=false, we force a full fetch.
	if !c.noCache && cached != nil && includeClosed && !cached.IncludesClosed {
		slog.Debug("Cache upgrade (fetching closed tasks)", "method", "GetTasks", "listID", listID)
		tasks, err := c.inner.GetTasks(listID, true)
		if err == nil {
			hwm := int64(0)
			for _, t := range tasks {
				if du := parseUnixMs(t.DateUpdated); du > hwm {
					hwm = du
				}
			}
			c.store.Tasks[listID] = &TaskCache{
				Tasks:          tasks,
				FetchedAt:      time.Now(),
				MaxDateUpdated: hwm,
				IncludesClosed: true,
			}
			c.dirty = true
			return tasks, nil
		}
		// On error, fall back to what we have
		return filterActiveTasks(cached.Tasks), nil
	}

	if !c.noCache && cached != nil && time.Since(cached.FetchedAt) < TTLTasksFull {
		// Incremental update: fetch only tasks updated since high-water mark
		recent, err := c.inner.GetRecentTasks(listID, cached.MaxDateUpdated)
		if err != nil {
			slog.Warn("Incremental update failed, returning stale cache", "method", "GetTasks", "error", err)
			if !includeClosed {
				return filterActiveTasks(cached.Tasks), nil
			}
			return cached.Tasks, nil
		}
		if len(recent) > 0 {
			merged := mergeTasks(cached.Tasks, recent)
			// We store everything in the cache now, filtering happens at the UI level
			hwm := cached.MaxDateUpdated
			for _, t := range recent {
				if du := parseUnixMs(t.DateUpdated); du > hwm {
					hwm = du
				}
			}
			c.store.Tasks[listID] = &TaskCache{
				Tasks:          merged,
				FetchedAt:      time.Now(),
				MaxDateUpdated: hwm,
				IncludesClosed: cached.IncludesClosed, // stays the same, or could we have gained closed tasks?
			}
			// Actually, GetRecentTasks always has include_closed=true, so we might have gained closed tasks.
			// But it doesn't guarantee we have ALL closed tasks if we started with only active ones.

			c.dirty = true
			slog.Debug("Cache incremental update", "method", "GetTasks", "listID", listID, "updated", len(recent))
			
			if !includeClosed {
				return filterActiveTasks(merged), nil
			}
			return merged, nil
		}
		// No updates — refresh the TTL timer
		cached.FetchedAt = time.Now()
		c.dirty = true
		slog.Debug("Cache hit (no new updates)", "method", "GetTasks", "listID", listID)
		
		if !includeClosed {
			return filterActiveTasks(cached.Tasks), nil
		}
		return cached.Tasks, nil
	}

	// Full fetch
	tasks, err := c.inner.GetTasks(listID, includeClosed)
	if err != nil {
		if cached != nil {
			slog.Warn("API error, returning stale cache", "method", "GetTasks", "error", err)
			if !includeClosed {
				return filterActiveTasks(cached.Tasks), nil
			}
			return cached.Tasks, nil
		}
		return nil, err
	}
	hwm := int64(0)
	for _, t := range tasks {
		if du := parseUnixMs(t.DateUpdated); du > hwm {
			hwm = du
		}
	}
	c.store.Tasks[listID] = &TaskCache{
		Tasks:          tasks,
		FetchedAt:      time.Now(),
		MaxDateUpdated: hwm,
		IncludesClosed: includeClosed,
	}
	c.dirty = true
	slog.Debug("Cache miss (full fetch)", "method", "GetTasks", "listID", listID, "count", len(tasks))
	
	if !includeClosed {
		return filterActiveTasks(tasks), nil
	}
	return tasks, nil
}

func (c *CachedClient) GetRecentTasks(listID string, dateUpdatedGt int64) ([]clickup.Task, error) {
	// Pass through — this is already a targeted query
	return c.inner.GetRecentTasks(listID, dateUpdatedGt)
}

func (c *CachedClient) GetTask(taskID string) (clickup.Task, error) {
	if !c.noCache {
		if cached, ok := c.store.TaskDetail[taskID]; ok && time.Since(cached.FetchedAt) < TTLTaskDetail {
			slog.Debug("Cache hit", "method", "GetTask", "taskID", taskID)
			return cached.Data, nil
		}
	}
	task, err := c.inner.GetTask(taskID)
	if err != nil {
		if cached, ok := c.store.TaskDetail[taskID]; ok {
			slog.Warn("API error, returning stale cache", "method", "GetTask", "error", err)
			return cached.Data, nil
		}
		return clickup.Task{}, err
	}
	c.store.TaskDetail[taskID] = &CachedValue[clickup.Task]{Data: task, FetchedAt: time.Now()}
	c.dirty = true
	return task, nil
}

// --- Comments (TTL-based) ---

func (c *CachedClient) GetTaskComments(taskID string) ([]clickup.Comment, error) {
	if !c.noCache {
		if cached, ok := c.store.Comments[taskID]; ok && time.Since(cached.FetchedAt) < TTLComments {
			slog.Debug("Cache hit", "method", "GetTaskComments", "taskID", taskID)
			return cached.Data, nil
		}
	}
	comments, err := c.inner.GetTaskComments(taskID)
	if err != nil {
		if cached, ok := c.store.Comments[taskID]; ok {
			slog.Warn("API error, returning stale cache", "method", "GetTaskComments", "error", err)
			return cached.Data, nil
		}
		return nil, err
	}
	c.store.Comments[taskID] = &CachedValue[[]clickup.Comment]{Data: comments, FetchedAt: time.Now()}
	c.dirty = true
	return comments, nil
}

// --- Write operations (pass through + invalidate) ---

func (c *CachedClient) UpdateTaskStatus(taskID string, status string) error {
	err := c.inner.UpdateTaskStatus(taskID, status)
	if err == nil {
		c.invalidateTask(taskID)
	}
	return err
}

func (c *CachedClient) CreateTaskComment(taskID string, commentText string) error {
	err := c.inner.CreateTaskComment(taskID, commentText)
	if err == nil {
		delete(c.store.Comments, taskID)
		c.dirty = true
	}
	return err
}

func (c *CachedClient) CreateTask(listID string, name string, description string, status string, assignees []int64) (clickup.Task, error) {
	task, err := c.inner.CreateTask(listID, name, description, status, assignees)
	if err == nil {
		// Invalidate task cache for this list so next fetch picks up the new task
		delete(c.store.Tasks, listID)
		c.dirty = true
	}
	return task, err
}

// --- Helpers ---

// invalidateTask removes a task from all list caches and the detail cache.
func (c *CachedClient) invalidateTask(taskID string) {
	delete(c.store.TaskDetail, taskID)
	for listID, tc := range c.store.Tasks {
		for i, t := range tc.Tasks {
			if t.ID == taskID {
				c.store.Tasks[listID].Tasks = append(tc.Tasks[:i], tc.Tasks[i+1:]...)
				c.dirty = true
				return
			}
		}
	}
	c.dirty = true
}

// mergeTasks merges updated tasks into the existing list, replacing by ID.
func mergeTasks(existing, updates []clickup.Task) []clickup.Task {
	index := make(map[string]int, len(existing))
	for i, t := range existing {
		index[t.ID] = i
	}
	for _, t := range updates {
		if i, ok := index[t.ID]; ok {
			existing[i] = t
		} else {
			existing = append(existing, t)
		}
	}
	return existing
}

// filterActiveTasks removes closed/completed tasks to match GetTasks (include_closed=false) behavior.
func filterActiveTasks(tasks []clickup.Task) []clickup.Task {
	result := tasks[:0]
	for _, t := range tasks {
		status := strings.ToLower(t.Status.Status)
		if status != "closed" {
			result = append(result, t)
		}
	}
	return result
}

func parseUnixMs(s string) int64 {
	if s == "" {
		return 0
	}
	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return ms
}

// Info returns human-readable cache statistics.
func (c *CachedClient) Info() string {
	s := c.store
	taskCount := 0
	for _, tc := range s.Tasks {
		taskCount += len(tc.Tasks)
	}
	commentCount := 0
	for range s.Comments {
		commentCount++
	}
	return fmt.Sprintf("Cache version: %d\nLast updated: %s\nTeams cached: %v\nSpaces cached: %d\nFolders cached: %d\nLists cached: %d\nTask lists cached: %d (total tasks: %d)\nTask details cached: %d\nComment sets cached: %d",
		s.Version,
		s.UpdatedAt.Format(time.RFC3339),
		s.Teams != nil,
		len(s.Spaces),
		len(s.Folders),
		len(s.Lists),
		len(s.Tasks), taskCount,
		len(s.TaskDetail),
		commentCount,
	)
}

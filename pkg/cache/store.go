package cache

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"clickup-tui/pkg/clickup"
)

const cacheVersion = 1

// Store is the top-level cache structure, serialized to a single JSON file.
type Store struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`

	// Organizational data (TTL-based)
	User    *CachedValue[clickup.User]   `json:"user,omitempty"`
	Teams   *CachedValue[[]clickup.Team] `json:"teams,omitempty"`
	Spaces  map[string]*CachedValue[[]clickup.Space]  `json:"spaces,omitempty"`
	Folders map[string]*CachedValue[[]clickup.Folder] `json:"folders,omitempty"`
	Lists   map[string]*CachedValue[[]clickup.List]   `json:"lists,omitempty"`
	ListDetail map[string]*CachedValue[clickup.List]  `json:"list_detail,omitempty"`
	WsUsers map[string]*CachedValue[[]clickup.User]   `json:"ws_users,omitempty"`

	// Task data (timestamp-based incremental updates)
	Tasks map[string]*TaskCache `json:"tasks,omitempty"`

	// Individual task detail data (TTL-based, keyed by taskID)
	TaskDetail map[string]*CachedValue[clickup.Task] `json:"task_detail,omitempty"`

	// Comment data (TTL-based)
	Comments map[string]*CachedValue[[]clickup.Comment] `json:"comments,omitempty"`
}

// CachedValue wraps any cached value with its fetch timestamp.
type CachedValue[T any] struct {
	Data      T         `json:"data"`
	FetchedAt time.Time `json:"fetched_at"`
}

// TaskCache stores tasks for a list plus the high-water mark for incremental updates.
type TaskCache struct {
	Tasks          []clickup.Task `json:"tasks"`
	FetchedAt      time.Time      `json:"fetched_at"`
	MaxDateUpdated int64          `json:"max_date_updated"`
}

func newStore() *Store {
	return &Store{
		Version:    cacheVersion,
		Spaces:     make(map[string]*CachedValue[[]clickup.Space]),
		Folders:    make(map[string]*CachedValue[[]clickup.Folder]),
		Lists:      make(map[string]*CachedValue[[]clickup.List]),
		ListDetail: make(map[string]*CachedValue[clickup.List]),
		WsUsers:    make(map[string]*CachedValue[[]clickup.User]),
		Tasks:      make(map[string]*TaskCache),
		TaskDetail: make(map[string]*CachedValue[clickup.Task]),
		Comments:   make(map[string]*CachedValue[[]clickup.Comment]),
	}
}

// CachePath returns the path to the cache file.
func CachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "clickup-tui", "cache.json")
}

func loadOrCreate(path string) *Store {
	data, err := os.ReadFile(path)
	if err != nil {
		return newStore()
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		slog.Warn("Corrupt cache file, starting fresh", "error", err)
		return newStore()
	}
	if s.Version != cacheVersion {
		slog.Warn("Unsupported cache version, starting fresh", "version", s.Version)
		return newStore()
	}
	// Initialize nil maps
	if s.Spaces == nil {
		s.Spaces = make(map[string]*CachedValue[[]clickup.Space])
	}
	if s.Folders == nil {
		s.Folders = make(map[string]*CachedValue[[]clickup.Folder])
	}
	if s.Lists == nil {
		s.Lists = make(map[string]*CachedValue[[]clickup.List])
	}
	if s.ListDetail == nil {
		s.ListDetail = make(map[string]*CachedValue[clickup.List])
	}
	if s.WsUsers == nil {
		s.WsUsers = make(map[string]*CachedValue[[]clickup.User])
	}
	if s.Tasks == nil {
		s.Tasks = make(map[string]*TaskCache)
	}
	if s.TaskDetail == nil {
		s.TaskDetail = make(map[string]*CachedValue[clickup.Task])
	}
	if s.Comments == nil {
		s.Comments = make(map[string]*CachedValue[[]clickup.Comment])
	}
	return &s
}

func flush(store *Store, path string) error {
	store.UpdatedAt = time.Now()
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

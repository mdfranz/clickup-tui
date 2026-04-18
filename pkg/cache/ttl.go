package cache

import "time"

const (
	TTLUser       = 24 * time.Hour
	TTLTeams      = 4 * time.Hour
	TTLSpaces     = 4 * time.Hour
	TTLFolders    = 1 * time.Hour
	TTLLists      = 1 * time.Hour
	TTLListDetail = 1 * time.Hour
	TTLWsUsers    = 4 * time.Hour
	TTLTaskDetail = 10 * time.Minute
	TTLComments   = 5 * time.Minute
	// Tasks use incremental updates via GetRecentTasks within this window.
	// After this duration, a full refresh is performed.
	TTLTasksFull = 30 * time.Minute
)

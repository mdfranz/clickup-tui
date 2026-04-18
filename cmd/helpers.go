package cmd

import (
	"log/slog"

	"clickup-tui/pkg/cache"
	"clickup-tui/pkg/clickup"
)

// newCachedClient wraps a ClickUp client with caching.
// Returns the client (as clickup.API) and a cleanup function that flushes the cache.
func newCachedClient(pat string) (clickup.API, func()) {
	inner := clickup.NewClient(pat)
	cached, err := cache.NewCachedClient(inner, noCache)
	if err != nil {
		slog.Warn("Failed to initialize cache, proceeding without", "error", err)
		return inner, func() {}
	}
	return cached, func() {
		if err := cached.Flush(); err != nil {
			slog.Warn("Failed to flush cache", "error", err)
		}
	}
}

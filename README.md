# clickup-tui

A terminal user interface (TUI) for ClickUp, built with Go, Cobra, and Bubble Tea.

## Features

- **Menu**: Interactive menu to quickly launch any command
- **Setup**: Interactive wizard to configure your ClickUp workspace, space, and folders
- **Tasks**: Display tasks from your configured folders with filtering, nested subtasks, and comments
- **Browse (Dual-Pane)**: Interactively browse tasks with a fast, split-screen layout that shows details and comments as you scroll
- **New**: Create tasks in your saved folders with interactive assignee selection (including unassigned)
- **Track**: View user activity for the last 10 days
- **Standup**: Interactive, multi-step workflow to select tasks, view recent comments, update statuses, and post updates
- **Team Status**: View and aggregate team activity across a configurable window of days with factual AI summaries
- **AI Summarization**: Generate high-level folder summaries and team status reports using Gemini
- **Offline Caching**: Fast API response caching with flags to refresh/bypass cache, plus statistics and clean options
- **Show**: Display current configuration
- **Flexible Filtering**: Show active tasks or all open tasks, and filter by assignee (defaults to your tasks)
- **Comment Display**: View recent comments for tasks
- **XDG Compliance**: Respects XDG Base Directory specification for config storage and cache


## Installation

### Build from Source

Requirements: Go 1.26+

```bash
git clone https://github.com/yourusername/clickup-tui.git
cd clickup-tui
make build
```

The binary will be created as `clickup-tui` in the current directory.

### Using go install

```bash
go install github.com/yourusername/clickup-tui@latest
```

## Configuration

### Environment Variables

Configure ClickUp access, AI integrations, and logging via environment variables:

```bash
# ClickUp Personal Access Token (Required)
# Get yours from: https://app.clickup.com/settings/apps
export CLICKUP_PAT="your_personal_access_token"

# Gemini AI API Key (Optional, required for AI summaries)
export GEMINI_API_KEY="your_gemini_api_key"
# (Can also use GOOGLE_API_KEY="your_key")
```

### Initialize Configuration

Run the setup wizard to select your workspace, space, and folders:

```bash
clickup-tui setup
```

This will save your configuration to:
- `$XDG_CONFIG_HOME/clickup-tui/config.toml` (if set)
- `~/.config/clickup-tui/config.toml` (default)
- `~/.local/clickup-tui.toml` (legacy, for backwards compatibility)

### Offline Caching

To reduce API rate limits and accelerate performance, `clickup-tui` automatically caches API responses.

You can bypass or clear the cache using global flags:

```bash
# Bypass the cache and pull fresh data directly from ClickUp
clickup-tui tasks --refresh
# Or short flag:
clickup-tui tasks -r

# Force-clear the local cache before running the command
clickup-tui tasks --clear-cache
```

### Logging Configuration

Control logging verbosity with environment variables:

```bash
# Default (minimal logging, response bodies redacted)
clickup-tui tasks

# Log full API response bodies for debugging
LOG_RESPONSE_BODIES=1 clickup-tui tasks

# Log sensitive data (emails, user IDs, etc.)
LOG_SENSITIVE_DATA=1 clickup-tui tasks

# Combined verbose logging
LOG_RESPONSE_BODIES=1 LOG_SENSITIVE_DATA=1 clickup-tui tasks

# Store logs locally instead of cache directory
LOG_LOCAL=1 clickup-tui tasks
```

Logs are written to:
- `app.log` (if `LOG_LOCAL=1`)
- `$XDG_CACHE_HOME/clickup-tui/app.log` (default)
- `~/.cache/clickup-tui/app.log` (if XDG_CACHE_HOME not set)

## Usage

### Interactive Menu

Launch an interactive menu to select and launch any command:

```bash
clickup-tui menu
```

### Display Active Tasks

Show your tasks currently in progress, scoping, blocked, or in review (defaults to your assigned tasks). If nested subtasks are present, they are rendered in a tree format under their parent task:

```bash
clickup-tui tasks
```

### Display All Open Tasks

Show all your tasks except completed/closed (includes Backlog):

```bash
clickup-tui tasks --all
```

### Display All Users' Tasks

Show tasks for all assignees, not just yours:

```bash
clickup-tui tasks --mine=false
```

### Display Tasks with Comments

Show active tasks with the last 3 comments for each:

```bash
clickup-tui tasks --detailed
```

### Browse Tasks Interactively

Start an interactive split-pane browser for your tasks:

```bash
clickup-tui browse
```

To browse tasks for all assignees:

```bash
clickup-tui browse --mine=false
```

- **Left Pane**: Interactive scrollable list of tasks (sorted by last updated).
- **Right Pane**: Displays detailed task metadata and comments, updating automatically as you scroll.
- **Controls**:
  - `Arrow Up/Down` or `k/j`: Scroll task list
  - `c`: Add a task comment (opens a full-screen editor modal)
  - `s`: Change task status (opens an interactive status picker modal)
  - `q` / `Esc`: Exit back to terminal/menu
  - `Ctrl+C`: Exit program

### Conduct a Daily Standup

Start a multi-step guided standup workflow:

```bash
clickup-tui standup
```

1. **Task Selection**: Filter and multi-select tasks you worked on (supports `Space` to select, `a` to toggle all, `Enter` to confirm).
2. **Review & Update**: For each chosen task, view recent comments, add a new comment, and change its status (`Tab` to select status, `Ctrl+S` to submit, `Esc` to skip).
3. **Submit**: Updates are sent to ClickUp in the background.
4. **Summary**: Displays a neat summary of all comments added and statuses changed.

Flags:
- `--all` or `-a`: Include backlog/all open tasks
- `--mine=false`: Show other team members' tasks too

### Folder Summaries (AI)

Generate high-level, rich AI summaries of active tasks across all your configured folders (requires `GEMINI_API_KEY` or `GOOGLE_API_KEY`):

```bash
clickup-tui summarize
```

Flags:
- `--all` or `-a`: Include all open tasks (including backlog)
- `--team`: Include folder tasks for the entire team
- `--mine`: Limit folder tasks to those assigned to you (default: true)

### Team Status (AI & Raw Activity)

View and aggregate contributions, comments, and completions across your configured workspace and folders over a window of days (defaults to 7 days, requires `GEMINI_API_KEY` or `GOOGLE_API_KEY`):

```bash
clickup-tui team-status
```

Flags:
- `--days` or `-d`: Custom activity window in days (default: 7)
- `--summarize` or `-s`: Toggle generating an AI summary of team activity (default: true)
- `--raw` or `-c`: Display the raw, chronologically sorted activity log underneath or instead of the AI summary (default: false)

### Create a New Task

Create a task with interactive wizard guides for selecting folders, lists, and statuses, entering names/descriptions, and choosing an assignee:

```bash
clickup-tui new
```

- **Assignee Selection**: Automatically prompts whether the task is for you or someone else. If "No", displays a list of workspace users, including an option to leave the task **Unassigned**.

### Track User Activity

View the last 10 days of activity for a specific user:

```bash
clickup-tui track
```

You can also provide a user ID directly:

```bash
clickup-tui track 1234567
```

### Manage Local Cache

Inspect or clear your local offline response cache:

```bash
# View cache file location, size, and statistics (hits, misses, entries)
clickup-tui cache info

# Clear the local response cache manually
clickup-tui cache clear
```

### Clean Configuration and Cache

Interactively remove configuration and cache files:

```bash
clickup-tui clean
```

### View Current Configuration

Display your current setup:

```bash
clickup-tui show
```

## Development

### Running Tests

```bash
make test
```

View test coverage:

```bash
go test ./... -cover
```

### Code Quality

Format code:

```bash
make fmt
```

Run linter:

```bash
make lint
```

### Project Structure

```
.
├── cmd/              # CLI subcommands
│   ├── root.go       # Root command, persistent flags, caching helpers
│   ├── menu.go       # Interactive command menu launcher
│   ├── setup.go      # Interactive configuration wizard
│   ├── tasks.go      # Active and open task lister (with subtasks)
│   ├── browse.go     # Dual-pane task detail and comments browser
│   ├── new.go        # Task creation with assignee/unassigned flow
│   ├── standup.go    # Guided Daily Standup task update flow
│   ├── summarize.go  # Folder-level AI task summaries
│   ├── team_status.go# Team activity aggregator and AI summary
│   ├── track.go      # View user activity (10-day history)
│   ├── cache.go      # Local API cache manager (info/clear)
│   ├── clean.go      # Interactive config/cache remover
│   └── config.go     # Configuration viewer (show)
├── pkg/
│   ├── ai/           # Gemini AI content/summary generation clients
│   ├── cache/        # Local SQLite or file response cache layer
│   ├── clickup/      # ClickUp API model definitions and client
│   ├── config/       # TOML configuration loading and paths
│   ├── filter/       # Task status and assignee filters
│   ├── format/       # Custom date/time and relative time helpers
│   ├── ui/           # Shared Lipgloss styles, console spinners, prompts
│   └── util/         # Shell environment helpers
├── ARCHITECTURE.md   # Architectural design, diagrams, and data flows
├── PKG.md            # Classifications of 3rd-party dependencies
├── main.go           # CLI application entry point
├── go.mod            # Go module dependencies
├── Makefile          # Formatting, testing, and compilation targets
└── README.md         # This documentation
```

For more in-depth system information, see:
- [ARCHITECTURE.md](ARCHITECTURE.md) — System architecture, visual data flows, and design decisions.
- [PKG.md](PKG.md) — 3rd-party dependency classifications and usages.



## Troubleshooting

### "CLICKUP_PAT environment variable not set"

Solution: Set your ClickUp Personal Access Token:

```bash
export CLICKUP_PAT="your_token"
```

### "No configuration found"

Solution: Run the setup wizard:

```bash
clickup-tui setup
```

### "Error getting lists: API error: status 401"

Solution: Verify your CLICKUP_PAT is correct and hasn't expired.

### "No tasks found"

This is normal if:
- No tasks in your configured folders
- All tasks are already completed
- Your search filters exclude all tasks

Use `--all` flag to see all open tasks, or `--mine=false` to see tasks assigned to other people.
## License

MIT License - see LICENSE file for details

# clickup-tui

A terminal user interface (TUI) for ClickUp, built with Go, Cobra, and Bubble Tea.

## Features

- **Menu**: Interactive menu to quickly launch any command
- **Setup**: Interactive wizard to configure your ClickUp workspace, space, and folders
- **Tasks**: Display tasks from your configured folders with filtering and comments
- **Browse**: Interactively browse tasks with detailed view
- **New**: Create tasks in your saved folders
- **Show**: Display current configuration
- **Flexible Filtering**: Show active tasks or all open tasks, and filter by assignee (defaults to your tasks)
- **Comment Display**: View recent comments for tasks
- **XDG Compliance**: Respects XDG Base Directory specification for config storage

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

### Setup ClickUp Personal Access Token

1. Get your Personal Access Token from [ClickUp Settings](https://app.clickup.com/settings/apps)
2. Set the environment variable:

```bash
export CLICKUP_PAT="your_personal_access_token"
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

## Usage

### Interactive Menu

Launch an interactive menu to select a command:

```bash
clickup-tui menu
```

### Display Active Tasks

Show your tasks currently in progress or in review (defaults to your assigned tasks):

```bash
clickup-tui tasks
```

### Display All Open Tasks

Show all your tasks except completed/closed:

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

Start an interactive browser for your tasks:

```bash
clickup-tui browse
```

To browse tasks for all assignees:

```bash
clickup-tui browse --mine=false
```

Navigate with:
- Arrow keys: Select task
- Enter: View details
- Esc/q: Go back
- Ctrl+C: Exit

### Create a New Task

Create a task in one of your saved folders and lists:

```bash
clickup-tui new
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
├── cmd/              # CLI commands
│   ├── root.go       # Root command and version
│   ├── menu.go       # Interactive launcher
│   ├── setup.go      # Interactive setup
│   ├── tasks.go      # Task display
│   ├── browse.go     # Interactive browse
│   ├── new.go        # Create tasks
│   ├── standup.go    # Standup workflow
│   └── config.go     # Config display

│   └── config.go     # Config display
├── pkg/
│   ├── clickup/      # ClickUp API client
│   ├── config/       # Config management
│   ├── filter/       # Task filtering utilities
│   ├── format/       # Date formatting utilities
│   ├── ui/           # Styling and UI constants
│   └── util/         # General utilities
├── main.go           # Entry point
├── go.mod            # Module definition
├── Makefile          # Build targets
└── README.md         # This file
```

## Architecture

### CLI Structure

- **Cobra**: Command-line framework with subcommands
- **Bubble Tea**: Terminal UI framework for interactive features
- **Lipgloss**: Styling library for rich terminal output

### API Client

- Centralized HTTP client with `doRequest` helper to eliminate boilerplate
- Support for 30-second timeouts to prevent hanging
- Clean error messages with context

### Utilities

- **filter**: Task filtering logic (status-based)
- **format**: Date/time formatting (handles Unix millisecond timestamps)
- **ui**: Centralized styling with named colors and status color mapping
- **util**: Environment variable loading with helpful error messages

## Error Handling

- Clear error messages with helpful hints
- Missing CLICKUP_PAT provides link to get token
- Missing config suggests running setup
- Network errors include HTTP status codes
- All API errors are properly propagated

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

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - TUI components
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/pelletier/go-toml/v2` - TOML config

## License

MIT License - see LICENSE file for details

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Submit a pull request

## Roadmap

- [ ] Task search/filtering UI
- [ ] Quick task status update
- [ ] Subtask display
- [ ] Custom fields support
- [ ] Cache/offline mode
- [ ] Configuration profiles
- [ ] Custom status color themes

## Support

For issues or questions, please open an issue on GitHub.

# Dual-Pane Browse Feature

## Objective
Modify the `browse` command's TUI layout to be dual-pane. The left pane will display the list of active tasks, and the right pane will automatically display the details and comments for the currently selected task as the user scrolls, eliminating the need to explicitly select a task to view its comments.

Comment and status actions should remain full-screen modal flows. Pressing `c` or `s` from the dual-pane browse view should temporarily replace the main browse layout with the existing comment editor or status picker, then return to the dual-pane browse view when the action completes or is canceled.

## Key Files & Context
- `cmd/browse.go`: Contains the state machine (`browseModel`), layout handling, and rendering logic for the TUI.
- BubbleTea & Lipgloss: Used for TUI components (`list`, `viewport`, `textarea`) and layout (`lipgloss.JoinHorizontal`).

## Implementation Steps

### 1. Update State & Initial Model
- Remove `stateDetail` from the `browseState` enum since the list and detail views will be merged into `stateList`.
- Maintain the tracking of `m.selectedTask` and `m.comments`, but update them dynamically as the cursor moves in the list.
- Keep `stateComment` and `stateStatus` as separate full-screen modal states. Any existing transitions that currently return to `stateDetail` should instead return to the dual-pane browse state (`stateList`).

### 2. Window Resizing & Layout Setup
- In the `Update` function's `tea.WindowSizeMsg` case, split the available width evenly between the `list` (left) and `viewport` (right).
- Make sure `m.textarea` and `m.statusList` bounds are updated to look correct within the new layout.

### 3. Dynamic Comment Fetching (Scrolling)
- In the `Update` function for `stateList`, capture the previous selected task ID before calling `m.list.Update(msg)`.
- After `m.list.Update(msg)` processes the input, check the new selected task ID.
- If the selected task ID has changed (i.e. the user scrolled up or down), dispatch a `tea.Cmd` to fetch the new task's comments asynchronously and update `m.selectedTask`.
- When the initial `browseTasksMsg` (initial tasks load) is received, auto-select the first task and dispatch a command to fetch its comments.

### 4. Update Keybindings
- Move the keybindings from the old `stateDetail` (e.g. 'c' for comment, 's' for status) into `stateList`.
- Remove the 'Enter' key mapping for entering `stateDetail` and the 'Esc' mapping for exiting `stateDetail`, as it's no longer needed.
- Preserve the current full-screen behavior for `stateComment` and `stateStatus`: `c` and `s` should launch those modes from `stateList`, and cancel/submit paths should return to the dual-pane browse view.

### 5. Render View (Dual-Pane)
- Update `View()` to construct the main interface using `lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), m.viewport.View())`.
- Update `m.viewport.SetContent(m.renderDetail())` whenever the selected task or comments change.
- Ensure `m.renderDetail()` adapts to the new `m.viewport` width and does not assume full-screen width.
- Keep the existing full-screen rendering for comment entry and status selection, so `stateComment` and `stateStatus` continue to replace the whole screen instead of rendering inside a pane.

## Verification & Testing
- Run `go run main.go browse` (or build) and ensure the split layout appears correctly.
- Scroll up and down the list; verify that the right pane updates smoothly and asynchronously without blocking the UI.
- Press 'c' to leave a comment on a selected task and confirm it saves and updates the comment list.
- Press 's' to change the status of a task and verify it updates correctly in both panes.
- Confirm that comment entry and status selection still open as full-screen modal views and return to the dual-pane browse screen after cancel or submit.
- Verify that terminal window resizing gracefully reflows the text in the right pane and resizes the list on the left.

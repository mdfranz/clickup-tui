# Implementation Plan: Support Multiple Spaces in One Workspace

## Objective
Update the configuration structure to support multiple spaces within a single configured workspace, with each space owning its own list of folders via a nested `SpaceConfig` structure.

## Scope & Impact
This change is limited to one configured workspace at a time. It does not add simultaneous multi-workspace support or profiles.

This is a breaking config change. Existing users must rerun `clickup-tui setup` after upgrading.

This change will structurally alter the TOML configuration file and require updates to the `setup` wizard, configuration display, and any command that iterates over configured folders.

---

## Current State

**Step 1 is already done.** `pkg/config/config.go` already defines:

```go
type SpaceConfig struct {
    ID      string         `toml:"id"`
    Name    string         `toml:"name"`
    Folders []FolderConfig `toml:"folders"`
}

type Config struct {
    WorkspaceID   string        `toml:"workspace_id"`
    WorkspaceName string        `toml:"workspace_name"`
    Spaces        []SpaceConfig `toml:"spaces"`
}
```

**The build is currently broken.** Steps 2–4 were not applied when the struct was changed. All remaining steps must be completed together to restore compilation. The following files reference removed fields (`SpaceID`, `SpaceName`, `Folders` on `Config`):

- `cmd/setup.go:58–64` — builds `Config{SpaceID, SpaceName, Folders}`
- `cmd/tasks.go:73,91,93` — `cfg.Folders`, `cfg.SpaceName`
- `cmd/config.go:29–33` — `cfg.SpaceName`, `cfg.SpaceID`, `cfg.Folders`
- `cmd/new.go:35,152–154` — `cfg.Folders`
- `cmd/standup.go:154` — `m.cfg.Folders`
- `cmd/browse.go:171` — `m.cfg.Folders`
- `cmd/track.go:187` — `m.cfg.Folders`
- `cmd/summarize.go:67,78,80` — `cfg.Folders`, `cfg.SpaceName`

---

## Implementation Steps

### 1. ~~Update Configuration Structs~~ ✅ Done

`pkg/config/config.go` already has the correct struct shape. Tests in `pkg/config/config_test.go` should be updated to assert save/load round-trips for one workspace containing multiple spaces and folders.

### 2. Add a Shared Flattening Helper (`pkg/config/config.go`)

Add `ConfiguredFolder` and a `FlattenFolders` helper to `pkg/config` — this package is the right location because the helper operates on `Config` directly, is importable by all commands, and can be unit-tested alongside the serialization tests.

```go
type ConfiguredFolder struct {
    SpaceID    string
    SpaceName  string
    FolderID   string
    FolderName string
}

func FlattenFolders(cfg Config) []ConfiguredFolder {
    var out []ConfiguredFolder
    for _, space := range cfg.Spaces {
        for _, folder := range space.Folders {
            out = append(out, ConfiguredFolder{
                SpaceID:    space.ID,
                SpaceName:  space.Name,
                FolderID:   folder.ID,
                FolderName: folder.Name,
            })
        }
    }
    return out
}
```

Also add a `HasFolders(cfg Config) bool` convenience used by empty-state checks across commands.

### 3. Add a Legacy Config Migration Guard (`pkg/config/config.go`)

The old TOML format had `space_id`, `space_name`, `folders` at the top level. Loading the old format into the new `Config` struct produces an empty `Spaces` slice with no error — the user sees "No folders configured" with no explanation.

Add a migration check in `LoadConfig` (or as a helper called after load) that detects the old format and returns a clear error:

```go
// After unmarshal: if workspace is set but no spaces, the user has the old format
if cfg.WorkspaceID != "" && len(cfg.Spaces) == 0 {
    return cfg, fmt.Errorf("config format has changed — please run 'clickup-tui setup' to reconfigure")
}
```

### 4. Update the Interactive Wizard (`cmd/setup.go`)

#### State changes required

The current model uses a single space and flat folder map:

```go
selectedSpace   clickup.Space
selectedFolders map[string]string // id → name
```

Replace with multi-space state that preserves space context for each folder:

```go
selectedSpaces  []clickup.Space
foldersBySpace  map[string][]config.FolderConfig // spaceID → []FolderConfig
```

#### UX flow change for space selection

`stepSpace` currently uses single-select: pressing Enter on a space immediately advances to folder loading. For multi-select this must change:

1. Space list renders with checkboxes (same `itemDelegate` pattern already used for folders)
2. Space toggles on `" "` (space bar), same as folders
3. Enter confirms the selection and advances — add a guard: at least one space must be selected
4. The existing `stepFolder` label can be reused; title should read "Select Folders (Space to toggle, Enter to confirm)"

#### Multi-space folder fetching

After spaces are confirmed, folders must be fetched for all selected spaces. The current architecture fires one `foldersMsg` per space selection. For N spaces, use an accumulator pattern:

- Introduce a new message type `allFoldersMsg map[string][]clickup.Folder` (keyed by space ID)
- Fire one goroutine that iterates selected spaces, calls `GetFolders` for each, and returns the combined map
- On receiving `allFoldersMsg`, flatten all folders into a single list with "SpaceName / FolderName" display labels
- Store space context in `foldersBySpace` so the final config build knows which folder belongs to which space

#### Final config build (`stepDone`, lines 53–72)

Replace the current flat build with a nested one:

```go
spaces := make([]config.SpaceConfig, 0, len(m.selectedSpaces))
for _, space := range m.selectedSpaces {
    folders := m.foldersBySpace[space.ID] // already []config.FolderConfig
    spaces = append(spaces, config.SpaceConfig{
        ID:      space.ID,
        Name:    space.Name,
        Folders: folders,
    })
}
cfg := config.Config{
    WorkspaceID:   m.selectedWorkspace.ID,
    WorkspaceName: m.selectedWorkspace.Name,
    Spaces:        spaces,
}
```

The confirmation print should summarize spaces and folder counts, not a single space name.

### 5. Update CLI Commands

Replace every `cfg.Folders` iteration with `config.FlattenFolders(cfg)`. Replace every `cfg.SpaceName` reference. Update empty-state checks. Per-command specifics:

#### `cmd/tasks.go`
- Line 73: `len(cfg.Folders) == 0` → `!config.HasFolders(cfg)`
- Line 91: header `"Space: %s", cfg.SpaceName` → `"Workspace: %s", cfg.WorkspaceName` (or remove space name entirely since multiple spaces are now shown)
- Line 93: `for _, folder := range cfg.Folders` → `for _, folder := range config.FlattenFolders(cfg)` using `ConfiguredFolder`; update `folder.Name` to `folder.FolderName` and `folder.ID` to `folder.FolderID`
- Folder header label at line 97 can optionally include space context: `"[SpaceName] Folder: FolderName"`

#### `cmd/summarize.go`
- Line 67: `len(cfg.Folders) == 0` → `!config.HasFolders(cfg)`
- Line 78: `cfg.SpaceName` → `cfg.WorkspaceName`
- Line 80: `for _, folder := range cfg.Folders` → `config.FlattenFolders(cfg)`; update field references to `FolderID`/`FolderName`

#### `cmd/standup.go`
- Line 154: `for _, folder := range m.cfg.Folders` → `config.FlattenFolders(m.cfg)`; update field references
- `standupTask.folderName` is displayed as `folderName/listName` (lines 527, 573); consider prepending `SpaceName` here if users have same-named folders across spaces

#### `cmd/browse.go`
- Line 171: `for _, folder := range m.cfg.Folders` → `config.FlattenFolders(m.cfg)`; update field references
- `taskItem.Description()` at line 92 already shows `"Folder: ..."` — this remains correct; space context can be added if desired

#### `cmd/track.go`
- Line 187: `for _, folder := range m.cfg.Folders` → `config.FlattenFolders(m.cfg)`; update field references

#### `cmd/new.go`
- Line 35: `len(cfg.Folders) == 0` → `!config.HasFolders(cfg)`
- Lines 152–154: `cfg.Folders` → `config.FlattenFolders(cfg)`
- `folderItem` currently wraps `config.FolderConfig` — change it to wrap `config.ConfiguredFolder`
- Update `folderItem.Title()` to return `folder.FolderName`, `folderItem.Description()` to return `folder.SpaceName` (this gives space context in the list without extra labeling)
- Update `selectedFolder config.FolderConfig` on `newModel` to `selectedFolder config.ConfiguredFolder`; update all uses of `selectedFolder.ID`/`selectedFolder.Name` to `FolderID`/`FolderName`

#### `cmd/config.go` (show command)
- Lines 29–33: replace flat `SpaceName`/`SpaceID`/`Folders` output with hierarchical display:
  ```
  Workspace: Name (ID)
    Space: SpaceName (SpaceID)
      - FolderName (FolderID)
      - FolderName (FolderID)
    Space: SpaceName (SpaceID)
      - FolderName (FolderID)
  ```

---

## Verification

1. **Build first.** Run `go build ./...` before any other check. The current build is broken; all steps above must be applied before the build will pass.
2. **Tests.** Run `go test ./...`.
3. **Config serialization tests** (`pkg/config/config_test.go`): assert round-trip for one workspace with two spaces and two folders each; assert `FlattenFolders` returns the correct flat list; assert the migration guard returns the right error when `WorkspaceID` is set but `Spaces` is empty.
4. **Flattening helper tests**: verify `FlattenFolders` ordering and field mapping.
5. **Manual testing:**
   1. Run `./clickup-tui setup` and configure 1 workspace with 2 spaces and 2 folders each.
   2. Run `./clickup-tui show` to verify the configuration is grouped by workspace → space → folder.
   3. Run `./clickup-tui tasks` to verify tasks from both spaces load correctly and the header no longer references a single space name.
   4. Run `./clickup-tui new` to verify folder selection shows space context in the description column.
   5. Manually place an old-format config file (with `space_id`/`folders` at top level) and verify `./clickup-tui tasks` prints a clear migration message instead of "No folders configured."

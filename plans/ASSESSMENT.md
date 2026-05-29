# ClickUp TUI - Architecture Assessment & Feedback

**Date**: 2026-04-17  
**Assessment Against**: Golang TUI Best Practices & Architecture Plan

---

## Executive Summary

The clickup-tui application demonstrates solid foundational understanding of BubbleTea and the Charm ecosystem, with functional implementations of core features (standup, track, browse, tasks). However, the architecture significantly deviates from enterprise-grade TUI patterns, creating maintenance friction and architectural debt. 

**Critical**: The multi-program menu pattern breaks the TEA model's guarantees, introducing state loss and preventing proper navigation history.

**Note on Best Practices Reference**: The "Golang TUI Patterns and Best Practices.md" document is an excellent architectural foundation but **has notable gaps** in error handling patterns, dependency injection, testing strategies, logging, and refactoring guidance for existing apps. This assessment augments that reference with practical approaches not well-covered there.

---

## Critical Issues

### 1. **Multi-Program Menu Pattern (HIGH IMPACT)**

**Status**: ❌ Does not follow best practices  
**Location**: `cmd/menu.go:14-68`

**Problem**:
- The menu launches separate `tea.NewProgram` instances for each command, not a unified program
- Screen clearing (`\033[H\033[2J`) breaks the TEA contract
- Setting environment variable `CLICKUP_TUI_MENU="1"` to signal downstream commands is a fragile coupling mechanism
- Each subcommand runs independently with its own Program, destroying history and preventing proper back-navigation

**Best Practice Violation**:
From the Golang TUI best practices: "Because the BubbleTea runtime manages an internal event loop that continuously reads from message channels and writes to the terminal buffer, mutating the Model outside of the Update function introduces severe data races."

This design does the opposite—it exits and re-enters the BubbleTea runtime multiple times, losing the single event loop guarantee.

**Expected Approach** (Model Stack Controller Pattern):
```go
// Single root program with a stack of models
type RootModel struct {
    models []tea.Model  // Stack: [MenuModel, StandupModel, ...]
}

// When user selects "standup" from menu:
// Push standupModel onto stack, root delegates Update/View to active model
// When user presses Esc: Pop the model, return to menu with all state preserved
```

**Impact**: Users cannot navigate back cleanly; state is lost between screens; the application is not truly interactive.

---

### 2. **Value vs. Pointer Receiver Inconsistency (MEDIUM IMPACT)**

**Status**: ⚠️ Partially incorrect  
**Location**: `cmd/standup.go:151, 189, etc.`

**Problem**:
```go
func (m standupModel) Init() tea.Cmd { ... }
func (m standupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
```

- Using **value receivers** for `Init()`, `Update()`, and `View()` on `standupModel`
- `standupModel` is a large struct with nested `textarea.Model`, `[]standupTask`, etc.
- Each call creates a full copy of the struct, causing excessive memory allocation and GC pressure

**Best Practice**:
The Golang TUI best practices state: "Pointer receivers are acceptable and actively practiced in large codebases, provided that the cardinal rule of modifying state exclusively within the Update loop is rigidly respected."

**Fix**: Use pointer receivers consistently:
```go
func (m *standupModel) Init() tea.Cmd { ... }
func (m *standupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { ... }
```

**Impact**: Performance degradation during fast user input; unnecessary GC pauses.

---

### 3. **Blocking N+1 API Load in `track` Command (MEDIUM IMPACT)**

**Status**: ❌ Does not follow best practices  
**Location**: `cmd/track.go:163-229` (loadActivity function)

**Problem**:
```go
for _, folder := range m.cfg.Folders {
    lists, err := m.client.GetLists(folder.ID)      // Blocking call #1
    for _, listObj := range lists {
        tasks, err := m.client.GetRecentTasks(...)   // Blocking call #N
        for _, task := range tasks {
            // More API calls...
        }
    }
}
```

- All API calls are synchronous within a single `tea.Cmd` function
- Entire command blocks until all data is fetched (5-15 seconds reported)
- User sees a static spinner with no progress indication

**Best Practice** (Stateful Workflow Machine Pattern):
Break into discrete stages with progress messages:
```go
type Stage struct {
    Name          string      // "Fetching Folder 1...", "Processing Tasks...", etc.
    Action        func() error
    IsCompleteFunc func() bool // For idempotency
}

// Each stage completion emits progressMsg for UI re-render
// User sees: "Fetching Folder 1..." → "Fetching Folder 2..." → "Processing..."
```

**Impact**: UI appears frozen; no feedback during long operations; users assume app is broken.

---

### 4. **Error Handling Destroys UI State (CRITICAL IMPACT)**

**Status**: ❌ Does not follow best practices  
**Location**: `cmd/standup.go:233-235`, throughout all commands

**Problem**:
```go
case errMsg:
    m.err = msg
    return m, nil
```

The `View()` function checks `if m.err != nil` and renders a full-screen error, replacing the entire UI. User must quit to recover.

**Why This is Critical**:
The reference "Golang TUI Patterns" doc barely covers error handling (only p. 174), yet it's one of the hardest UX problems in TUIs. In clickup-tui, **any API failure destroys all context**—selection state, loaded data, cursor position—all lost.

**Best Practice** (Not well-covered in reference):
Errors should be:
1. **Non-modal by default**: Render in a persistent footer or inline notification
2. **Dismissible**: `Esc` or `Space` clears the error without quitting
3. **Actionable**: Include retry option (e.g., "Error: Network timeout. Press R to retry")
4. **Non-destructive**: Keep the previous UI rendered beneath; error is an overlay

**Expected Behavior**:
```
[Normal UI content (still fully visible)]
════════════════════════════════════════
[ERROR] Failed to fetch tasks: Network timeout
[R] Retry  [Esc] Dismiss  [Q] Quit
```

**Implementation Pattern**:
```go
type ErrorOverlay struct {
    msg     string
    retryFn func() tea.Cmd  // Re-execute the failed operation
    onDismiss func()         // Called when user presses Esc
}

// In root model's View():
if m.errorOverlay != nil {
    return m.renderWithErrorOverlay()  // Dim background, show error on top
}
return m.normalView()
```

**Impact**: Users lose all work on errors; errors feel unrecoverable; appears broken. This is a **dealbreaker for production TUIs**.

---

## Major Issues

### 5. **Hardcoded Dimensions & Missing WindowSizeMsg Propagation (MEDIUM)**

**Status**: ⚠️ Partially implemented  
**Location**: `cmd/standup.go:370-395` (viewSelect function)

**Problem**:
- `WindowSizeMsg` is captured but dimensions are stored in top-level model only
- Nested components (textarea) don't receive updated dimensions on resize
- Hardcoded padding and string widths cause wrapping and layout breakage on resize

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    return m, nil
    // Textarea does NOT get the resize event
```

**Best Practice** (Dynamic Dimensionality):
Every child model must receive `WindowSizeMsg`. Parent must:
1. Store window dimensions
2. Calculate exact footprint needed (borders + padding + margins)
3. Pass remaining space to children
4. Use `lipgloss.Width()` for dynamic width calculation

**Impact**: UI breaks when users resize their terminal; components overlap or disappear.

---

### 6. **Global Variables Coupling Commands & Models (LOW-MEDIUM)**

**Status**: ⚠️ Present  
**Location**: `cmd/standup.go:24-27`, `cmd/track.go:27-29`

```go
var (
    standupAll  bool
    standupMine bool
)
```

**Problem**:
- Global flags couple CLI parsing to TUI model initialization
- Violates the architecture principle of separation: "cmd/ should only handle routing"
- Hinders unit testing and composability

**Better Approach**:
Pass through explicit constructor injection:
```go
// In cmd/standup.go
m := tui.NewStandupModel(client, cfg, userID, standupAll, standupMine)

// Models are in internal/tui/ and don't reference global cmd state
```

**Impact**: Testability is harder; tightly coupled; refactoring requires changing cmd/ and tui/ simultaneously.

---

## Moderate Issues

### 7. **Menu Item Selection Coupling (LOW)**

**Status**: ⚠️ Fragile  
**Location**: `cmd/menu.go:36-50`

The menu model returns a choice string, then the outer code searches for a matching Cobra subcommand by name. This is fragile if commands are renamed or reorganized.

**Better**: Define a proper command type or mapping structure.

---

### 8. **Missing Progressive Disclosure (UX)**

**Status**: ✗ Not implemented  
**Location**: Throughout views

**Expected**: Footer showing only relevant keybindings for the current context, with `?` to expand. Currently missing.

---

### 9. **No Model Composition for Complex Views**

**Status**: ⚠️ Partial  

The `standup` and `track` commands are monolithic models that handle multiple states (loading, select, update, display). They would benefit from sub-models:
- `SelectionModel` (multi-select UI)
- `ProgressModel` (loading state with spinner)
- `DetailModel` (display and editing)

---

## Critical Gaps Not Addressed by Reference Doc

The "Golang TUI Patterns" document is strong on architecture but **weak on practical implementation details** for real-world refactoring. These gaps are particularly relevant to clickup-tui:

### 10. **Dependency Injection Pattern (Not Well-Covered)**

**Status**: ⚠️ Current approach is implicit  

**Problem**:
- Models receive `clickup.API`, `config.Config`, `ai.Summarizer` as constructor args—good.
- But there's no pattern for passing these through nested models or managing lifecycle.
- If you refactor to Model Stack Controller, you'll need to pass these dependencies through the root model to all children.

**Recommendation**:
Create a `Context` struct that travels with the root model:
```go
type AppContext struct {
    Client     clickup.API
    Config     config.Config
    Summarizer *ai.Summarizer
    Cache      *cache.Client
}

// Root model holds context
type RootModel struct {
    models  []tea.Model
    context *AppContext
}

// Child models request what they need
func NewStandupModel(ctx *AppContext, ...) *StandupModel {
    return &StandupModel{
        client: ctx.Client,
        config: ctx.Config,
    }
}
```

This avoids passing 5+ parameters through every constructor and makes dependency management explicit.

---

### 11. **Error Recovery & Retry Logic (Not Well-Covered)**

**Status**: ❌ Not implemented  

**Problem**:
Network errors are permanent in clickup-tui—user must restart the command. No retry logic or transient error handling.

**Recommendation**:
Implement backoff + retry for transient errors:
```go
type RetryableMsg struct {
    operation func() tea.Msg  // The failed operation
    attempt   int
    maxRetries int
}

// In Update():
case RetryableMsg:
    if msg.attempt < msg.maxRetries && isTransient(err) {
        // Exponential backoff: 100ms, 200ms, 400ms, ...
        return m, tea.Tick(time.Duration(100 * (1 << msg.attempt)) * time.Millisecond, func(t time.Time) tea.Msg {
            return RetryableMsg{operation: msg.operation, attempt: msg.attempt + 1, maxRetries: msg.maxRetries}
        })
    }
    // If max retries exceeded or non-transient: show error overlay
```

---

### 12. **Structured Logging in Async Context (Not Covered)**

**Status**: ❌ Not implemented  

**Problem**:
`pkg/logger/logger.go` exists but is not used effectively in async operations. Hard to trace issues across goroutines without structured logs.

**Recommendation**:
- Use structured logging (e.g., `slog` in Go 1.21+)
- Log at boundaries: "API call start", "message received", "state transition"
- Include trace IDs for async operations:

```go
type ContextMsg struct {
    msg     tea.Msg
    traceID string  // UUID, propagate through async chain
}

// When dispatching async command:
return func() tea.Msg {
    result, err := m.client.GetTasks(...)
    log.Info("API call completed", slog.String("traceID", traceID), slog.Int("tasks", len(result)))
    return ContextMsg{msg: TasksLoaded(result), traceID: traceID}
}
```

This makes debugging the 5-15 second load in `track` vastly easier.

---

### 13. **Testing Strategy for Async Models (Not Covered)**

**Status**: ❌ Minimal testing  

**Problem**:
`pkg/clickup/client_test.go` and `pkg/config/config_test.go` exist but no tests for TUI models. Testing async BubbleTea code is hard—reference doc covers `teatest` but not how to structure tests.

**Recommendation**:
Create test helpers for common patterns:
```go
// tests/helpers.go
func TestModelFlow(t *testing.T, model tea.Model, inputs []tea.Msg, expected State) {
    for _, msg := range inputs {
        var cmd tea.Cmd
        model, cmd = model.Update(msg)
        // Execute any commands synchronously in tests
        if cmd != nil {
            resultMsg := cmd()
            if resultMsg != nil {
                model, _ = model.Update(resultMsg)
            }
        }
    }
    // Assert final state
    if !stateEqual(model.CurrentState(), expected) {
        t.Fatalf("expected %v, got %v", expected, model.CurrentState())
    }
}

// Usage:
TestModelFlow(t, standupModel, []tea.Msg{
    standupTasksLoaded(tasks),
    tea.KeyMsg{Type: tea.KeyEnter},
    standupStatusesLoaded(statuses),
}, StandupUpdate)
```

---

### 14. **Project Structure & Package Organization (Not Covered)**

**Status**: ⚠️ Partially organized  

**Current Layout**:
```
cmd/        <- Cobra commands (also contains BubbleTea models!)
pkg/
  ├── ui/   <- Styles and spinner (good)
  ├── clickup/  <- API client
  ├── config/   <- Configuration
  ├── cache/    <- Caching layer
  └── ...
```

**Problem**:
BubbleTea models are mixed in `cmd/` with Cobra commands. This violates the principle of "cmd/ handles routing only."

**Recommendation**:
```
cmd/
  ├── root.go
  ├── menu.go
  └── subcommands/
       ├── standup.go (just flag parsing + initialization)
       └── track.go
       
internal/
  └── tui/
       ├── root.go          <- RootModel with stack
       ├── menu.go          <- MenuModel
       ├── standup/
       │    ├── model.go    <- StandupModel
       │    ├── update.go   <- Update logic
       │    ├── view.go     <- View logic
       │    └── messages.go <- Message types
       ├── track/
       │    ├── model.go
       │    └── ...
       ├── shared/
       │    ├── error_overlay.go
       │    ├── progress.go
       │    └── selection.go
       └── context.go       <- AppContext

pkg/
  ├── ui/
  ├── clickup/
  ├── config/
  └── ...
```

This makes testing, refactoring, and understanding data flow much easier.

---

### 15. **Incremental Refactoring Path (Not Covered)**

**Status**: 🔄 Applicable  

**Problem**:
The reference doc assumes greenfield. Refactoring from multi-program to single program is risky. How do you do it incrementally without breaking everything?

**Recommendation**:
See "Pragmatic Refactoring Approach" section below.

---

## Positive Aspects

### ✅ What's Done Well

1. **Proper Use of Custom Messages**: `standupTasksLoaded`, `standupStatusesLoaded`, `errMsg` types are well-defined
2. **Async Handling**: Blocking API calls are wrapped in `tea.Cmd` functions, preventing UI freeze (despite poor feedback)
3. **Spinner Integration**: Uses BubbleTea's spinner properly for loading states
4. **Styling**: `pkg/ui/styles.go` centralizes styling with Lipgloss (good separation of concerns)
5. **Caching Layer**: Recent addition of caching is aligned with performance best practices
6. **Configuration Injection**: Models receive `config.Config` explicitly, not global state
7. **Standup State Machine**: Good use of `standupState` constants and state-driven rendering

---

## Refactoring Priority & Roadmap

### Phase 1: Critical (Blocks UX & Architecture)

**1. Fix Value/Pointer Receiver Inconsistency** (Start here—fastest win)
   - Convert all BubbleTea models to pointer receivers
   - **Effort**: 2-4 hours | **Impact**: Medium (performance + correctness)
   - **Blockers**: None
   - **Files**: All `cmd/*.go` files with `Init()`, `Update()`, `View()` methods
   - **Why first**: Low risk, quick validation that refactoring is on track

**2. Implement Model Stack Controller Pattern** (Core refactoring)
   - Move from multi-program to single `tea.NewProgram` in main
   - Root model maintains `models []tea.Model` stack
   - Menu pushes new models, `Esc` pops back
   - **Effort**: 2-3 days | **Impact**: Critical
   - **Blockers**: Requires extracting models from `cmd/` to `internal/tui/`
   - **Files**: 
     - New: `internal/tui/root.go`, `internal/tui/menu/`, `internal/tui/standup/`, etc.
     - Modify: `main.go`, all `cmd/` subcommands (just initialization)
   - **Incremental Strategy**: See "Pragmatic Refactoring Approach" below

**3. Create AppContext for Dependency Injection**
   - Centralize client, config, cache in one struct
   - Pass through root model to all children
   - **Effort**: 4-6 hours | **Impact**: High (maintainability)
   - **Blockers**: None; can be done parallel with #2
   - **Files**: New: `internal/tui/context.go`, modify all model constructors

### Phase 2: High Impact (UX & Reliability)

**4. Implement Non-Destructive Error Overlays** (CRITICAL for reliability)
   - Create `ErrorOverlay` component that dims background + shows error
   - Retry logic with exponential backoff
   - Allow dismissal with `Esc`
   - **Effort**: 1-2 days | **Impact**: Critical
   - **Depends on**: Phase 1 completion (needs root model for overlay rendering)
   - **Files**: New: `internal/tui/shared/error_overlay.go`, modify all models

**5. Implement Stateful Workflow Machine for Long Operations** (track, standup)
   - Break `loadActivity()` into discrete stages with progress messages
   - Each stage: name, action, idempotency check, error handling
   - Emit progress messages between stages for UI updates
   - **Effort**: 1-2 days | **Impact**: High (UX)
   - **Depends on**: #1 (pointer receivers)
   - **Files**: `internal/tui/track/model.go`, `internal/tui/standup/model.go`

**6. Centralize WindowSizeMsg Propagation**
   - Root model broadcasts `WindowSizeMsg` to all active children
   - Implement priority collapse for responsive layouts
   - Test on 80-col and 200-col terminals
   - **Effort**: 1 day | **Impact**: Medium (resilience)
   - **Depends on**: Phase 1 (root model exists)
   - **Files**: `internal/tui/root.go`, all child models

### Phase 3: Cleanup & Polish (Technical Debt)

**7. Restructure Project Layout** (Already partially done)
   - Move models fully to `internal/tui/` with subpackages
   - Keep `cmd/` for Cobra routing only
   - **Effort**: 1 day | **Impact**: Low (maintenance + testability)
   - **Depends on**: Phases 1-2 complete

**8. Add Structured Logging** (for debugging)
   - Use `slog` for structured logs
   - Add trace IDs to async operations
   - Log at API boundaries and state transitions
   - **Effort**: 1 day | **Impact**: Medium (debuggability)
   - **Files**: Modify all models + API calls

**9. Add Unit Tests for Model Logic** (using teatest)
   - Test State machines (standup flow, track flow)
   - Test error recovery and retry logic
   - Use golden files for View output
   - **Effort**: 2-3 days | **Impact**: High (confidence)
   - **Depends on**: Phases 1-2 (stable models)
   - **Files**: New: `internal/tui/*/model_test.go`

**10. Add Progressive Disclosure (Help Footer)**
   - Integrate Bubbles' `help` component
   - Add `?` modal for full keybindings
   - **Effort**: 4-6 hours | **Impact**: Low (UX polish)
   - **Files**: `internal/tui/shared/help.go`, modify all models

---

## Pragmatic Refactoring Approach (Incremental Strategy)

Refactoring from multi-program to single program is risky. **Don't try to do it all at once.** Here's a low-risk, incremental path:

### Week 1: Foundation
**Step 1.1**: Fix pointer receivers (2-4 hours)
- Simply change `func (m Model)` to `func (m *Model)` throughout
- Verify: `go test ./...`
- **Commit**: "refactor: use pointer receivers for BubbleTea models"

**Step 1.2**: Create `AppContext` struct (4-6 hours)
- New file: `internal/tui/context.go`
- Change all model constructors to accept `*AppContext`
- Update all `cmd/` subcommands to build context once and pass it
- **Commit**: "refactor: centralize dependency injection with AppContext"

**Step 1.3**: Extract StandupModel to `internal/tui/standup/` (4-6 hours)
- Move `standupModel`, `standupTask`, etc. to new package
- Rename `cmd.standupModel` → `standup.Model`
- Update imports, keep CLI behavior identical
- **Commit**: "refactor: move StandupModel to internal/tui/standup"

**Pause & Test**: At this point, all commands still work as separate programs. The refactoring is NOT breaking. Users don't know anything changed. You've validated the extraction pattern.

### Week 2: Root Model
**Step 2.1**: Create Root Model & Model Stack (1 day)
- New file: `internal/tui/root.go`
- `RootModel` holds `models []tea.Model`, starts with `MenuModel`
- Implement `Update()` to delegate to active model
- Implement `View()` to render active model
- **Still using multi-program**: Keep `cmd/standup.go` creating its own Program for now
- **Commit**: "feat: create RootModel with model stack"

**Step 2.2**: Migrate one command to use RootModel (standup) (1 day)
- In `cmd/standup.go`: Instead of `tea.NewProgram(standupModel)`, create `RootModel` starting with standup
- Test: `clickup-tui standup` should work as before, but now uses unified program
- **Commit**: "feat: migrate standup command to RootModel"

**Step 2.3**: Test back-navigation (2 hours)
- Verify that `Esc` in standup pops the model and returns (but we won't have a menu yet, so it just quits)
- Add menu to the stack and test full flow
- **Commit**: "feat: add back-navigation to root model"

### Week 3: Menu Integration
**Step 3.1**: Create MenuModel in `internal/tui/menu/` (4-6 hours)
- Copy the list-based menu logic from `cmd/menu.go`
- Return a custom message like `MenuSelectedMsg{command: "standup"}` instead of quitting
- Update RootModel to push the selected model onto the stack

**Step 3.2**: Migrate all remaining commands (standup, track, browse, etc.) (2-3 hours each)
- Follow same pattern as standup
- Each command now uses RootModel starting with its own model
- Test: `clickup-tui menu` → select standup → use standup → press Esc → back to menu

**Step 3.3**: Unify all commands under single entry point (2-4 hours)
- Update `main.go` or `root.go` to start with MenuModel by default
- Keep individual command paths for CI/CD (clickup-tui standup --no-menu)
- **Commit**: "feat: unified single-program architecture with model stack"

### Week 4: Polish & Hardening
**Step 4.1**: Error overlays + retry logic (1-2 days)
- Implement `ErrorOverlay` component
- Update all models to show errors instead of crashing
- Add retry handlers

**Step 4.2**: Workflow machines for long operations (1 day)
- Refactor `loadActivity()` in track to emit progress
- Add stage-based loading with per-stage error handling

**Step 4.3**: Window resize propagation (1 day)
- Root broadcasts `WindowSizeMsg` to all children
- Test on different terminal sizes

**Step 4.4**: Tests & logging (2-3 days)
- Unit tests for critical flows (standup task selection, track retry)
- Structured logging at boundaries

---

### Validation Checkpoints
- **After Week 1**: All unit tests pass; individual commands still work
- **After Week 2**: Single RootModel handles one command; back-nav works
- **After Week 3**: All commands use unified program; menu integrates cleanly
- **After Week 4**: Errors are recoverable; long operations show progress; tests cover critical flows

**Risk Mitigation**:
- Each step is mergeable on its own (backward compatible)
- You can rollback any week if issues arise
- Users don't notice changes until Week 3 (the menu behavior change is transparent)

---

## Concrete Examples

### Current (Broken) Multi-Program Menu
```go
// menu.go
p := tea.NewProgram(m, tea.WithAltScreen())  // Program 1
res, _ := p.Run()
fmt.Print("\033[H\033[2J")                     // Screen clear (destructive)
root.SetArgs([]string{finalModel.choice})
root.Execute()                                 // Program 2 (standup/track/etc)
// History is lost; state is lost; back-navigation is impossible
```

### Expected (Single Program Stack)
```go
// main.go
root := tui.NewRootModel()  // Stack starts with MenuModel
p := tea.NewProgram(root, tea.WithAltScreen())
p.Run()

// tui/root.go
func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    active := m.models[len(m.models)-1]
    newModel, cmd := active.Update(msg)
    
    // If user pressed Esc and we want to go back:
    if isBackMsg(msg) && len(m.models) > 1 {
        m.models = m.models[:len(m.models)-1]  // Pop the stack
    }
    
    m.models[len(m.models)-1] = newModel
    return m, cmd
}

// When standup is done:
// Simply pop from stack and return to menu (all state preserved)
```

---

---

## Reference Doc Gaps & Implications for ClickUp-TUI

This assessment surfaced **significant limitations** in the "Golang TUI Patterns & Best Practices.md" document that directly impact your refactoring strategy:

| Gap | Impact on ClickUp-TUI | Solution |
|-----|----------------------|----------|
| **Error Handling** (1 paragraph, p. 174) | No pattern for overlays or retry logic; errors destroy UI | Implement ErrorOverlay + backoff retry (detailed above) |
| **Dependency Injection** (brief, p. 31) | Unclear how to pass dependencies through nested models | Create AppContext struct pattern |
| **Testing Strategies** (visual only, p. 187-192) | No guidance on unit testing Update() logic | Create test helpers + teatest golden files |
| **Logging in Async Ops** (Not covered) | Hard to debug the 5-15 second freeze in track command | Add structured logging with trace IDs |
| **Project Structure** (1 sentence, p. 27) | Unclear where models should live; cmd/ is conflated with tui/ | Reorganize: cmd/ = routing, internal/tui/ = models |
| **Refactoring Existing Apps** (Not covered) | Doc assumes greenfield; no path for migrating multi-program → single-program | Provide 4-week incremental strategy (above) |

**Implication**: Don't treat the reference doc as complete. Use it for architectural foundation, but augment with the patterns above for practical implementation.

---

## Conclusion

**Current State**: The codebase is **functional but architecturally unsound**. It demonstrates good understanding of individual BubbleTea components but violates the core TEA principle: one program, one event loop, pure state management.

**Critical Finding**: The reference "Golang TUI Patterns & Best Practices" document, while excellent on architecture, **has significant gaps** in error handling, dependency injection, testing, and refactoring guidance. This assessment fills those gaps with practical patterns from production TUI work.

**Key Priorities** (in order):
1. **Pointer receivers** (quick win, validates approach)
2. **RootModel + Model Stack** (core architectural fix)
3. **Error overlays** (dealbreaker for production)
4. **Structured logging & tests** (confidence for users)

**Realistic Timeline** (following pragmatic incremental approach):
- **Week 1**: Foundation (pointer receivers, AppContext, one model extraction) = 1-2 days work
- **Week 2**: Root model + single command migration = 2 days work
- **Week 3**: Menu integration + all commands = 2-3 days work
- **Week 4**: Error handling, workflow machines, tests = 3-4 days work
- **Total**: ~4 weeks elapsed, ~10-12 days actual work (can be done part-time)

**Key Difference from Original Roadmap**: This update emphasizes **incremental, non-breaking steps** rather than a big rewrite. Each week is independently valuable and mergeable, reducing risk.

**Do Not Attempt**: A "rewrite everything at once" approach. That's guaranteed to break things and is unnecessary. The incremental path is safer and lets you validate each architectural decision.

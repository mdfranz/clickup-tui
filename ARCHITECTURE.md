# System Architecture

This document provides a detailed overview of the system architecture, component layers, data flows, and design patterns utilized in `clickup-tui`.

---

## 1. Architectural Overview

`clickup-tui` is a terminal user interface (TUI) application constructed using Go, Cobra, and Bubble Tea. It operates as a local agent, interacting with the **ClickUp REST API** for task and workspace management and the **Gemini API** for AI summarization.

To deliver responsive performance, the application implements a layered architecture, a thread-safe local JSON caching subsystem, and async background processes.

```mermaid
flowchart TD
    %% User and Shell
    U([User Terminal]) <--> CMD[Cobra CLI Parser]

    %% CLI and Orchestration
    subgraph CLI ["CLI Layer (cmd/)"]
        CMD -->|Config Loaded| MC[menuCmd]
        CMD -->|Command Routing| SC[Subcommands]
        SC -->|tasks / browse| TUI_L[Bubble Tea TUI Program]
        SC -->|summarize / team-status| AI_L[AI Summarizer Service]
        SC -->|cache / clean / show| UTIL_L[Utility Commands]
    end

    %% TUI Architecture
    subgraph TUI ["Interactive TUI Layer (Bubble Tea / Lipgloss)"]
        TUI_L -->|Model| MV[Model State]
        TUI_L -->|Update| UP[Update Loop]
        TUI_L -->|View| VW[View Renderer]
        UP -->|Dispatches| CMD_CH[tea.Cmd Async Triggers]
        VW -->|Displays| U
    end

    %% Core Services and Cache
    subgraph Core ["Core Service & Cache Layer"]
        CMD_CH -.->|Requests| CC[Cached Client]
        AI_L -->|Requests| CC
        CC -->|1. Hit/Incremental| CS[(JSON Local Store)]
        CC -->|2. Miss / Fresh| CL[ClickUp REST Client]
    end

    %% Remote Endpoints
    subgraph Remote ["Remote API Gateways"]
        CL <-->|HTTPS REST| CU_API{{ClickUp API v2}}
        AI_L <-->|HTTPS Rest / RPC| GEM_API{{Gemini Developer API}}
    end

    %% Styling
    classDef layer fill:#1d2021,stroke:#3b4252,stroke-width:1px,color:#d8dee9;
    classDef component fill:#2e3440,stroke:#88c0d0,stroke-width:1.5px,color:#eceff4;
    classDef storage fill:#3b4252,stroke:#a3be8c,stroke-width:1.5px,color:#eceff4;
    classDef remote fill:#2e3440,stroke:#bf616a,stroke-width:1.5px,color:#eceff4;

    class CLI,TUI,Core,Remote layer;
    class CMD,MC,SC,TUI_L,MV,UP,VW,CMD_CH,AI_L,CC,CL component;
    class CS storage;
    class CU_API,GEM_API remote;
```

---

## 2. Component Layers

### A. The CLI Layer (`cmd/`)
The CLI entry point is orchestrated by **Cobra**. 
- **Command Router**: Parses console arguments and options (`cmd/root.go`). It configures global, persistent flags like `--refresh` or `--clear-cache` to bypass or invalidate the local cache.
- **Menu System**: `clickup-tui menu` functions as an interactive subcommand hub, launching nested Bubble Tea programs. It retains control over the shell's active screen via alternative screen buffers (`tea.WithAltScreen`).
- **Configuration Hook**: Inspects the presence of TOML configuration on startup, throwing user-friendly guides prompting `clickup-tui setup` if missing.

### B. The TUI Layer (`pkg/ui/` & Bubble Tea Models)
The interactive interface implements the **Elm Architecture (Model-Update-View)** via the `bubbletea` package:
- **Model**: Structures the visual state (cursors, input values, active spinners, list dimensions).
- **Update**: A pure-ish message dispatcher that captures terminal events, input keys, and asynchronous completions (`tea.Msg`), modifying the state and returning commands (`tea.Cmd`).
- **View**: A declarative renderer compiling Lipgloss definitions into clean, styled ANSI terminal outputs.
- **Key Screens & Components**:
  - `browse.go`: Implements a horizontal split-pane. The left pane uses the `list` bubble, and the right uses a scrollable `viewport`. Real-time, debounce-supported comments fetching triggers asynchronously as the user navigates.
  - `standup.go`: Features a sequential, multi-step state machine (`standupState`) transitioning from multi-select task lists to text input fields and custom status pickers.

### C. The Caching Subsystem (`pkg/cache/`)
To bypass aggressive API rate-limiting and accelerate response rendering, `clickup-tui` operates a localized caching layer wrapping the raw API client.

- **Storage Structure (`Store`)**: Fully structured and serialized to a single JSON file located at `$XDG_CACHE_HOME/clickup-tui/cache.json` (or OS cache fallback).
- **Dual Caching Strategy**:
  1. **TTL-Based Caching**: Used for high-level organizational structure (Users, Teams, Folders, Lists, Comments, Task Details). If the entry age exceeds its designated time-to-live threshold, a fresh API query is dispatched.
  2. **Timestamp-Based Incremental Loading**: Active task arrays are cached alongside a high-water mark (`MaxDateUpdated`). Instead of fetching complete listings on refresh, the client issues requests with `date_updated_gt=MaxDateUpdated`, merging newly created or edited task updates incrementally into the local dataset.
- **Thread-Safety & Atomicity**: The cached client employs a memory-based structure written atomically. To prevent data corruption on crash, the flushing sequence writes data to `.cache.json.tmp` and performs an atomic POSIX file rename (`os.Rename`) onto the final file path.

### D. The AI summarizer Layer (`pkg/ai/`)
Integrates the terminal with Gemini Generative LLM APIs using LangChainGo (`github.com/tmc/langchaingo`).
- **Summarizer Services**: Formulates structured, prompt-engineered contexts using ClickUp task descriptions, assignees, dates, and discussion threads.
- **Context Injection**: Truncates lengthy descriptions and limits context arrays to avoid exceeding model token constraints.
- **Objective Summaries**: Employs strict prompt guidelines requesting objective, factual summaries devoid of hyperbolic language or subjective filler.

---

## 3. Core Data Flows

### A. The API Query Lifecycle
The following sequence details how the application retrieves data under caching policies:

```mermaid
sequenceDiagram
    autonumber
    participant TUI as Bubble Tea (TUI)
    participant CC as Cached Client
    participant Store as JSON Local Store
    participant CU as ClickUp API

    TUI->>CC: GetTasks(ListID)
    CC->>Store: Load cache.json (in-memory)
    
    alt Force Refresh (noCache = true)
        CC->>CU: GetTasks(ListID, include_closed=true)
        CU-->>CC: Return Fresh Tasks
        CC->>Store: Merge and Write cache.json (Atomic)
    else Use Cache
        Store-->>CC: Return Cached Task Metadata
        CC->>CU: GetTasks(ListID, date_updated_gt=MaxDateUpdated)
        CU-->>CC: Return Incremental Updates
        alt Updates Received
            CC->>Store: Merge Updates & Write cache.json
        end
    end
    
    CC-->>TUI: Return Aggregated Task List
    TUI->>TUI: Render Task Tree / Dual Pane
```

### B. Guided Daily Standup Workflow
The daily standup workflow acts as a linear state machine traversing multiple screens:

```mermaid
stateDiagram-v2
    [*] --> Loading: clickup-tui standup
    Loading --> SelectTasks: Fetch active tasks from cache/API
    SelectTasks --> UpdateTask_1: Multi-select with Space & confirm with Enter
    
    state UpdateTask_1 {
        [*] --> FetchComments: Async read last 3 comments
        FetchComments --> AddComment: Type comment in Textarea
        AddComment --> PickerOverlay: Press Tab to change status
        PickerOverlay --> AddComment: Select status & Enter
        AddComment --> [*]: Press Ctrl+S to submit
    }
    
    UpdateTask_1 --> PostingUpdates: Loop through all selected tasks
    PostingUpdates --> Complete: Post comments & update statuses via API
    Complete --> [*]: View summary & Press Q to quit
```

---

## 4. Key Design Decisions

1. **SQLite vs. Flat-file JSON Caching**: 
   Since the configuration space and active task volume are bounded, a flat-file JSON serialization store was chosen over SQLite to eliminate CGO build requirements and simplify deployment across diverse platforms.
2. **Atomic Writes**:
   All modifications to configuration files or caches utilize temporary scratch files and atomic renaming. This guarantees files are never left half-written in the event of a sudden exit or loss of power.
3. **No-Blocking UI Loop**:
   Heavy queries—such as fetching comments or compiling team statuses—are handled in Go routines wrapped inside `tea.Cmd`. The TUI loop renders an animated Lipgloss spinner and remains interactive, avoiding UI freezes.

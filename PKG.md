# 3rd-Party Dependencies

This document tracks, classifies, and explains the 3rd-party libraries and modules used in `clickup-tui` to build the TUI, interface with the ClickUp and Gemini APIs, and manage configurations.

---

## 1. Terminal UI Ecosystem (Charmbracelet)

The visual design and interactivity of `clickup-tui` are built on the modern Charmbracelet stack.

| Library | Type | Purpose / Usage in ClickUp TUI |
| :--- | :--- | :--- |
| `github.com/charmbracelet/bubbletea` | **Direct** | Core TUI framework utilizing the Elm Architecture (Model-Update-View) to run the interactive CLI commands. |
| `github.com/charmbracelet/bubbles` | **Direct** | Standard interactive UI elements, including list search menus, text inputs, multiline textareas, and active loading spinners. |
| `github.com/charmbracelet/lipgloss` | **Direct** | Style definitions, layouts, text alignments, borders, and custom foreground/background coloring. |
| `github.com/charmbracelet/glamour` | **Direct** | Markdown-to-ANSI renderer used to display styled, formatted AI task and team summaries directly in the terminal. |
| `github.com/muesli/termenv` | *Indirect* | Advanced terminal color profile support and ANSI escape sequence creation. |
| `github.com/muesli/reflow` | *Indirect* | Text wrapping, truncation, and indenting utilities for adjusting layout widths dynamically on terminal resize. |
| `github.com/rivo/uniseg` | *Indirect* | Unicode boundary rules handling (grapheme clusters) to ensure correct text-width measurements. |
| `github.com/mattn/go-runewidth` | *Indirect* | Double-width character (e.g. East Asian symbols) terminal cell-width calculation support. |

---

## 2. Command Line Interface (CLI) Engine

The CLI structure, command routing, and flag parsing are powered by the standard Go CLI stack.

| Library | Type | Purpose / Usage in ClickUp TUI |
| :--- | :--- | :--- |
| `github.com/spf13/cobra` | **Direct** | Command router providing commands (`setup`, `tasks`, `browse`, `standup`, `summarize`, etc.), help screens, auto-completion, and command execution middleware. |
| `github.com/spf13/pflag` | *Indirect* | POSIX-compliant flag parsing (`-r` / `--refresh`, `--mine`, `-d` / `--days`). |

---

## 3. Artificial Intelligence & LLMs

The AI summarization engines are powered by LangChainGo and the official Google Generative AI bindings.

| Library | Type | Purpose / Usage in ClickUp TUI |
| :--- | :--- | :--- |
| `github.com/tmc/langchaingo` | **Direct** | Go LLM framework used to interface with Gemini models (`gemini-3.1-flash-lite-preview` or equivalent) for folder activity and team status summaries. |
| `cloud.google.com/go/ai` | *Indirect* | Official Google Cloud Generative AI client backend used by `langchaingo` under the hood. |
| `github.com/google/generative-ai-go` | *Indirect* | Google's official Go client for the Gemini API. |
| `google.golang.org/api` | *Indirect* | Low-level Google API transports, OAuth2 helpers, and endpoint discovery. |
| `pkoukk/tiktoken-go` | *Indirect* | BPE tokenization counting tool used to measure context window sizes for text models. |

---

## 4. Configuration & Serialization

| Library | Type | Purpose / Usage in ClickUp TUI |
| :--- | :--- | :--- |
| `github.com/pelletier/go-toml/v2` | **Direct** | Fast, spec-compliant TOML v2 loader and serializer used to read and update files in `~/.config/clickup-tui/config.toml`. |

---

## 5. Low-Level System & Text Utilities

These packages leverage official Go sub-repositories for low-level operating system bindings and text operations.

| Library | Type | Purpose / Usage in ClickUp TUI |
| :--- | :--- | :--- |
| `golang.org/x/term` | **Direct** | Queries current terminal sizes (`term.GetSize`) to dynamically resize responsive columns and wrap markdown lines. |
| `golang.org/x/text` | **Direct** | Text styling, unicode normalization, and locale-specific casing (`cases.Title`). |
| `golang.org/x/sys` | *Indirect* | Low-level system-specific calls for Unix / macOS / Windows terminal setups. |

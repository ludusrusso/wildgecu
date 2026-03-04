# gonesis

A bootstrappable AI agent with personality and identity, built in Go. On first run, the agent interviews you to discover who it should be, then writes its own soul to disk. Every session after that, it wakes up already knowing itself.

## Features

- **Soul system** — The agent bootstraps its own identity through a conversational interview, stored as `SOUL.md`
- **Provider abstraction** — LLM-agnostic design behind a simple `Provider` interface (ships with Google Gemini)
- **Streaming TUI** — Real-time chat interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), with streaming token output
- **Tool framework** — Agents can call tools during conversation; the bootstrap itself uses a `write_soul` tool
- **Agent loop** — Built-in agentic loop that handles tool calls, execution, and re-prompting automatically
- **Background daemon** — Long-running daemon with health checks, self-update, and system service support
- **Unified CLI** — Single binary with subcommands for chat, daemon management, and service lifecycle

## How it works

```
First run:                              Every run after:

┌─────────────┐                         ┌─────────────┐
│  No SOUL.md │                         │ Load SOUL.md│
└──────┬──────┘                         └──────┬──────┘
       │                                       │
       ▼                                       ▼
┌─────────────────┐                     ┌─────────────────┐
│   Bootstrap TUI │                     │  Build system   │
│  (interview you)│                     │  prompt from    │
│                 │                     │  AGENT + SOUL   │
└──────┬──────────┘                     │  + USER (opt.)  │
       │                                └──────┬──────────┘
       ▼                                       │
┌─────────────────┐                            ▼
│  Agent calls    │                     ┌─────────────────┐
│  write_soul     │                     │    Chat TUI     │
│  → .gonesis/    │                     │  (normal mode)  │
│    SOUL.md      │                     └─────────────────┘
└──────┬──────────┘
       │
       ▼
    Chat TUI
```

**Bootstrap phase**: The agent receives a system prompt (BOOTSTRAP.md) that guides it to ask about your agent's name, purpose, personality, expertise, and boundaries. After a few exchanges, it calls the `write_soul` tool to persist its identity.

**Normal mode**: The system prompt is assembled from three parts — base behavior (AGENT.md), the agent's identity (SOUL.md), and optional user preferences (USER.md).

## Prerequisites

- Go 1.25.5+
- A [Google Gemini API key](https://aistudio.google.com/apikey)

## Getting started

```bash
git clone https://github.com/ludusrusso/gonesis.git
cd gonesis

export GEMINI_API_KEY="your-api-key"

go run .
```

On first run, the agent will start a bootstrap conversation to establish its identity. Answer a few questions and it will write `.gonesis/SOUL.md` automatically, then switch to normal chat mode.

## CLI commands

Gonesis is a single binary. Chat is the default command; daemon management is available as subcommands.

```bash
# Chat (default)
gonesis              # interactive chat session
gonesis chat         # same thing, explicit

# Daemon lifecycle
gonesis start        # start the background daemon
gonesis stop         # stop the daemon
gonesis restart      # stop + start
gonesis status       # show daemon status (pid, uptime, version)
gonesis health       # exit 0 if daemon is healthy, 1 otherwise
gonesis logs         # show last 50 log lines
gonesis logs -f      # follow log output

# System service
gonesis install      # install as a system service
gonesis uninstall    # remove the system service

# Self-update
gonesis update --url <binary-url>   # trigger a self-update
```

Build with a version tag:

```bash
go build -ldflags "-X gonesis/cmd.Version=1.0.0" -o gonesis .
```

## Configuration

Gonesis uses a unified home directory at `~/.gonesis/` for all global state.

### Global files (`~/.gonesis/`)

| File | Purpose |
|---|---|
| `gonesis.yaml` | Configuration (API key, model, base folder) — created on first run |
| `gonesis.pid` | Daemon PID file |
| `gonesis.sock` | Daemon Unix domain socket |
| `gonesis.log` | Daemon log file (JSON) |

### Project files (`.gonesis/` in working directory)

| File | Purpose |
|---|---|
| `SOUL.md` | Agent identity — created during bootstrap |
| `USER.md` | Optional user preferences — create manually to pass context about yourself |

Delete `SOUL.md` to re-run the bootstrap and give your agent a new identity.

### Config file

The config file is searched in order: `./gonesis.yaml`, then `~/.gonesis/gonesis.yaml`. Override with `--config`:

```bash
gonesis --config /path/to/config.yaml
```

Environment variables also work:

```bash
export GEMINI_API_KEY="your-key"
```

## Architecture

```
gonesis.go                  # Entry point → cmd.Execute()
│
├── cmd/                    # CLI layer (Cobra)
│   ├── root.go             # Root command, config init, Version var
│   ├── chat.go             # chat subcommand (also default)
│   ├── start.go            # start subcommand + runDaemon()
│   ├── stop.go             # stop subcommand
│   ├── restart.go          # restart subcommand
│   ├── status.go           # status subcommand
│   ├── health.go           # health subcommand
│   ├── logs.go             # logs subcommand + readLastLines()
│   ├── update.go           # update subcommand
│   ├── install.go          # install subcommand
│   ├── uninstall.go        # uninstall subcommand
│   ├── setsid_unix.go      # reExecDetached() for Unix
│   └── setsid_windows.go   # reExecDetached() stub for Windows
│
├── agent/                  # Agent logic
│   ├── agent.go            # Run() — orchestrates bootstrap → chat
│   ├── bootstrap.go        # Bootstrap interview + write_soul tool
│   ├── soul.go             # Soul I/O and system prompt assembly
│   ├── prompt.go           # Embeds AGENT.md and BOOTSTRAP.md
│   ├── AGENT.md            # Base agent behavior prompt
│   └── BOOTSTRAP.md        # Bootstrap conversation prompt
│
├── x/config/               # Shared config package
│   └── config.go           # GlobalHome, GlobalFilePath, ProjectDir,
│                           # ProjectFilePath, EnsureConfigFile
│
├── internal/daemon/        # Daemon infrastructure
│   ├── daemon.go           # Run() — main loop, socket server, signal handling
│   ├── pidfile.go          # PID file management (uses x/config)
│   ├── client.go           # IPC client (uses x/config for socket path)
│   ├── socket.go           # Unix socket server + command dispatch
│   ├── service.go          # System service integration (kardianos/service)
│   ├── watchdog.go         # Periodic health checker
│   └── updater.go          # Self-update via binary replacement
│
├── provider/               # LLM provider abstraction
│   ├── provider.go         # Provider interface, types
│   ├── agent.go            # RunAgentLoop / RunAgentLoopStream
│   └── gemini/
│       └── gemini.go       # Google Gemini implementation
│
├── chat/
│   └── chat.go             # Config, RunTurn, RunTurnStream
│
└── tui/
    ├── tui.go              # Bubble Tea Model
    ├── messages.go          # Internal message types
    └── styles.go            # Lipgloss styling
```

### Key design decisions

- **Single binary**: All commands (chat, daemon management, service lifecycle) are subcommands of one `gonesis` binary — no separate `cmd/agent/` binary.
- **Unified home (`~/.gonesis/`)**: Config, PID, socket, and logs all live under one directory, managed by `x/config`.
- **`x/config` package**: Zero-dependency (stdlib only) shared package that all other packages import for path resolution. Prevents scattered `os.UserHomeDir()` + `filepath.Join()` patterns.
- **Project-local `.gonesis/`**: Per-project identity files (`SOUL.md`, `USER.md`) stay in the working directory, separate from global daemon state.

## Adding a new provider

Implement the `provider.Provider` interface:

```go
type Provider interface {
    Generate(ctx context.Context, params *GenerateParams) (*Response, error)
}
```

For streaming support, also implement `StreamProvider`:

```go
type StreamProvider interface {
    Provider
    GenerateStream(ctx context.Context, params *GenerateParams) (<-chan StreamChunk, <-chan error)
}
```

Then wire it up in `cmd/chat.go` instead of the Gemini provider.

## License

See [LICENSE](LICENSE) for details.

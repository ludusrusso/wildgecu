# WildGecu 🦎

**Wild by nature, safe by design.**

An open-source, multi-provider AI agent framework written in Go.

## Why "WildGecu"?

In Salento — the sun-scorched heel of Italy's boot — the gecko is everywhere. You'll find it clinging to the ancient dry stone walls (*muretti a secco*), perched on the warm tufa of baroque churches, navigating crumbling farmhouses at dusk. Locals call it *gecu*, and it has been a symbol of this land for centuries: resilient, adaptable, quietly useful.

The Mediterranean house gecko (*Tarentola mauritanica*) — whose scientific name traces back to Taranto, the gateway to Salento — owes its remarkable abilities to a simple, elegant mechanism: millions of microscopic lamellae on its toe pads that exploit Van der Waals forces to grip any surface, at any angle, without glue or suction. It doesn't need permission to climb. It just holds on.

That's the idea behind WildGecu. An AI agent framework that attaches to any surface — Anthropic, OpenAI, Ollama, or whatever comes next — and doesn't let go. One that runs wild and free as open-source software, but is engineered to be safe, predictable, and secure by default. One that lives quietly in the background, like a gecko on a warm wall, doing its work without fuss.

**Wild** because it's free, open, and untamed by vendor lock-in.
**Gecu** because every good project deserves a name that sounds like home.

## What is WildGecu?

WildGecu is a modular AI agent framework in Go. It provides a reusable foundation for building autonomous agents with:

- **Multi-provider support** — LLM-agnostic design behind a clean `Provider` interface (ships with Google Gemini, OpenAI, and Ollama)
- **Soul** — persistent identity bootstrapped through a conversational interview, stored as Markdown
- **Memory** — persistent context across sessions with automatic curation after each conversation
- **Skills** — a plugin system with lazy-loaded Markdown-based definitions and YAML frontmatter
- **Cron jobs** — an in-process scheduler with isolated sessions, powered by gocron
- **Parallel tool calling** — concurrent execution of independent tool calls within the agent loop
- **Telegram bridge** — daemon-based chat via Telegram bot
- **Self-update** — the agent can update its own binary at runtime
- **Background daemon** — long-running process with health checks, IPC socket, and system service support

No database required. File-based state. One binary. Your keys, your data, your gecko.

## How it works

WildGecu operates in three primary modes: **Bootstrap**, **Chat**, and **Code**.

```
wildgecu init:                          wildgecu chat / code:

┌─────────────┐                         ┌─────────────┐
│  No SOUL.md │                         │ Load SOUL.md│
└──────┬──────┘                         │ + MEMORY.md │
       │                                └──────┬──────┘
       ▼                                       │
┌─────────────────┐                     ┌─────────────────┐
│   Bootstrap TUI │                     │  Build system   │
│  (interview you)│                     │  prompt from    │
│                 │                     │  AGENT + SOUL   │
└──────┬──────────┘                     │  + USER + MEM   │
       │                                └──────┬──────────┘
       ▼                                       │
┌─────────────────┐              ┌─────────────┴─────────────┐
│  Agent calls    │              ▼                           ▼
│  write_soul     │      ┌─────────────────┐         ┌─────────────────┐
│  → .wildgecu/   │      │    Chat TUI     │         │    Code TUI     │
│    SOUL.md      │      │  (normal mode)  │         │  (working dir)  │
└──────┬──────────┘      └──────┬──────────┘         └──────┬──────────┘
       │                        │                           │
       ▼                        └─────────────┬─────────────┘
    Chat TUI                                  ▼
                                     ┌─────────────────┐
                                     │ Memory curation │
                                     │ → .wildgecu/    │
                                     │   MEMORY.md     │
                                     └─────────────────┘
```

**Bootstrap mode (`wildgecu init`)**: The `init` command starts an interactive interview where the agent asks about your agent's name, purpose, personality, and expertise. The agent receives a system prompt (`BOOTSTRAP.md`) that guides the conversation. After a few exchanges, it calls the `write_soul` tool to persist its identity in `.wildgecu/SOUL.md`. If `SOUL.md` already exists, the command exits with an error — delete it first to re-initialize.

**Chat mode (`wildgecu chat`)**: The default conversational mode. The system prompt is assembled from the base behavior (`AGENT.md`), the agent's identity (`SOUL.md`), and persistent memory (`MEMORY.md`).

**Code mode (`wildgecu code`)**: A specialized mode focused on development. The agent uses a different system prompt (`CODE_AGENT.md`) and is equipped with file-system tools (read, write, list, update) and a bash environment scoped to the current working directory.

**Memory curation**: After each session, a dedicated memory agent reviews the conversation and updates `MEMORY.md` — extracting key patterns, preferences, and context while keeping it concise.

## Prerequisites

- Go 1.26+
- At least one LLM provider configured:
  - [Google Gemini API key](https://aistudio.google.com/apikey) (default)
  - [OpenAI API key](https://platform.openai.com/api-keys)
  - [Ollama](https://ollama.com/) running locally (no API key needed)

## Getting started

```bash
git clone https://github.com/ludusrusso/wildgecu.git
cd wildgecu
```

### With Gemini (default)

```bash
export GEMINI_API_KEY="your-api-key"
go run .
```

### With OpenAI

```bash
export WILDGECU_PROVIDER=openai
export OPENAI_API_KEY="your-api-key"
go run . --model gpt-4o
```

### With Ollama

```bash
export WILDGECU_PROVIDER=ollama
go run . --model llama3
```

Then bootstrap your agent's identity:

```bash
go run . init
```

The `init` command starts an interactive conversation where the agent asks about its name, purpose, personality, and expertise. When done, it writes `.wildgecu/SOUL.md` automatically. After that, you can start chatting:

```bash
go run .
```

## CLI commands

WildGecu is a single binary. Chat is the default command; daemon management and specialized modes are available as subcommands.

```bash
# Bootstrap
wildgecu init         # create SOUL.md through an interactive interview

# Chat (default)
wildgecu              # interactive chat session
wildgecu chat         # same thing, explicit

# Code Mode
wildgecu code         # start a coding agent in the current directory

# Custom home directory
wildgecu --home /path/to/home start   # use a custom home instead of ~/.wildgecu
wildgecu --home /path/to/home chat    # all subcommands respect --home

# Daemon lifecycle
wildgecu start        # start the background daemon
wildgecu stop         # stop the daemon
wildgecu restart      # stop + start
wildgecu status       # show daemon status (pid, uptime, version)
wildgecu health       # exit 0 if daemon is healthy, 1 otherwise
wildgecu logs         # show last 50 log lines
wildgecu logs -f      # follow log output

# Cron jobs
wildgecu cron ls      # list all scheduled jobs
wildgecu cron add     # add a new cron job (interactive TUI)
wildgecu cron rm test # remove a cron job by name

# Skills
wildgecu skill ls     # list installed skills
wildgecu skill add    # add a new skill

# System service
wildgecu install      # install as a system service
wildgecu uninstall    # remove the system service

# Self-update
wildgecu update --url <binary-url>   # trigger a self-update
```

Build with a version tag:

```bash
go build -ldflags "-X wildgecu/cmd.Version=1.0.0" -o wildgecu .
```

## Cron jobs

The daemon executes scheduled LLM prompts. Cron jobs are defined as markdown files with YAML frontmatter in `~/.wildgecu/crons/`. Results are written to `~/.wildgecu/cron-results/`.

### Cron file format

```markdown
---
name: daily-summary
cron: "0 9 * * *"
---

Summarize the key events from yesterday and suggest priorities for today.
```

The frontmatter requires `name` and `cron` (standard 5-field cron expression). Everything after the closing `---` is the LLM prompt.

## Skills

Skills are domain-specific knowledge files that the agent can load on demand. They are stored as markdown files with YAML frontmatter in `~/.wildgecu/skills/`.

### Skill file format

```markdown
---
name: code-review
description: Guidelines for reviewing Go code
tags: [go, review]
---

When reviewing Go code, focus on...
```

The agent loads skills dynamically via the `load_skill` tool during conversation.

## Configuration

WildGecu uses a unified home directory at `~/.wildgecu/` for all global state. Override it with `--home`:

```bash
wildgecu --home /path/to/custom/home start
```

This allows running multiple independent instances, each with its own config, socket, crons, and skills. The flag accepts absolute paths, relative paths, and `~/...` tilde expansion.

### Global files (`~/.wildgecu/`)

| File / Directory | Purpose |
| --- | --- |
| `wildgecu.yaml` | Configuration (provider, API keys, model) — created on first run |
| `wildgecu.pid` | Daemon PID file |
| `wildgecu.sock` | Daemon Unix domain socket |
| `wildgecu.log` | Daemon log file (JSON) |
| `crons/` | Cron job definitions (markdown + YAML frontmatter) |
| `cron-results/` | Output from executed cron jobs |
| `skills/` | Domain-specific knowledge files |

### Project files (`.wildgecu/` in working directory)

| File | Purpose |
| --- | --- |
| `SOUL.md` | Agent identity — created during bootstrap |
| `MEMORY.md` | Persistent context — curated after each session |
| `USER.md` | Optional user preferences — create manually |

Delete `SOUL.md` and run `wildgecu init` again to give your agent a new identity.

### Config file

The config file is searched in order: `./wildgecu.yaml`, then `~/.wildgecu/wildgecu.yaml`. Override with `--config`:

```bash
wildgecu --config /path/to/config.yaml
```

Environment variables also work:

```bash
# Provider selection (default: gemini)
export WILDGECU_PROVIDER="gemini"   # or "openai" or "ollama"

# API keys (set the one matching your provider)
export GEMINI_API_KEY="your-key"
export OPENAI_API_KEY="your-key"

# Ollama settings (no API key needed)
export OLLAMA_BASE_URL="http://localhost:11434/v1"  # default
```

## Architecture

```
wildgecu.go                  # Entry point → cmd.Execute()
│
├── cmd/                     # CLI layer (Cobra)
│   ├── root.go              # Root command, config init, Version var
│   ├── init.go              # init subcommand — bootstrap SOUL.md
│   ├── chat.go              # chat subcommand (also default)
│   ├── start.go             # start subcommand + runDaemon()
│   ├── stop.go / restart.go # daemon lifecycle
│   ├── status.go / health.go
│   ├── logs.go              # logs subcommand
│   ├── update.go            # self-update subcommand
│   ├── install.go           # system service install/uninstall
│   ├── cron.go / cron_add.go
│   └── skill.go / skill_add.go
│
├── pkg/                     # Core domain packages
│   ├── agent/               # Agent logic
│   │   ├── agent.go         # Prepare() / Finalize() — orchestrates bootstrap → chat
│   │   ├── bootstrap.go     # Bootstrap interview + write_soul tool
│   │   ├── soul.go          # Soul I/O and system prompt assembly
│   │   ├── memory.go        # Memory persistence and curation
│   │   ├── prompt.go        # Embeds AGENT.md, BOOTSTRAP.md, MEMORY_AGENT.md
│   │   ├── AGENT.md         # Base agent behavior prompt
│   │   ├── BOOTSTRAP.md     # Bootstrap conversation prompt
│   │   └── MEMORY_AGENT.md  # Memory curation instructions
│   │
│   ├── provider/            # LLM provider abstraction
│   │   ├── provider.go      # Provider interface, types
│   │   ├── factory/          # Provider factory (factory.New)
│   │   ├── agent.go         # RunAgentLoop / RunAgentLoopStream
│   │   ├── tool/            # Type-safe tool system (Tool, Registry, Executor)
│   │   ├── gemini/          # Google Gemini implementation
│   │   └── openai/          # OpenAI / Ollama implementation
│   │
│   ├── session/             # Conversation management
│   │   └── session.go       # RunTurn, RunTurnStream, callbacks
│   │
│   ├── chat/                # Chat frontends
│   │   ├── tui/             # Bubble Tea terminal UI
│   │   └── telegram/        # Telegram bot bridge
│   │
│   ├── cron/                # Cron scheduling
│   │   ├── cron.go          # CronJob struct, Parse, LoadAll
│   │   ├── executor.go      # Execute() — runs a single cron job
│   │   └── scheduler.go     # Scheduler — wraps gocron
│   │
│   ├── skill/               # Skills system
│   │   └── skill.go         # Skill struct, Parse, Load
│   │
│   └── daemon/              # Daemon infrastructure
│       ├── daemon.go        # Main loop, socket server, signals
│       ├── sessions.go      # SessionManager for concurrent chats
│       ├── socket.go        # Unix socket server + command dispatch
│       ├── chat_client.go   # NDJSON streaming client for TUI/Telegram
│       ├── client.go        # Command-based IPC client
│       ├── watchdog.go      # Periodic health checker
│       ├── updater.go       # Self-update via binary replacement
│       ├── pidfile.go
│       └── service.go       # System service integration
│
└── x/                       # General-purpose utilities
    ├── config/              # Shared config (GlobalHome, ProjectDir)
    ├── home/                # File abstraction (Home, FSHome, MemHome)
    ├── context/             # Context utilities
    └── debug/               # Debug logging
```

### Key design decisions

- **Single binary** — All commands (chat, daemon, cron, skills, service) are subcommands of one `wildgecu` binary.
- **`pkg/` and `x/` layout** — Core domain packages live under `pkg/`, general-purpose utilities with no domain knowledge live under `x/`.
- **Unified home (`~/.wildgecu/`)** — Config, PID, socket, logs, crons, and skills all live under one directory, managed by `x/config`. Overridable via `--home` for running multiple isolated instances.
- **`x/config` package** — Zero-dependency (stdlib only) shared package that all other packages import for path resolution.
- **Project-local `.wildgecu/`** — Per-project identity files (`SOUL.md`, `MEMORY.md`, `USER.md`) stay in the working directory, separate from global daemon state.
- **Home abstraction** — File operations are abstracted behind an interface (`FSHome` for disk, `MemHome` for tests), keeping the agent logic testable.
- **Parallel tool calling** — Independent tool calls within a single agent turn are executed concurrently for lower latency.

## Providers

WildGecu ships with three providers:

| Provider | Package | Streaming | Tool Calling | API Key Required |
|----------|---------|-----------|--------------|------------------|
| Google Gemini | `pkg/provider/gemini` | Yes | Yes | Yes |
| OpenAI | `pkg/provider/openai` | Yes | Yes | Yes |
| Ollama | `pkg/provider/openai` (shared) | Yes | Model-dependent | No |

Ollama uses the same OpenAI-compatible implementation with a custom base URL.

### Adding a new provider

1. Implement the `provider.Provider` interface:

```go
type Provider interface {
    Generate(ctx context.Context, params *GenerateParams) (*Response, error)
}
```

2. For streaming support, also implement `StreamProvider`:

```go
type StreamProvider interface {
    Provider
    GenerateStream(ctx context.Context, params *GenerateParams) (<-chan StreamChunk, <-chan error)
}
```

3. Register it in the factory at `pkg/provider/factory/factory.go`.

## License

Apache 2.0 — see [LICENSE](LICENSE) for details.

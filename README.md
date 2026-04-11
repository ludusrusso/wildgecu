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

Set your API key (via shell export or `~/.wildgecu/.env` file):

```bash
export GEMINI_API_KEY="your-api-key"
go run .
```

On first run, WildGecu creates `~/.wildgecu/wildgecu.yaml` pre-configured for Gemini. To use other providers, edit the config file — see the [Configuration](#configuration) section.

### With multiple providers

Edit `~/.wildgecu/wildgecu.yaml` to add providers and model aliases:

```yaml
providers:
  gemini:
    type: gemini
    api_key: env(GEMINI_API_KEY)
  openai:
    type: openai
    api_key: env(OPENAI_API_KEY)
  ollama:
    type: ollama

models:
  fast: gemini/gemini-2.0-flash
  local: ollama/llama3

default_model: gemini/gemini-2.5-flash
```

Then switch models at runtime with `--model`:

```bash
go run . --model openai/gpt-4o
go run . --model local            # uses the "local" alias → ollama/llama3
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
| `.env` | Optional environment variables loaded at startup |
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

### Config file (`wildgecu.yaml`)

On first run, WildGecu creates a default `~/.wildgecu/wildgecu.yaml`. Here is a full example showing all available options:

```yaml
providers:
  gemini:
    type: gemini
    api_key: env(GEMINI_API_KEY)
    google_search: true           # enable Gemini's Google Search grounding

  openai:
    type: openai
    api_key: env(OPENAI_API_KEY)

  ollama:
    type: ollama                  # base_url defaults to http://localhost:11434/v1

  mistral:
    type: mistral                 # base_url defaults to https://api.mistral.ai/v1
    api_key: env(MISTRAL_API_KEY)

  regolo:
    type: regolo                  # base_url defaults to https://api.regolo.ai/v1
    api_key: env(REGOLO_API_KEY)

  custom:
    type: openai
    api_key: env(CUSTOM_API_KEY)
    base_url: "https://my-provider.example.com/v1"  # any OpenAI-compatible endpoint

models:
  fast: gemini/gemini-2.0-flash
  smart: gemini/gemini-2.5-pro
  local: ollama/llama3

default_model: gemini/gemini-2.5-flash  # or use an alias: "fast"

telegram_token: env(TELEGRAM_BOT_TOKEN) # optional, for the Telegram bridge
```

**Key concepts:**

- **`providers`** — a named map of LLM providers. Each entry requires a `type` field (`gemini`, `openai`, `ollama`, `mistral`, `regolo`). The name you give a provider is how you reference it elsewhere (e.g. `gemini/gemini-2.5-flash` means the provider named `gemini`, model `gemini-2.5-flash`).
- **`models`** — optional aliases for `provider/model` pairs. Alias names must not contain `/`.
- **`default_model`** — required. Can be a direct `provider/model` reference or an alias name.
- **`telegram_token`** — optional. Token for the Telegram bot bridge.

### `env()` syntax

Config values can reference environment variables using the `env(VAR_NAME)` syntax:

```yaml
api_key: env(GEMINI_API_KEY)    # resolved from the GEMINI_API_KEY env var
base_url: env(CUSTOM_URL)       # works for base_url too
telegram_token: env(TG_TOKEN)   # and for telegram_token
default_model: env(DEFAULT_MODEL) # and default_model
```

If the referenced variable is not set, WildGecu exits with an error naming the missing variable. This syntax works for `api_key`, `base_url`, `telegram_token`, and `default_model` fields.

### `.env` file

WildGecu automatically loads a `.env` file from the home directory (`~/.wildgecu/.env`) at startup. This is a convenient alternative to exporting variables in your shell profile:

```bash
# ~/.wildgecu/.env
GEMINI_API_KEY=your-gemini-key
OPENAI_API_KEY=your-openai-key
TELEGRAM_BOT_TOKEN=your-bot-token
```

**Precedence:** environment variables already set in your shell take priority over values in the `.env` file. If the file does not exist, it is silently ignored.

### Provider defaults

Some provider types have built-in default base URLs, so you don't need to specify `base_url` for them:

| Type | Default `base_url` |
| --- | --- |
| `ollama` | `http://localhost:11434/v1` |
| `mistral` | `https://api.mistral.ai/v1` |
| `regolo` | `https://api.regolo.ai/v1` |

You can always override these by setting `base_url` explicitly. The `gemini` and `openai` types use their respective SDK defaults and don't need a base URL.

### Config file search order

The config file is loaded from `~/.wildgecu/wildgecu.yaml`. You can also override the model at runtime:

```bash
wildgecu --model gpt-4o          # override with a provider/model reference
wildgecu --model fast             # or use a model alias
```

## Architecture

```
wildgecu.go                  # Entry point → cmd.Execute()
│
├── cmd/                     # CLI layer (Cobra)
│
├── pkg/                     # Core domain packages
│   ├── agent/               # Agent orchestration (Prepare, Finalize, bootstrap, memory, prompts)
│   │   └── tools/           # Tool suites (general, exec, files, skills)
│   ├── provider/            # LLM provider abstraction
│   │   ├── tool/            # Type-safe tool framework (Tool, Registry, schema generation)
│   │   ├── factory/         # Provider factory
│   │   ├── gemini/          # Google Gemini implementation
│   │   └── openai/          # OpenAI / Ollama implementation
│   ├── session/             # Conversation management (RunTurn, RunTurnStream)
│   ├── chat/                # Chat frontends (tui/, telegram/)
│   ├── cron/                # Cron scheduling and execution
│   ├── skill/               # Skills system (parse, load)
│   └── daemon/              # Background daemon (socket, sessions, watchdog, updater, service)
│
└── x/                       # General-purpose utilities (config, home, context, debug)
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

WildGecu ships with two provider implementations that cover multiple services:

| Provider | Type | Package | Streaming | Tool Calling | API Key Required |
|----------|------|---------|-----------|--------------|------------------|
| Google Gemini | `gemini` | `pkg/provider/gemini` | Yes | Yes | Yes |
| OpenAI | `openai` | `pkg/provider/openai` | Yes | Yes | Yes |
| Ollama | `ollama` | `pkg/provider/openai` (shared) | Yes | Model-dependent | No |
| Mistral | `mistral` | `pkg/provider/openai` (shared) | Yes | Yes | Yes |
| Regolo | `regolo` | `pkg/provider/openai` (shared) | Yes | Yes | Yes |

Ollama, Mistral, and Regolo use the OpenAI-compatible implementation with their respective default base URLs. Any OpenAI-compatible endpoint can be used by setting `type: openai` with a custom `base_url`.

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

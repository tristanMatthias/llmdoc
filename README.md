# llmdoc

[![CI](https://github.com/tristanmatthias/llmdoc/actions/workflows/ci.yml/badge.svg)](https://github.com/tristanmatthias/llmdoc/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/tristanmatthias/llmdoc)](https://goreportcard.com/report/github.com/tristanmatthias/llmdoc)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> Keep your codebase legible — for you, your team, and your AI tools.

`llmdoc` scans a codebase, generates concise LLM-powered summaries for every source file, and stores them as structured comment headers or in a separate index file. A SHA-256 hash in each annotation detects file changes, so only modified files ever call the LLM.

```
$ llmdoc annotate .
  created    internal/auth/jwt.go
  created    internal/auth/middleware.go
  unchanged  internal/config/config.go
  updated    cmd/root.go

Summary: 2 created, 1 updated, 1 unchanged, 0 errors  (3.4s)
Tokens:  8,421 in / 312 out  (8,733 total)
```

## Why llmdoc?

When you paste a large codebase into an LLM, you pay for tokens on every file — even ones the model barely needs. `llmdoc` pre-annotates your code so AI tools can reason about the whole project at a glance:

- **`llmdoc annotate`** – add or refresh summaries across your entire codebase
- **`llmdoc dump`** – export all summaries as Markdown, XML, or plain text, ready to drop into an LLM context window
- **`llmdoc check`** – CI gate that fails if any annotation is stale or missing

## Features

- **Incremental** — only files that have actually changed call the LLM
- **Two storage modes** — *inline* (summaries live in source files) or *index* (summaries in a separate YAML file; source files never modified)
- **50+ languages** — Go, TypeScript, Python, Rust, Java, Ruby, SQL, YAML, and more
- **Multiple providers** — Anthropic and OpenAI out of the box; easy to extend
- **CI-ready** — `llmdoc check` exits 1 on stale annotations, with no API key required
- **`.gitignore`-aware** — respects root and nested `.gitignore` files automatically
- **Cost estimation** — `--dry-run` shows a projected cost before you spend a token
- **Self-updating** — `llmdoc update` fetches and installs the latest release in place

## Installation

### macOS / Linux (install script)

```bash
curl -fsSL https://raw.githubusercontent.com/tristanmatthias/llmdoc/main/install.sh | sh
```

To pin a specific version:

```bash
VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/tristanmatthias/llmdoc/main/install.sh | sh
```

### go install

```bash
go install github.com/tristanmatthias/llmdoc@latest
```

### Pre-built binaries

Download the latest binary for your platform from the [Releases page](https://github.com/tristanmatthias/llmdoc/releases), extract the archive, and move the binary onto your `PATH`.

### Build from source

```bash
git clone https://github.com/tristanmatthias/llmdoc.git
cd llmdoc
make install
```

## Quick start

```bash
# 1. Create a config file in your project root
llmdoc init

# 2. Set your API key (or add it to .llmdoc.yaml)
export ANTHROPIC_API_KEY=sk-ant-...

# 3. Annotate your codebase
llmdoc annotate .
```

That's it. Every matching source file now has a structured summary, and subsequent runs only re-annotate files you've actually changed.

## What an annotation looks like

### Go / C / Rust / TypeScript (block-comment syntax)

```go
/*llmdoc:start
summary: Orchestrates concurrent file annotation by walking a directory, computing
  content hashes, and calling an LLM provider to generate summaries. The Run
  function fans out work across a semaphore-bounded goroutine pool and streams
  Result values through a channel as files complete.
hash: b319623030f056cebc58c153d60d2ca4dc787ed664385a0194c83b6a113ca98a
model: claude-sonnet-4-6
generated: 2025-06-01T10:23:44Z
version: 1
llmdoc:end*/
package annotator
```

### Python / Ruby / YAML (line-comment syntax)

```python
#llmdoc:start
# summary: Loads application configuration from environment variables and a YAML
#   file, applies validation, and exposes a typed Config dataclass used throughout
#   the service.
# hash: 4a8f2c1d...
# model: claude-sonnet-4-6
# generated: 2025-06-01T10:23:44Z
# version: 1
#llmdoc:end

from dataclasses import dataclass
```

## Commands

### `annotate`

```
llmdoc annotate [path] [flags]
```

Scans `path` (default: `.`) and adds or updates llmdoc annotations for every matching source file.

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | `false` | Show which files would be annotated and estimate the cost — no files modified, no LLM calls |
| `--verbose`, `-v` | `false` | Print status for unchanged files too |
| `--quiet`, `-q` | `false` | Suppress all output except errors |
| `--force` | `false` | Re-annotate even when the hash is unchanged |
| `--provider` | config | Override LLM provider (`anthropic`, `openai`) |
| `--model` | config | Override model (e.g. `gpt-4o`, `claude-opus-4-6`) |
| `--concurrency` | `4` | Number of parallel LLM calls |

**Dry-run output example:**

```
  created    internal/auth/jwt.go
  created    internal/auth/middleware.go
  updated    cmd/root.go

Cost estimate for 3 file(s):
  model  claude-sonnet-4-6  ($3.00 in / $15.00 out per MTok)
  tokens ~12,450 input / ~450 output
  cost   ~$0.0444
  prices from https://www.anthropic.com/pricing — actual usage may vary

Summary: 2 created, 1 updated, 1 unchanged, 0 errors  (dry run)
```

### `check`

```
llmdoc check [path] [flags]
```

Validates that every annotation hash matches the current file content. Exits 0 if all annotations are current; exits 1 if any are stale or missing. Does **not** call an LLM.

| Flag | Default | Description |
|------|---------|-------------|
| `--quiet`, `-q` | `false` | Only print stale/missing files; suppress ok status |

### `dump`

```
llmdoc dump [path] [flags]
```

Exports all annotations as a single document, suitable for pasting into an LLM context window.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `markdown` | Output format: `markdown`, `xml`, `plain` |
| `--output`, `-o` | stdout | Write to a file instead of stdout |
| `--include-content` | `false` | Include full file content alongside summaries |
| `--no-tree` | `false` | Omit the directory tree from Markdown output |

### `init`

```
llmdoc init [flags]
```

Writes a starter `.llmdoc.yaml` to the current directory.

| Flag | Description |
|------|-------------|
| `--force` | Overwrite an existing config file |

### `update`

```
llmdoc update [flags]
```

Checks GitHub for a newer release and, if found, downloads the binary for the current platform and replaces the running executable in place. Not supported on Windows (download manually from the Releases page).

| Flag | Description |
|------|-------------|
| `--check` | Report whether an update is available without installing it |

### Global flags

All subcommands accept these persistent flags:

| Flag | Description |
|------|-------------|
| `--config` | Path to `.llmdoc.yaml` (default: auto-discover) |
| `--provider` | Override the LLM provider |
| `--model` | Override the model |
| `--concurrency` | Override the concurrency limit |
| `--force` | Override the force flag |

## Configuration

`llmdoc` auto-discovers config files in this order:

1. `--config` flag (if provided)
2. `.llmdoc.yaml` / `.llmdoc.yml` in the current directory
3. `~/.config/llmdoc/config.yaml` / `~/.config/llmdoc/config.yml`

Run `llmdoc init` to generate a starter file with all available options documented inline.

```yaml
# LLM provider: "anthropic" or "openai"
provider: anthropic

# API key — prefer environment variables over storing keys in files
api_key: ""

# Model identifier
model: claude-sonnet-4-6

# Storage mode: "inline" or "index" (see Storage Modes below)
mode: index

# Path to the index file (only used when mode: index)
index_file: .llmdoc/index.yaml

# File extensions to annotate
extensions:
  - .go
  - .ts
  - .tsx
  - .js
  - .py
  - .rs

# Ignore patterns (gitignore-style, relative to project root)
ignore:
  - vendor/
  - node_modules/
  - .git/
  - dist/
  - build/
  - "**/*.min.js"
  - "**/*.generated.go"

# Number of concurrent LLM calls
concurrency: 4

# Force re-annotation even when the file hash is unchanged
force: false
```

### API key resolution

Keys are resolved in priority order:

1. `api_key` in `.llmdoc.yaml`
2. `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` (provider-specific environment variable)
3. `LLMDOC_API_KEY` (provider-agnostic fallback)
4. `.env` file in the current directory (loaded automatically; existing shell variables take precedence)

## Storage modes

### Inline mode

Summaries are stored as structured comment headers at the top of each source file. They travel with the code in version control and are visible during code review.

```
mode: inline
```

**Pros:** annotations are co-located with the code; visible in diffs; no extra files to manage.
**Cons:** every annotated file is modified; may add noise to diffs.

### Index mode (default)

Summaries are stored in `.llmdoc/index.yaml`. Source files are **never modified**.

```
mode: index
index_file: .llmdoc/index.yaml  # default
```

**Pros:** source files stay pristine; suitable for teams that don't want annotation churn in diffs.
**Cons:** index file must be committed (or deliberately excluded) separately.

### Switching modes

Change `mode` in your config and run `llmdoc annotate`. Existing summaries are migrated automatically — the tool reuses them without calling the LLM.

## CI integration

Add `llmdoc check` to your pipeline to enforce that no annotation goes stale:

```yaml
# .github/workflows/ci.yml
- name: Check llmdoc annotations
  run: llmdoc check .
```

`check` requires no API key and exits 1 if any file's hash doesn't match its stored annotation. Combined with a pre-commit or CI step that runs `llmdoc annotate`, this guarantees your summaries stay current.

## Supported languages

| Language | Extensions |
|----------|------------|
| Go | `.go` |
| TypeScript | `.ts`, `.tsx` |
| JavaScript | `.js`, `.jsx`, `.mjs`, `.cjs` |
| Python | `.py` |
| Rust | `.rs` |
| Java | `.java` |
| Kotlin | `.kt`, `.kts` |
| Swift | `.swift` |
| C / C++ | `.c`, `.h`, `.cpp`, `.hpp`, `.cc` |
| C# | `.cs` |
| PHP | `.php` |
| Scala | `.scala` |
| Groovy | `.groovy` |
| Dart | `.dart` |
| Ruby | `.rb` |
| Elixir | `.ex`, `.exs` |
| Crystal | `.cr` |
| Perl | `.pl`, `.pm` |
| R | `.r`, `.R` |
| Haskell | `.hs` |
| Elm | `.elm` |
| Ada | `.ads`, `.adb` |
| Lua | `.lua` |
| SQL | `.sql` |
| Shell | `.sh`, `.bash`, `.zsh`, `.fish` |
| YAML | `.yaml`, `.yml` |
| TOML | `.toml` |
| INI / Config | `.ini`, `.conf` |
| HTML | `.html`, `.htm` |
| XML | `.xml` |
| SVG | `.svg` |
| Vue | `.vue` |
| Svelte | `.svelte` |

To add a language, see [CONTRIBUTING.md](CONTRIBUTING.md#adding-a-language).

## How it works

1. **Scan** — `scanner.Walk` collects all files matching your configured `extensions`, respecting both `ignore` patterns and any `.gitignore` files found in the tree (root and nested).
2. **Hash** — each file is SHA-256 hashed *after* stripping any existing llmdoc block, making the hash invariant to annotation changes.
3. **Diff** — the current hash is compared to the stored hash (in the index or in the file's own header). Unchanged files are skipped.
4. **Summarise** — new or changed files are sent to the LLM with a system prompt requesting a 2–4 sentence summary. If a previous summary exists, it is included so the model can produce an incremental update.
5. **Store** — the summary and hash are written back, either as a comment header at the top of the source file (inline mode) or to the index YAML (index mode).

## Contributing

Contributions are very welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style, and how to add new providers or languages.

## License

MIT — see [LICENSE](LICENSE).

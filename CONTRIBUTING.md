# Contributing to llmdoc

Thank you for your interest in contributing. All contributions are welcome — bug fixes, new features, documentation improvements, and new language or provider support.

## Prerequisites

- [Go](https://go.dev/dl/) 1.22 or later
- An Anthropic or OpenAI API key for manual testing (not required for the test suite)

## Development setup

```bash
git clone https://github.com/tristanmatthias/llmdoc.git
cd llmdoc

# Build
make build

# Run the test suite
make test

# Lint
make lint
```

The test suite does not call any LLM APIs — it uses in-process mock providers.

## Project structure

```
main.go                        Entry point; injects version string at build time
cmd/                           Cobra subcommands (annotate, check, dump, init, update)
internal/
  annotator/annotator.go       Orchestration: goroutine pool, change detection, write-back
  comment/
    block.go                   Block struct, Render(), Parse(), IsValidSentinelLine()
    language.go                Extension → CommentSyntax map (add new languages here)
  config/config.go             Config struct, Load(), StarterYAML()
  dumper/dumper.go             Markdown/XML/plain output for `dump`
  hasher/hasher.go             StripBlock(), ComputeHash()
  index/index.go               Index YAML load/save
  llm/
    provider.go                Provider interface, baseClient, buildPrompt
    anthropic.go               Anthropic implementation
    openai.go                  OpenAI implementation
    factory.go                 NewProvider()
  pricing/pricing.go           Model price table, token estimation, cost projection
  scanner/scanner.go           Walk(), matchesIgnore(), readGitignore()
  updater/updater.go           LatestVersion(), IsNewer(), Update()
```

## Adding a language

Open `internal/comment/language.go` and add an entry to `extensionMap`:

```go
".zig": lineComment("// ", "Zig"),   // line-comment style
".ex":  lineComment("# ", "Elixir"), // already present — example only
".go":  blockComment("Go"),          // block-comment style (/* ... */)
".vue": htmlComment("Vue"),          // HTML-style (<!-- ... -->)
```

Use the constructor that matches the language's comment style:

| Constructor | Example output |
|-------------|----------------|
| `blockComment(lang)` | `/*llmdoc:start ... llmdoc:end*/` |
| `lineComment(prefix, lang)` | `# llmdoc:start ... # llmdoc:end` |
| `htmlComment(lang)` | `<!--llmdoc:start ... llmdoc:end-->` |

Then add the extension to the `Extensions` default slice in `internal/config/config.go` and to the supported languages table in `README.md`.

## Adding an LLM provider

1. Create `internal/llm/myprovider.go` implementing the `Provider` interface:

```go
type MyProvider struct{ baseClient }

func NewMyProvider(apiKey, model string) *MyProvider {
    return &MyProvider{newBaseClient(apiKey, model)}
}

func (p *MyProvider) Summarize(ctx context.Context, req SummaryRequest) (string, TokenUsage, error) {
    // build payload, call p.postJSON, extract text and token counts
}
```

2. Register the new provider in `internal/llm/factory.go`:

```go
case "myprovider":
    return NewMyProvider(cfg.APIKey, cfg.Model), nil
```

3. Add pricing data for the provider's models in `internal/pricing/pricing.go`:

```go
"myprovider-model-name": {InputPerMTok: 1.00, OutputPerMTok: 3.00, PriceURL: "https://myprovider.com/pricing"},
```

4. Update the error message in `factory.go` and the `StarterYAML` comment in `config.go` to mention the new provider.

## Code style

- Run `gofmt` before committing (`make fmt`).
- Keep functions short and focused. When a helper is used in only one place, keep it local.
- Avoid adding dependencies — the project intentionally uses `net/http` directly rather than LLM SDKs.
- All exported symbols must have a doc comment.

## Submitting a pull request

1. Fork the repository and create a feature branch from `main`.
2. Write tests for new behaviour.
3. Ensure `make test lint` passes.
4. Open a PR against `main` with a clear description of what changed and why.

For large changes, open an issue first to discuss the approach.

## Reporting bugs

Please use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md) and include:
- Your OS and Go version (`go version`)
- The llmdoc version (`llmdoc --version`)
- The smallest `.llmdoc.yaml` that reproduces the issue
- Full command output (redact any API keys)

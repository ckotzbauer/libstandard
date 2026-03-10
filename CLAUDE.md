# CLAUDE.md

## Project Overview

`libstandard` is a shared Go library (`github.com/ckotzbauer/libstandard`) that provides reusable utilities for other projects by the same author. It serves as a lightweight replacement for Viper, offering configuration reading from multiple sources (environment variables, YAML/JSON files, and CLI flags), plus helpers for logging setup, data compression, and common string/slice operations.

The config-handling code is largely ported from the [cleanenv](https://github.com/ilyakaznacheev/cleanenv) library (MIT licensed).

- **Author:** Christian Kotzbauer
- **License:** MIT
- **Repository:** `github.com/ckotzbauer/libstandard`

## Tech Stack

- **Language:** Go 1.24 (`go.mod` specifies `go 1.24`)
- **Package:** Single flat package `libstandard` (no sub-packages)
- **Key dependencies:**
  - `github.com/spf13/cobra` v1.10.1 -- CLI command framework
  - `github.com/spf13/pflag` v1.0.10 -- POSIX flag parsing
  - `github.com/sirupsen/logrus` v1.9.4 -- Structured logging
  - `github.com/andybalholm/brotli` v1.2.0 -- Brotli compression
  - `github.com/iancoleman/strcase` v0.3.0 -- String case conversion
  - `gopkg.in/yaml.v3` v3.0.1 -- YAML parsing
- **Test dependencies:**
  - `github.com/stretchr/testify` v1.11.1 -- Test assertions
- **Linting tools (downloaded to `.tmp/`):**
  - `golangci-lint` v2.0.2
  - `gosec` v2.22.3
- **Dependency management:** Renovate Bot (extends `github>ckotzbauer/renovate-config:default` and `github>ckotzbauer/renovate-config:monthly`)

## Project Structure

```
libstandard/
  config.go            # Config reading from env vars, flags, YAML/JSON files
  config_test.go       # Comprehensive config tests (env, flags, files, combinations)
  compression.go       # Brotli compress/decompress helpers
  compression_test.go  # Compression round-trip test
  flags.go             # Constants: Verbosity="verbosity", Config="config"
  initializer.go       # DefaultInitializer(): reads config + sets up logging via cobra command
  logger.go            # SetupLogging(), AddVerbosityFlag() using logrus
  logger_test.go       # Logging setup tests
  util.go              # String/slice utilities: Unescape, Unique, FirstOrEmpty, ToMap
  util_test.go         # Utility function tests
  go.mod / go.sum      # Module definition
  Makefile             # Build, test, lint commands
  .gitignore           # Ignores .tmp/ and cover.out
  renovate.json        # Renovate Bot config
  .github/
    label-commands.json             # Bot label commands for issues/PRs
    workflows/
      test.yml                      # Test + coverage on push to main
      code-checks.yml               # golangci-lint + gosec on all branches and PRs
      stale.yml                     # Daily stale issue/PR cleanup
      label-issues.yml              # Auto-labeling on issues and PRs
      size-label.yml                # PR size labeling
```

All Go source files belong to the single `libstandard` package at the repository root. There are no sub-directories with Go code.

## Architecture & Patterns

### Configuration System (`config.go`)

The configuration system uses struct tags to define how fields are populated. It supports a layered approach with this precedence (later source wins):

1. **Default values** via `env-default` tag
2. **File values** from YAML (`.yaml`/`.yml`) or JSON (`.json`) files
3. **Environment variables** via `env` tag
4. **CLI flags** via `flag` tag (integrated with `pflag.FlagSet`)

Supported struct tags:
- `env:"VAR_NAME"` -- environment variable name(s), comma-separated for multiple
- `env-default:"value"` -- default value
- `env-required:"true"` -- marks field as required
- `env-separator:","` -- custom separator for slices/maps (default is `,`)
- `env-prefix:"PREFIX_"` -- prefix for nested struct env var names
- `flag:"flag-name"` -- pflag flag name
- `yaml:"key"` / `json:"key"` -- file-based config keys

The `Setter` interface allows custom types to implement `SetValue(string) error` for custom parsing.

The `DefaultFileConfig` struct enables automatic config file discovery by specifying a base name, file extensions, and search paths (including `~/` expansion).

Entry points:
- `ReadFromEnv(cfg)` -- env vars only
- `ReadFromFile(cfg, file, defaultCfg)` -- file + env vars
- `ReadFromFlags(cfg, flags)` -- flags + env vars
- `Read(cfg, flags, file, defaultCfg)` -- all sources combined

### Initializer Pattern (`initializer.go`)

`DefaultInitializer(cfg, cmd, name)` provides a one-call setup that:
1. Reads the `--config` flag from the cobra command
2. Calls `Read()` with default file discovery (looks in `.` and `~/.config/<name>` for `<name>.yaml`)
3. Extracts a `Verbosity` field from the config struct (using `strcase.ToCamel`)
4. Calls `SetupLogging()` with the verbosity level

Consuming projects are expected to define a config struct with a `Verbosity` field and register both `--config` and `--verbosity` flags via `AddConfigFlag()` and `AddVerbosityFlag()`.

### Compression (`compression.go`)

Simple Brotli wrappers:
- `Compress(data []byte)` -- compresses at quality level 11 (maximum)
- `Decompress(data []byte)` -- decompresses Brotli data

### Utilities (`util.go`)

- `Unescape(s)` -- strips backslashes and double quotes
- `Unique(slice)` -- deduplicates a string slice preserving order
- `FirstOrEmpty(slice)` -- returns first element or empty string
- `ToMap(slice)` -- converts `["key=value", ...]` to `map[string]string`

## Build & Development

### Prerequisites

- Go 1.24+
- Make

### Makefile Targets

| Command | Description |
|---|---|
| `make` / `make all` | Runs `build` (which runs `fmt`, `vet`, then `go build ./...`) |
| `make build` | Format, vet, and compile |
| `make fmt` | `go fmt ./...` |
| `make vet` | `go vet ./...` |
| `make test` | `go test ./... -coverprofile cover.out` |
| `make lint` | Run `golangci-lint` (must bootstrap first) |
| `make lintsec` | Run `gosec ./...` (must bootstrap first) |
| `make bootstrap-tools` | Download `golangci-lint` v2.0.2 and `gosec` v2.22.3 into `.tmp/` |

### First-Time Setup

```sh
make bootstrap-tools   # downloads linting tools to .tmp/
make                   # build
make test              # run tests
```

## Testing

- **Framework:** Standard `testing` package with `github.com/stretchr/testify/assert` for assertions
- **Test style:** Table-driven tests (see `config_test.go` for extensive examples)
- **Test files:** `config_test.go`, `compression_test.go`, `logger_test.go`, `util_test.go`
- **Run tests:** `make test` (generates `cover.out` coverage profile)
- **Coverage:** Reported to external service in CI via `cover.out`

Tests use `os.Setenv`/`os.Clearenv` for environment variable tests and temporary files for config file parsing tests.

## Linting & Code Style

- **golangci-lint** v2.0.2 -- invoked via `make lint` (binary in `.tmp/golangci-lint`); run with `--timeout 5m`
- **gosec** v2.22.3 -- security scanner invoked via `make lintsec` (binary in `.tmp/gosec`)
- **go fmt** -- standard Go formatting, run as part of `make build`
- **go vet** -- standard Go static analysis, run as part of `make build`
- `// nolint` comments are used sparingly (e.g., for deferred `Close()` calls and `os.Setenv` in tests)
- `/* #nosec */` comments suppress gosec warnings for intentional file operations

### Code Conventions

- All source is in a single flat package `libstandard`
- Exported functions use PascalCase; no method receivers on the utility functions
- godoc-style comments on all exported functions (see `config.go` for multi-line examples)
- Error handling follows Go convention: return `error` as last return value, no panics in normal flow
- The `isZero` function is a manual backport of `reflect.Value.IsZero()`

## CI/CD

All CI workflows use reusable workflows from `ckotzbauer/actions-toolkit@0.51.0`.

### Workflows

| Workflow | File | Trigger | Purpose |
|---|---|---|---|
| `test` | `.github/workflows/test.yml` | Push to `main` | Runs `make test`, reports coverage from `cover.out` |
| `code-checks` | `.github/workflows/code-checks.yml` | All pushes + PRs | Two parallel jobs: `gosec` and `golint` (both bootstrap tools first) |
| `stale` | `.github/workflows/stale.yml` | Daily cron (`0 0 * * *`) | Marks/closes stale issues and PRs |
| `label-issues` | `.github/workflows/label-issues.yml` | Issue/PR open, comment create/edit | Auto-labels via bot commands |
| `size-label` | `.github/workflows/size-label.yml` | PR open/reopen/sync | Labels PRs by diff size |

### Label Bot Commands (`.github/label-commands.json`)

Only allowed for user `ckotzbauer`:
- `/hold` / `/hold cancel` -- add/remove `hold` label
- `/label <name>` / `/remove-label <name>` -- arbitrary label management
- `/kind <bug|feature|documentation|test|cleanup|security>` -- add `kind/<type>` label
- `/lifecycle <stale|frozen>` / `/remove-lifecycle <stale|frozen>` -- lifecycle labels

## Key Commands

```sh
# Build
make                     # format + vet + compile

# Test
make test                # run all tests with coverage

# Lint
make bootstrap-tools     # one-time: download golangci-lint and gosec
make lint                # run golangci-lint
make lintsec             # run gosec security scanner

# Format
make fmt                 # go fmt ./...
```

## Important Conventions

- This is a **library**, not a standalone application. It is consumed as a Go module dependency by other projects (e.g., `sbom-git-operator`, `vulnerability-operator`).
- All Go code lives in the **root directory** under a single `libstandard` package. There are no sub-packages.
- Config struct tags (`env`, `flag`, `yaml`, `json`, `env-default`, `env-required`, `env-separator`, `env-prefix`) are the primary API contract for the configuration system.
- The `DefaultInitializer` function expects consuming projects to have both `--config` (`-c`) and `--verbosity` (`-v`) flags registered on their cobra root command, and a `Verbosity` field in their config struct.
- Linting tools are **not committed** to the repo; they are downloaded on demand via `make bootstrap-tools` into the `.tmp/` directory (which is gitignored).
- The `cover.out` file is gitignored but was accidentally committed at some point; it should not be tracked.

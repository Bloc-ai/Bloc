# Bloc CLI

The official command-line interface for [bloc-hub.com](https://bloc-hub.com).

Run community-crafted AI model recipes locally — download, configure, and launch `llama-server` in a single command.

## Quick Start

```bash
# Install (macOS)
brew install bloc-org/bloc/bloc

# Or via curl
curl -fsSL https://bloc-hub.com/install.sh | bash
```

```bash
# Deploy a recipe
bloc deploy arnav080/qwen3-30b-moe-8gb-cpu-offload

# Dry run (preview the llama-server command)
bloc deploy arnav080/qwen3-30b-moe-8gb-cpu-offload --dry-run

# Search the registry
bloc search qwen3 --vram 8GB --platform cuda

# See cached models
bloc models

# Manage telemetry
bloc telemetry off
```

## How `bloc deploy` Works

```
1. Fetch recipe YAML from bloc-hub.com/api
2. Probe your hardware (GPU/VRAM/RAM)
3. Check llama-server capabilities via feature probing
4. Download model weights (resumable, SHA256-verified)
5. Execute pre-run setup commands (with your confirmation)
6. Build and launch llama-server
7. Stream logs to terminal + open http://127.0.0.1:8080
8. On shutdown: optionally share anonymous benchmark
```

## Building from Source

```bash
# Requires Go 1.21+
cd cli/
go mod tidy
go build -o bloc .

# With version injection
go build -ldflags "-X github.com/bloc-org/bloc/cmd.Version=0.1.0" -o bloc .
```

## Project Structure

```
cli/
├── main.go
├── cmd/
│   ├── root.go         # Cobra root
│   ├── deploy.go       # Core command (all 8 steps)
│   ├── search.go
│   ├── models.go
│   ├── login.go
│   ├── telemetry.go
│   ├── update.go
│   └── version.go
├── internal/
│   ├── config/         # Auth + telemetry settings (~/.config/bloc)
│   ├── recipe/         # YAML parsing + flag builder
│   ├── hardware/       # GPU/RAM detection (macOS + Linux)
│   ├── probe/          # llama-server capability check
│   ├── downloader/     # Resumable downloads + SHA256 cache
│   ├── runner/         # llama-server subprocess wrapper
│   └── telemetry/      # Opt-in telemetry pipeline
└── .goreleaser.yaml    # Release automation (Homebrew + apt + GitHub Releases)
```

## Telemetry

Off by default. First `bloc deploy` will prompt once. You can also:

```bash
bloc telemetry off   # Disable permanently
bloc telemetry on    # Enable
BLOC_NO_TELEMETRY=1  # Environment variable override
```

Data collected: CLI version, OS, recipe ID, tokens/sec, peak VRAM, success/failure.  
Never collected: file paths, model content, hostnames, IP addresses.

# Contributing to Bloc

Thank you for your interest in contributing to Bloc! 

Bloc's mission is to be the single, unified orchestrator for local AI deployment. We decouple metadata indexing from execution logic, allowing developers to share recipes that work across different hardware constraints and backend engines.

This guide will help you understand our monorepo architecture, set up your local development environment, and contribute code or recipes.

---

## 📁 Monorepo Architecture Overview

Bloc is a single monorepo containing everything needed for the ecosystem:

- **`cli/`**: The Go-based terminal client and core orchestration logic. It parses recipes, provisions environments, and launches engines.
- **`hub/`**: The Next.js web application for `bloc-hub.com`, handling the registry, telemetry, user authentication, and documentation.
- **`recipes/`**: Curated YAML blueprints containing hardware constraints and engine configurations.
- **`scripts/`**: Python CI/CD validation scripts used to automatically validate and sync recipes.

---

## 🛠️ Local Development Setup

### CLI Development
The CLI is written in Go.
1. **Requirements**: Install Go 1.22+.
2. **Setup**:
   ```bash
   cd cli/
   go mod download
   ```
3. **Build & Run**:
   ```bash
   go build -o bloc main.go
   ./bloc --help
   ```

### Hub Development
The Hub is a Next.js application.
1. **Requirements**: Install Node.js 20+ and npm.
2. **Setup**:
   ```bash
   cd hub/
   npm install
   ```
3. **Run Dev Server**:
   ```bash
   # You will need a local or cloud Supabase instance configured in .env.local
   npm run dev
   ```

---

## ⚙️ CLI Architecture: The Engine Plugin System

The CLI uses a capability-driven Plugin Architecture to support multiple backend inference engines. If you want to add support for a new engine (like TensorRT-LLM or Ollama), you do not need to modify the core orchestration code.

### The Pipeline Flow
Execution follows a strict pipeline:
`Validate -> Download -> Resolve -> Probe -> MapFlags -> Launch -> Await -> Present`

### How to Add a New Engine
1. **Create the Package**: Create a new folder under `cli/internal/engine/<name>/`.
2. **Implement the `Engine` Interface**:
   ```go
   type Engine interface {
       Name() string
       Capabilities(ctx context.Context) (*CapabilitySet, error)
       BuildArgs(caps *CapabilitySet, cfg recipe.EngineConfig) ([]string, error)
       Supervisor(cfg LaunchConfig) (*process.Supervisor, error)
   }
   ```
3. **Capability Probing**: Do NOT hardcode engine flag strings (e.g., `--flash-attn`). Instead, your `Capabilities()` method should run `--help` on the engine binary or query the Docker container to build a map of supported features. `BuildArgs` uses this map to construct the correct syntax.
4. **Use the Shared Supervisor**: Use `process.Supervisor` to handle execution, log streaming, signal handling, and `/health` readiness polling.
5. **Register the Engine**: Register your engine in an `init()` block using the Registry pattern.

---

## 📄 Contributing to the Recipe Registry

Bloc relies on a GitOps workflow to update the public recipe registry.

1. **Fork the Repository**: Create a fork of `Bloc-ai/Bloc`.
2. **Create a Namespace**: Under the `recipes/` directory, create a folder named after your GitHub username (e.g., `recipes/myusername/`).
3. **Create the Recipe**: Copy `recipes/TEMPLATE.yaml` into your namespace and configure it for your target model and hardware.
4. **Submit a PR**: Open a Pull Request against the `main` branch. 
5. **CI/CD Validation**: Our automated Python scripts (`scripts/validate_recipe.py`) will check your YAML syntax, hardware requirements, and structure. Once merged, it automatically syncs to the Supabase backend.

---

## ✅ Pull Request Process & Code Standards

1. **Formatting**:
   - **CLI**: Ensure all Go code is formatted using `go fmt`.
   - **Hub**: Ensure Next.js code follows our ESLint and Prettier rules.
2. **Descriptive PRs**: Provide clear PR titles and summarize the changes. If your PR fixes an issue, link to it (e.g., `Fixes #123`).
3. **Testing**: If you add new CLI features, please include relevant tests in your package.

Thank you for helping make local AI deployment seamless for everyone!

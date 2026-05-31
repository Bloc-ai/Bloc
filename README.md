# Bloc: The Unified Local AI Orchestration Monorepo

Bloc is a lightweight, local-first orchestrator for deploying, caching, and serving large language models (LLMs) on consumer hardware. 

By separating the indexing and registry layer from the local runner engine (the **Sealed Envelope Pattern**), Bloc allows developers to configure, share, and run optimized model deployments with maximum stability and zero-configuration setups.

---

## 🚀 Key Features

* **Multi-Engine Support**: Seamlessly run lightweight, single-file GGUF models via `llama.cpp`, or high-throughput Safetensors repositories via `vLLM`.
* **Multi-Runtime Isolation**: Deploy models natively on your host OS as a local process, or run them inside isolated containers using the automated `docker` runtime.
* **Auto-Probing Hardware**: Probes GPU availability, system memory, and platform constraints to prevent out-of-memory (OOM) crashes before weights are downloaded.
* **Secure Cache & Downloads**: Automatically pulls gated Hugging Face models using secure credential workflows, validates files with SHA-256 hashes, and caches them under `~/.cache/bloc`.
* **Containerized GPU Passthrough**: Automatically configures CUDA GPU access for containerized runs via the NVIDIA Container Toolkit.
* **GitOps Recipe Sync**: Curated model configs are sync'd directly from GitHub PRs to the registry database, validating schemas before merging.

---

## 📁 Monorepo Structure

```
bloc/
├── cli/                 # Go-based CLI terminal client
│   ├── cmd/             # Cobra commands (deploy, cache, images, runtime, models)
│   ├── internal/        # Core CLI logic (hardware probe, downloader, runtimes)
│   └── main.go
│
├── hub/                 # Next.js web application (bloc-hub.com)
│   ├── app/             # Web pages, API routes, and OAuth flows
│   ├── content/docs/    # Fumadocs Markdown documentation
│   ├── lib/             # Database clients, utility helper libraries
│   └── supabase/        # Database schema migrations & edge functions
│
├── recipes/             # Curated community blueprints/manifests
│   ├── TEMPLATE.yaml    # Standard starter blueprint
│   └── arnav080/        # Author namespace folder containing recipes
│
└── scripts/             # GitOps CI/CD automation & validation tools
```

---

## 🛠️ Getting Started with the CLI

### 1. Installation

* **macOS (Homebrew):**
  ```bash
  brew tap bloc-ai/bloc
  brew install bloc
  ```
* **Linux (Debian/Ubuntu):**
  ```bash
  wget https://github.com/Bloc-ai/Bloc/releases/download/v0.2.1/bloc_0.2.1_linux_amd64.deb
  sudo dpkg -i bloc_0.2.1_linux_amd64.deb
  ```

### 2. Basic Commands

```bash
# Log in to link your terminal with your Bloc Hub profile
bloc login

# Search for optimized Qwen models matching your hardware
bloc search qwen3 --vram 8GB --platform cuda

# Deploy a recipe natively as a local process
bloc deploy arnav080/qwen3-30b-moe-8gb-cpu-offload

# Deploy a recipe inside an isolated Docker container
bloc deploy arnav080/llama-3-8b-instruct --runtime docker

# View local download cache and virtual environments
bloc cache status
bloc runtime status
```

---

## 💻 Web Hub Development (`/hub`)

The web hub serves the public registry, documentation, telemetry collector, and user OAuth device authorization page. It is built with Next.js and Supabase.

### 1. Prerequisites
* Node.js v20+ and `npm`
* A running Supabase instance (local or cloud)

### 2. Development Setup
```bash
cd hub/
npm install

# Copy environment template and fill in variables
cp .env.local.example .env.local

# Run the dev server
npm run dev
```
Open [http://localhost:3000](http://localhost:3000) to view the application.

---

## 🤝 Contributing & GitOps Recipe Flow

To publish a custom recipe optimization blueprint to the live registry:

1. **Fork** this repository.
2. Create a folder under `recipes/` named after your GitHub username (e.g. `recipes/my-username/`).
3. Copy `recipes/TEMPLATE.yaml` into your folder, rename it, and configure the fields.
4. Open a **Pull Request** to merge your file into `main`.
5. The CI pipeline will automatically parse the YAML, check it against the schema validation rules, and run connectivity tests.
6. Once merged, a GitHub action will sync the recipe to the Supabase database, and it will immediately go live on the Hub.

---

## 📄 License

This monorepo is licensed under the [MIT License](file:///Users/arnavgautam/Documents/bloc/bloc-product/LICENSE).

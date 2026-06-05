# v0.6.5 (June 2026)
- **Robust Downloader Integrity**
- Replaced strict model size validation checks against recipe metadata with dynamic server-side validation and file magic-bytes checks, preventing false-positive download failures on rounded or approximate size values.
- **Documentation & Templates**
- Exposed optional `sha256` fields and updated size validation notes in `TEMPLATE.yaml` and `recipes.mdx` to make recipe creation easier.

# v0.6.4 (June 2026)
- **Reliable Downloads & Cache Protection**: Added automatic retries for download failures and ensured files don't conflict or get corrupted, automatically cleaning up bad cache files.
- **Improved App Stability & Crash Detection**: The app now shuts down cleanly, prevents leftover background tasks, and immediately detects engine crashes to keep you informed.
- **Smoother Command Line Experience**: Increased the update file size limit to 500MB, fixed progress bar crashes, and added helpful error messages if you mistype a configuration path.
- **Conflict Prevention**: Added checks to prevent starting on a port already in use and delayed exit logs so you can read what went wrong before the interface closes.

# v0.6.3 (June 2026)
- **Port Configuration Fix**: Fixed a bug where setting the custom port flag (`--port`) caused the backend server to immediately exit. You can now reliably configure custom ports.

# v0.6.2 (June 2026)
- **Host Configuration Fix**: Fixed a startup crash caused by configuring the backend server host address (`--host`), ensuring port and host flags are correctly interpreted.

# v0.6.1 (June 2026)
- **Improved Testing Stability**: Stabilized internal tests on Windows and macOS/Linux by using native components instead of brittle system scripts.
- **Faster Windows Shutdown**: Fixed a persistent 5-second shutdown delay on Windows when terminating processes.

# v0.6.0 (June 2026)
- Core Engine Rebuild & Runtime System Redesign
  - Unified all inference backends under a common `Engine` interface (`llama.cpp`, `vLLM`, and `SGLang`), allowing seamless plug-and-play capability.
  - Standardized core CLI execution via a stage-based execution pipeline (`Validate` ➔ `Download` ➔ `Resolve` ➔ `Probe` ➔ `MapFlags` ➔ `Launch` ➔ `Await` ➔ `Present`).
  - Implemented the `process.Supervisor` shared execution layer to unify container/subprocess lifecycles, signal handling, watchdog-monitored startups, active health polling (`/health`), and concurrent persistent logging under `~/.cache/bloc/logs/`.
  - Added capability-first flag mapping, dynamically resolving flag naming and syntax at runtime (e.g., `-fa` vs `--flash-attn on`) via help-probing instead of hardcoded string arrays.
- Security Hardening
  - Removed dangerous shell execution patterns (`sh -c`) in pre-run script stages, replacing them with a secure allowlist-filtered system execution.
  - Mitigated path traversal risks by introducing strict containment checking on all user-supplied paths via `filepath.Abs` and prefix validation.
  - Enhanced API token privacy by preventing Hugging Face tokens from escaping into error messages or system logs.
  - Implemented environment filtering to scrub sensitive environment variables (such as `LD_PRELOAD`) from child subprocesses.
- Performance Tuning & Correctness
  - Cached CPU and VRAM capabilities probing via `sync.Once` to prevent expensive redundant python execution loops.
  - Gated CUDA-based Docker testing behind a host-level `nvidia-smi` presence check, preventing automatic ~100MB downloads on CPU-only machines.
  - Resolved nil-map panic vectors in local download operations and fixed deprecated flag compatibility issues with vLLM 0.10.0+.

# v0.5.4 (June 2026)
- **Modern Model Reasoning & Performance Support**
- Updated compatibility flags for modern local model engines (like `llama-server` and `vLLM`) to prevent startup failures, and enabled native reasoning/chain-of-thought extraction for reasoning models like `DeepSeek-R1`.

# v0.5.3 (June 2026)
- **Flash Attention Support**
- Fixed a crash when running models with Flash Attention enabled by updating the settings flags to match the latest changes in the underlying model engine (`llama.cpp`).

# v0.5.2 (June 2026)
- **Instant Screen Sync**
- Fixed a visual delay in the interface where changes in the Chat or History tab did not update across both screens instantly.

# v0.5.1 (June 2026)
- **Contextual UI Shortcuts**
- Replaced static tips in the Studio with helpful, dynamic keyboard shortcuts and linked the "Recent Chats" list to your actual chat history.

# v0.5.0 (June 2026)
- Introduced the Bloc Studio
- A major update bringing a full-fledged Terminal User Interface (TUI) to Bloc for managing models, chatting, and viewing history.

# v0.4.1 (June 2026)
- **Seamless Self-Updates**
- Improved update checks on macOS for Homebrew users, and added a smooth fallback update mechanism if the primary update fails.

# v0.4.0 (June 2026)
- SGLang Engine Support
- Added native integration for the highly optimized SGLang engine, enabling massive throughput improvements for production deployments.

# v0.3.3 (June 2026)
- **Simplified CLI Commands**
- Renamed the deployment command from `bloc deploy` to a simpler and more intuitive `bloc run`, while keeping the old command working for backward compatibility.

# v0.3.2 (June 2026)
- **Enhanced Security Safeguards**
- Expanded the list of blocked or unsafe settings flags (now covering 36 flags) to protect your system from dangerous model behavior.

# v0.3.1 (May 2026)
- **Resolved Freeze Bugs**
- Fixed a critical freeze (deadlock) that occurred when deleting a model from your local cache.

# v0.3.0 (May 2026)
- Update Pointer Migration
- Successfully migrated the repository pointer in the update module to the new Bloc-ai/Bloc organization.

# v0.2.1 (May 2026)
- **Hardened Security Certificates**
- Updated secure connection certificates to maintain safe, encrypted communications.

# v0.2.0 (May 2026)
- Windows Cross-Compilation Support
- Resolved cross-compilation toolchain errors, stabilizing the Windows build pipeline.

# v0.1.0 (May 2026)
- Initial Release
- The first stable release of Bloc, featuring the core CLI, dynamic recipe system, and native llama.cpp execution capabilities.

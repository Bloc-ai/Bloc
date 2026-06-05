# v0.6.3 (June 2026)
- Fixed --port flag being misclassified as a boolean implicit (no value) flag by inferValueType, causing llama-server to exit immediately with "expected value for argument". Added PORT to the string-value placeholder token list alongside the HOST fix from v0.6.2.

# v0.6.2 (June 2026)
- Fixed a bug where --host was misclassified as a boolean implicit flag, causing the next flag (--port) to be consumed as the hostname and llama-server to fail on startup. Added HOST, ADDR, and IP to string-value type inference to resolve this.

# v0.6.1 (June 2026)
- Stabilized Windows and Unix CI tests by replacing flaky shell wrappers (`pause`, `timeout`, `sh -c`) with a native Go `blocker` binary.
- Bypassed `syscall.SIGTERM` signal usage on Windows to fix a persistent 5-second process kill timeout delay.

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
- Speculative Decoding & vLLM Reasoning Flag Updates
- Updated speculative decoding flags to use the modern `llama-server` flags (`--spec-draft-model`, `--spec-draft-n-max`, `--spec-draft-p-min`) instead of deprecated/legacy aliases, preventing capability probe and startup failures on modern builds.
- Updated vLLM flag builder to automatically append the `--enable-reasoning` boolean flag when a reasoning parser is specified (such as `deepseek_r1`), enabling chain-of-thought extraction natively.
- Updated corresponding unit test suites to verify modern flags are built and probed correctly.

# v0.5.3 (June 2026)
- Flash Attention Flag Fix
- Fixed a crash affecting all recipes with `flash_attn: true`. A breaking change in `llama.cpp` renamed `--flash-attn` from a boolean toggle (`-fa`) to a value-required flag (`--flash-attn [on|off|auto]`). The CLI was emitting bare `-fa`, causing `llama-server` to consume the next flag (`-b`) as the value and abort on startup. `BuildFlags()` now correctly emits `--flash-attn on`, and `RequiredFlags()` probes for the long-form flag. Tests updated to reject bare `-fa` emission.

# v0.5.2 (June 2026)
- Sync Tab State
- Fixed an aggressive caching bug in the TUI. The Chat and History tabs now seamlessly synchronize session states dynamically.

# v0.5.1 (June 2026)
- Dynamic TUI Shortcuts & Recent History
- Replaced hardcoded CLI tips in the Studio with context-aware shortcuts and dynamically linked the Recent Chats list to actual session history.

# v0.5.0 (June 2026)
- Introduced the Bloc Studio
- A major update bringing a full-fledged Terminal User Interface (TUI) to Bloc for managing models, chatting, and viewing history.

# v0.4.1 (June 2026)
- CLI Self-Update Fallback
- Implemented robust Homebrew upgrade detection and a seamless self-update fallback mechanism for the update command.

# v0.4.0 (June 2026)
- SGLang Engine Support
- Added native integration for the highly optimized SGLang engine, enabling massive throughput improvements for production deployments.

# v0.3.3 (June 2026)
- Transitioned Deploy to Run
- Refactored 'bloc deploy' to 'bloc run' for better ergonomics, while maintaining complete backward compatibility.

# v0.3.2 (June 2026)
- Expanded Security Blocklist
- Migrated security rules to an embedded JSON deny-list and expanded the blocklist to proactively intercept 36 potentially dangerous engine flags.

# v0.3.1 (May 2026)
- Deadlock Fixes
- Resolved a critical sync.RWMutex deadlock encountered during the DeleteCachedModel operation.

# v0.3.0 (May 2026)
- Update Pointer Migration
- Successfully migrated the repository pointer in the update module to the new Bloc-ai/Bloc organization.

# v0.2.1 (May 2026)
- Cert Pinning Updates
- Hardened security by updating Vercel GTS cert pinning hashes to the new WR1 intermediate certificates.

# v0.2.0 (May 2026)
- Windows Cross-Compilation Support
- Resolved cross-compilation toolchain errors, stabilizing the Windows build pipeline.

# v0.1.0 (May 2026)
- Initial Release
- The first stable release of Bloc, featuring the core CLI, dynamic recipe system, and native llama.cpp execution capabilities.

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

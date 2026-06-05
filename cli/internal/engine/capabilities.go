package engine

// BuildCapabilities parses a raw flag map (produced by parsing engine --help output)
// into a fully populated CapabilitySet with semantic feature detection.
//
// This is the single source of truth for "what does a raw flag mean".
// When an engine renames a flag (e.g. llama.cpp renamed -fa → --flash-attn),
// only this function needs updating — no BuildArgs() callers change.
//
// Feature naming convention: snake_case, permanent stable identifiers.
// A feature name NEVER changes, even if the underlying flag does.
//
// Semantic features recognised:
//
//	flash_attn          –fa (legacy bool_implicit) or --flash-attn (new bool_on_off)
//	speculative_decoding –--spec-type present (was: --speculative-type in very old builds)
//	spec_type           –the specific flag for speculative decoding type
//	kv_cache_types      –both cache-type-k and cache-type-v flags present
//	                      short: -ctk/-ctv  long: --cache-type-k/--cache-type-v
//	jinja               –--jinja present
//	moe_cpu_offload     –--n-cpu-moe present
//	mmap                –--no-mmap present
//	mlock               –--mlock present
//	split_mode          –--split-mode present (multi-GPU)
//	tensor_split        –--tensor-split present (multi-GPU)
func BuildCapabilities(engineName, version string, rawFlags map[string]FlagSpec) *CapabilitySet {
	caps := &CapabilitySet{
		EngineName:   engineName,
		Version:      version,
		flags:        rawFlags,
		features:     make(map[string]bool),
		featureFlags: make(map[string]FlagSpec),
	}

	// ── Flash attention ────────────────────────────────────────────────────────
	// Renamed from -fa (bool_implicit, older builds) to --flash-attn (bool_on_off,
	// current builds as of ~b3700). We detect both and map to the same feature name
	// so BuildArgs() calls HasFeature("flash_attn") regardless of binary age.
	if spec, ok := rawFlags["--flash-attn"]; ok {
		caps.features["flash_attn"] = true
		caps.featureFlags["flash_attn"] = spec
	} else if spec, ok := rawFlags["-fa"]; ok {
		caps.features["flash_attn"] = true
		caps.featureFlags["flash_attn"] = spec
	}

	// ── Speculative decoding ───────────────────────────────────────────────────
	// The flag was named --speculative-type in early builds, renamed to --spec-type.
	// We detect either. The feature "spec_type" provides access to the concrete spec.
	if spec, ok := firstOf(rawFlags, "--spec-type", "--speculative-type"); ok {
		caps.features["speculative_decoding"] = true
		caps.featureFlags["spec_type"] = spec
	}

	// Spec draft model path.
	// Canonical current flag: --model-draft / -md (stable since speculative decoding launched ~Nov 2024).
	// Note: --spec-draft-model does NOT exist in real llama.cpp builds.
	if s, ok := firstOf(rawFlags,
		"--model-draft",
		"-md",
		"--draft-model",
	); ok {
		caps.features["spec_draft_model"] = true
		caps.featureFlags["spec_draft_model"] = s
	}

	// Spec draft N-max — covers --spec-draft-n-max (current) and older names.
	if s, ok := firstOf(rawFlags,
		"--spec-draft-n-max",
		"--draft-max",
		"-cd",
		"--draft-n-max",
	); ok {
		caps.features["spec_draft_n_max"] = true
		caps.featureFlags["spec_draft_n_max"] = s
	}

	// Spec draft p-min (minimum acceptance probability for speculative decoding).
	if s, ok := firstOf(rawFlags,
		"--spec-draft-p-min",
		"--draft-p-min",
	); ok {
		caps.features["spec_draft_p_min"] = true
		caps.featureFlags["spec_draft_p_min"] = s
	}

	// ── KV cache quantization ─────────────────────────────────────────────────
	// Short aliases: -ctk (K) / -ctv (V).
	// Long forms:    --cache-type-k / --cache-type-v (note: no "kv-" prefix in real llama.cpp).
	// Many builds expose both short and long forms on the same line.
	// We detect whichever form the binary reports and map both to stable feature names.
	ktSpec, ktOk := firstOf(rawFlags, "--cache-type-k", "-ctk")
	vtSpec, vtOk := firstOf(rawFlags, "--cache-type-v", "-ctv")
	if ktOk && vtOk {
		caps.features["kv_cache_types"] = true
		caps.featureFlags["kv_cache_type_k"] = ktSpec
		caps.featureFlags["kv_cache_type_v"] = vtSpec
	}

	// ── Jinja templates ────────────────────────────────────────────────────────
	if spec, ok := rawFlags["--jinja"]; ok {
		caps.features["jinja"] = true
		caps.featureFlags["jinja"] = spec
	}

	// ── MoE CPU offload ────────────────────────────────────────────────────────
	// --n-cpu-moe: how many MoE expert layers to offload to CPU.
	if spec, ok := rawFlags["--n-cpu-moe"]; ok {
		caps.features["moe_cpu_offload"] = true
		caps.featureFlags["moe_cpu_offload"] = spec
	}

	// ── Memory mapping / locking ───────────────────────────────────────────────
	if spec, ok := rawFlags["--no-mmap"]; ok {
		caps.features["mmap"] = true
		caps.featureFlags["mmap"] = spec
	}
	if spec, ok := rawFlags["--mlock"]; ok {
		caps.features["mlock"] = true
		caps.featureFlags["mlock"] = spec
	}

	// ── Multi-GPU ─────────────────────────────────────────────────────────────
	if spec, ok := rawFlags["--split-mode"]; ok {
		caps.features["split_mode"] = true
		caps.featureFlags["split_mode"] = spec
	}
	if spec, ok := rawFlags["--tensor-split"]; ok {
		caps.features["tensor_split"] = true
		caps.featureFlags["tensor_split"] = spec
	}

	// ── GPU layers ────────────────────────────────────────────────────────────
	// Always present in any meaningful llama.cpp build; -ngl is the short form.
	if spec, ok := firstOf(rawFlags, "--gpu-layers", "-ngl"); ok {
		caps.features["gpu_layers"] = true
		caps.featureFlags["gpu_layers"] = spec
	}

	// MED-6: Register main_gpu feature so BuildArgs can emit --main-gpu.
	// -mg is the short form; --main-gpu is the long form.
	if spec, ok := firstOf(rawFlags, "--main-gpu", "-mg"); ok {
		caps.features["main_gpu"] = true
		caps.featureFlags["main_gpu"] = spec
	}

	// ── Context window size ───────────────────────────────────────────────────
	if spec, ok := firstOf(rawFlags, "--ctx-size", "-c"); ok {
		caps.features["ctx_size"] = true
		caps.featureFlags["ctx_size"] = spec
	}

	// ── Parallel slots (concurrent request capacity) ──────────────────────────
	if spec, ok := firstOf(rawFlags, "--parallel", "-np"); ok {
		caps.features["parallel"] = true
		caps.featureFlags["parallel"] = spec
	}

	// ── Batch size ────────────────────────────────────────────────────────────
	if spec, ok := firstOf(rawFlags, "--batch-size", "-b"); ok {
		caps.features["batch_size"] = true
		caps.featureFlags["batch_size"] = spec
	}

	// ── Port binding ──────────────────────────────────────────────────────────
	if spec, ok := rawFlags["--port"]; ok {
		caps.features["port"] = true
		caps.featureFlags["port"] = spec
	}

	// ── Host binding ──────────────────────────────────────────────────────────
	if spec, ok := rawFlags["--host"]; ok {
		caps.features["host"] = true
		caps.featureFlags["host"] = spec
	}

	// ── CPU threads ───────────────────────────────────────────────────────────
	// -t / --threads: set the number of CPU threads used for generation.
	if spec, ok := firstOf(rawFlags, "--threads", "-t"); ok {
		caps.features["threads"] = true
		caps.featureFlags["threads"] = spec
	}

	// ── NUMA topology ─────────────────────────────────────────────────────────
	if spec, ok := rawFlags["--numa"]; ok {
		caps.features["numa"] = true
		caps.featureFlags["numa"] = spec
	}

	// ── Verbose/log level ─────────────────────────────────────────────────────
	if spec, ok := rawFlags["--log-disable"]; ok {
		caps.features["log_disable"] = true
		caps.featureFlags["log_disable"] = spec
	}

	return caps
}

// ─── Private helpers ──────────────────────────────────────────────────────────

// hasAll returns true only when every flag in names exists in rawFlags.
func hasAll(rawFlags map[string]FlagSpec, names ...string) bool {
	for _, n := range names {
		if _, ok := rawFlags[n]; !ok {
			return false
		}
	}
	return true
}

// firstOf returns the FlagSpec for the first flag in names that exists in
// rawFlags. Returns (FlagSpec{}, false) if none match.
func firstOf(rawFlags map[string]FlagSpec, names ...string) (FlagSpec, bool) {
	for _, n := range names {
		if spec, ok := rawFlags[n]; ok {
			return spec, true
		}
	}
	return FlagSpec{}, false
}

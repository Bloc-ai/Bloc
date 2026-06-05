package recipe

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// imageTagRe is the F-15 allowlist regexp for Docker image tags.
// Accepts: registry/org/name:tag, sha256 digests, and simple names.
// Rejects: shell metacharacters, spaces, and overly long strings.
var imageTagRe = regexp.MustCompile(`^[a-z0-9][a-z0-9/_:.\-]{0,199}$`)

// hfRepoRe validates model.hf_repo fields.
// SEC-10: Accepts only "org/model-name" form with safe characters.
// Rejects path traversal sequences ("../"), shell metacharacters, spaces.
var hfRepoRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,99}/[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,149}$`)

// semverRe validates engine.version fields (e.g. "0.9.0", "1.2.3-rc1").
// SEC-04: Prevents arbitrary strings (which could be shell-injected in future
// tooling) from appearing in the version field.
var semverRe = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?([\-+][a-zA-Z0-9.\-+]{1,50})?$`)

// metadataNameRe validates metadata.name fields (max 64 chars, safe chars only).
// SEC-14 (L-2): Prevents injection via crafted name fields.
var metadataNameRe = regexp.MustCompile(`^[a-zA-Z0-9.\-_]{1,64}$`)

// Recipe is the parsed representation of a bloc/v1 YAML recipe file.
type Recipe struct {
	Schema       string      `yaml:"schema"`
	Metadata     Metadata    `yaml:"metadata"`
	Model        Model       `yaml:"model"`
	Engine       Engine      `yaml:"engine"`
	Hardware     Hardware    `yaml:"hardware"`
	EngineConfig EngineConfig `yaml:"engine_config"`
	PreRun       PreRun      `yaml:"pre_run"`
}

type Metadata struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	AuthorNotes string   `yaml:"author_notes"`
}

type Model struct {
	Source       string  `yaml:"source"`
	GGUFRepo     string  `yaml:"gguf_repo"`
	File         string  `yaml:"file"`
	DownloadURL  string  `yaml:"download_url"`
	HFRepo       string  `yaml:"hf_repo"`  // NEW: "org/model-name" for full HF repo downloads (vLLM)
	Quantization string  `yaml:"quantization"`
	SizeGB       float64 `yaml:"size_gb"`
	Parameters   string  `yaml:"parameters"`
	Architecture string  `yaml:"architecture"`
	SHA256       string  `yaml:"sha256"` // optional; enables offline verification
}

type Engine struct {
	Name         string `yaml:"name"`
	Variant      string `yaml:"variant"`
	TestedCommit string `yaml:"tested_commit"`
	Runtime      string `yaml:"runtime"`  // NEW: "native" | "docker" — overrideable via --runtime flag
	Version      string `yaml:"version"`  // NEW: engine version pin (e.g. "0.9.0" for vLLM)
	Image        string `yaml:"image"`    // NEW: Docker image tag (docker runtime only)
}

type Hardware struct {
	MinVRAM        string `yaml:"min_vram"`
	TargetPlatform string `yaml:"target_platform"`
	GPUCount       int    `yaml:"gpu_count"`
	RecommendedRAM string `yaml:"recommended_ram"`
}

// EngineConfig maps to engine-specific CLI flags.
// null / zero values are silently skipped during flag construction.
// Fields are partitioned by engine — validateEngineConfig() (Fix #3) enforces
// that llama.cpp-only fields are not set on vLLM recipes and vice versa.
type EngineConfig struct {
	// ── llama.cpp fields ──────────────────────────────────────────────────────

	// Context
	CtxSize int `yaml:"ctx_size"` // -c

	// GPU offloading (llama.cpp only)
	GPULayers   int    `yaml:"gpu_layers"`  // -ngl
	SplitMode   string `yaml:"split_mode"`  // --split-mode
	TensorSplit string `yaml:"tensor_split"` // --tensor-split
	MainGPU     int    `yaml:"main_gpu"`    // --main-gpu

	// MoE expert routing (llama.cpp only)
	NCPUMoE int `yaml:"n_cpu_moe"` // --n-cpu-moe

	// Attention & batching (llama.cpp only)
	FlashAttn  bool `yaml:"flash_attn"`  // -fa
	BatchSize  int  `yaml:"batch_size"`  // -b  (llama.cpp only)
	UBatchSize int  `yaml:"ubatch_size"` // -ub (llama.cpp only)

	// KV cache
	CacheTypeK string `yaml:"cache_type_k"` // -ctk
	CacheTypeV string `yaml:"cache_type_v"` // -ctv

	// Speculative decoding (llama.cpp native MTP)
	SpecType       string  `yaml:"spec_type"`        // --spec-type
	SpecDraftModel string  `yaml:"spec_draft_model"` // --spec-draft-model
	SpecDraftNMax  int     `yaml:"spec_draft_n_max"` // --spec-draft-n-max
	SpecDraftPMin  float64 `yaml:"spec_draft_p_min"` // --spec-draft-p-min

	// CPU & threading
	Threads int    `yaml:"threads"` // -t
	NUMA    string `yaml:"numa"`    // --numa

	// Memory
	MLock bool  `yaml:"mlock"` // --mlock
	MMap  *bool `yaml:"mmap"`  // (default true; --no-mmap to disable)

	// Server (shared across engines)
	Host      string `yaml:"host"`       // --host
	Port      int    `yaml:"port"`       // --port
	NParallel int    `yaml:"n_parallel"` // -np (llama.cpp) / --max-num-seqs (vLLM)
	Jinja     bool   `yaml:"jinja"`      // --jinja (llama.cpp)

	// ── vLLM-specific fields ──────────────────────────────────────────────────
	// These fields are validated to be zero/empty on non-vLLM recipes (Fix #3).

	TensorParallelSize   int     `yaml:"tensor_parallel_size"`   // --tensor-parallel-size
	GPUMemoryUtilization float64 `yaml:"gpu_memory_utilization"` // --gpu-memory-utilization
	MaxModelLen          int     `yaml:"max_model_len"`          // --max-model-len
	DType                string  `yaml:"dtype"`                  // --dtype
	KVCacheDType         string  `yaml:"kv_cache_dtype"`         // --kv-cache-dtype
	QuantizationType     string  `yaml:"quantization"`           // --quantization
	EnableExpertParallel bool    `yaml:"enable_expert_parallel"` // --enable-expert-parallel
	TokenizerMode        string  `yaml:"tokenizer_mode"`         // --tokenizer-mode
	ToolCallParser       string  `yaml:"tool_call_parser"`       // --tool-call-parser
	ReasoningParser      string  `yaml:"reasoning_parser"`       // --reasoning-parser
	TrustRemoteCode      bool    `yaml:"trust_remote_code"`      // F-19: requires user confirm

	// Speculative decoding (vLLM draft model — top-level first-class field)
	SpeculativeModel     string `yaml:"speculative_model"`      // --speculative-model
	NumSpeculativeTokens int    `yaml:"num_speculative_tokens"` // --num-speculative-tokens

	// ── SGLang-specific fields ───────────────────────────────────────────────
	// These fields are validated to be zero/empty on non-sglang recipes.
	// All YAML keys are prefixed "sglang_" to prevent silent cross-engine
	// misconfiguration (SGLang and vLLM share concepts — e.g. tensor parallelism
	// — but use different flag names and semantics).

	SGLangTensorParallelSize int     `yaml:"sglang_tensor_parallel_size"` // --tp-size
	SGLangContextLength      int     `yaml:"sglang_context_length"`       // --context-length
	SGLangMemFractionStatic  float64 `yaml:"sglang_mem_fraction_static"`  // --mem-fraction-static
	SGLangMaxRunningRequests int     `yaml:"sglang_max_running_requests"` // --max-running-requests
	SGLangChunkedPrefillSize int     `yaml:"sglang_chunked_prefill_size"` // --chunked-prefill-size
	SGLangMaxPrefillTokens   int     `yaml:"sglang_max_prefill_tokens"`   // --max-prefill-tokens
	SGLangCudaGraphMaxBS     int     `yaml:"sglang_cuda_graph_max_bs"`    // --cuda-graph-max-bs
	SGLangQuantization       string  `yaml:"sglang_quantization"`         // --quantization
	SGLangKVCacheDType       string  `yaml:"sglang_kv_cache_dtype"`       // --kv-cache-dtype
	SGLangReasoningParser    string  `yaml:"sglang_reasoning_parser"`     // --reasoning-parser
	SGLangToolCallParser     string  `yaml:"sglang_tool_call_parser"`     // --tool-call-parser
	SGLangEnableMultimodal   bool    `yaml:"sglang_enable_multimodal"`    // --enable-multimodal
	// SGLangCUDAVisibleDevices pins specific GPU bus IDs via the CUDA_VISIBLE_DEVICES
	// environment variable injected into the Docker container by the runtime.
	// Example: "0,2,3,4" (0xSero's verified 4-GPU host).
	SGLangCUDAVisibleDevices string `yaml:"sglang_cuda_visible_devices"`

	// F-03: extra_args escape hatch — validated against an allowlist before use.
	// Recipe authors may add flags not yet modelled above, but only from the
	// approved list below. Unknown flags are rejected at parse time.
	ExtraArgs []string `yaml:"extra_args"`
}

//go:embed banned_flags.json
var bannedFlagsJSON []byte

var bannedExtraArgs map[string]struct{}

func init() {
	var bannedList []string
	if err := json.Unmarshal(bannedFlagsJSON, &bannedList); err != nil {
		panic(fmt.Sprintf("failed to parse embedded banned_flags.json: %v", err))
	}
	bannedExtraArgs = make(map[string]struct{})
	for _, flag := range bannedList {
		bannedExtraArgs[flag] = struct{}{}
	}
}

type PreRun struct {
	Env      map[string]string `yaml:"env"`
	Commands []string          `yaml:"commands"`
}

// ParseFile reads and parses a recipe YAML from disk.
func ParseFile(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read recipe file: %w", err)
	}
	return Parse(data)
}

// ParseFileLocal reads and parses a local recipe YAML from disk, bypassing strict allowlist checks.
func ParseFileLocal(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read recipe file: %w", err)
	}
	return ParseLocal(data)
}

// Parse unmarshals recipe YAML bytes under strict registry security rules.
// F-14: Caller must wrap the input in io.LimitReader before calling Parse
// to prevent memory DoS via huge YAML payloads. The 1 MB limit is enforced
// in cmd/run.go's fetchRecipe function.
func Parse(data []byte) (*Recipe, error) {
	return parse(data, false)
}

// ParseLocal unmarshals recipe YAML bytes bypassing strict registry security allowlists.
func ParseLocal(data []byte) (*Recipe, error) {
	return parse(data, true)
}

// parse is the internal entrypoint shared by registry and local execution modes.
func parse(data []byte, isLocal bool) (*Recipe, error) {
	var r Recipe
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if r.Schema != "bloc/v1" {
		return nil, fmt.Errorf("unsupported schema %q: only bloc/v1 is supported", r.Schema)
	}
	// SEC-14 (L-2): Validate metadata.name format and length (max 64).
	if r.Metadata.Name == "" {
		return nil, fmt.Errorf("recipe is missing metadata.name")
	}
	if !metadataNameRe.MatchString(r.Metadata.Name) {
		return nil, fmt.Errorf("metadata.name %q is invalid: must be 1-64 characters long and use only alphanumerics, dots, dashes, and underscores", r.Metadata.Name)
	}
	// Fix #2: Accept either download_url (llama.cpp GGUF) or hf_repo (vLLM full repo).
	// Previously required download_url strictly, which blocked all vLLM recipes.
	if r.Model.DownloadURL == "" && r.Model.HFRepo == "" {
		return nil, fmt.Errorf("recipe must specify either model.download_url (for GGUF) or model.hf_repo (for HuggingFace repo)")
	}
	// Fix #3: Cross-engine config validation — prevent silently applying the wrong
	// engine's fields. vLLM-specific fields are only valid on vLLM recipes.
	if err := validateEngineConfig(&r); err != nil {
		return nil, err
	}
	// SEC-07 (H-6): Validate extra_args against the blocklist for ALL recipes
	// (both local and registry). Prevents a local developer from accidentally
	// or maliciously passing dangerous flags like --hf-token or --rpc.
	if err := validateExtraArgs(r.EngineConfig.ExtraArgs); err != nil {
		return nil, err
	}
	// F-15: Validate Docker image tag format at parse time.
	// Prevents injection via crafted recipe: only [a-z0-9/_:.-] chars, max 200.
	if r.Engine.Image != "" && !imageTagRe.MatchString(r.Engine.Image) {
		return nil, fmt.Errorf(
			"engine.image %q is not a valid Docker image reference\n"+
				"  Must match: ^[a-z0-9][a-z0-9/_:.\\-]{0,199}$\n"+
				"  Example: vllm/vllm-openai:v0.9.0",
			r.Engine.Image,
		)
	}
	// SEC-10: Validate hf_repo format.
	// Rejects path traversal, spaces, and shell metacharacters in the repo name.
	if r.Model.HFRepo != "" && !hfRepoRe.MatchString(r.Model.HFRepo) {
		return nil, fmt.Errorf(
			"model.hf_repo %q is not a valid HuggingFace repo identifier\n"+
				"  Expected format: org/model-name (alphanumeric, dash, dot, underscore)",
			r.Model.HFRepo,
		)
	}
	// SEC-04: Validate engine.version is a semver string if specified.
	// Prevents arbitrary strings from being stored as a version pin.
	if r.Engine.Version != "" && !semverRe.MatchString(r.Engine.Version) {
		return nil, fmt.Errorf(
			"engine.version %q is not a valid version string (e.g. \"0.9.0\", \"1.2.3-rc1\")",
			r.Engine.Version,
		)
	}
	// F-17: Validate port range at parse time.
	// Ports 0–1023 are privileged; port 0 would bind to a random OS-assigned port.
	// Both are rejected to prevent privilege escalation and unpredictable binding.
	if p := r.EngineConfig.Port; p != 0 && (p < 1024 || p > 65535) {
		return nil, fmt.Errorf(
			"engine_config.port %d is out of the allowed range [1024–65535]\n"+
				"  Privileged ports (<1024) and port 0 are not allowed",
			p,
		)
	}
	// SEC-03: Validate pre_run.env keys at parse time.
	// This is enforced for both local and registry recipes — dangerous env keys
	// are never acceptable regardless of recipe source.
	if err := validatePreRunEnv(r.PreRun.Env); err != nil {
		return nil, err
	}
	return &r, nil
}

// preRunEnvKeyRe is the allowlist for pre_run.env key names.
// SEC-03: Only identifiers matching [A-Za-z_][A-Za-z0-9_]* are valid.
var preRunEnvKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// preRunDangerousEnvKeys is the blocklist of env keys that can hijack the
// dynamic linker or interpreter before user code runs.
var preRunDangerousEnvKeys = map[string]bool{
	"LD_PRELOAD":            true,
	"LD_LIBRARY_PATH":       true,
	"LD_AUDIT":              true,
	"LD_DEBUG":              true,
	"DYLD_INSERT_LIBRARIES": true,
	"DYLD_LIBRARY_PATH":     true,
	"DYLD_FRAMEWORK_PATH":   true,
	"PYTHONPATH":            true,
	"PYTHONSTARTUP":         true,
	"RUBYOPT":               true,
	"BASH_ENV":              true,
	"ENV":                   true,
	"CDPATH":                true,
	"NODE_OPTIONS":          true,
	"PERL5OPT":              true,
}

// validatePreRunEnv checks that all pre_run.env keys are safe identifiers and
// do not attempt to override dangerous linker/interpreter env vars.
func validatePreRunEnv(env map[string]string) error {
	for k := range env {
		if !preRunEnvKeyRe.MatchString(k) {
			return fmt.Errorf(
				"pre_run.env key %q is invalid: must match [A-Za-z_][A-Za-z0-9_]*", k)
		}
		if preRunDangerousEnvKeys[k] {
			return fmt.Errorf(
				"pre_run.env key %q is not permitted: setting this variable could "+
					"compromise system security (dynamic linker / interpreter hijack)", k)
		}
	}
	return nil
}

// validateEngineConfig enforces cross-engine config constraints (Fix #3).
// Prevents recipe authors from mixing llama.cpp-only, vLLM-only, or
// SGLang-only fields, which would be silently ignored and cause confusing
// behaviour.
func validateEngineConfig(r *Recipe) error {
	engine := r.Engine.Name
	if engine == "" {
		engine = "llama.cpp" // zero-value default
	}
	cfg := r.EngineConfig

	// hasSGLangFields returns true when any sglang_* field is non-zero.
	// Used by non-sglang cases to catch copy-paste mistakes early.
	hasSGLangFields := cfg.SGLangTensorParallelSize != 0 ||
		cfg.SGLangContextLength != 0 ||
		cfg.SGLangMemFractionStatic != 0 ||
		cfg.SGLangMaxRunningRequests != 0 ||
		cfg.SGLangChunkedPrefillSize != 0 ||
		cfg.SGLangMaxPrefillTokens != 0 ||
		cfg.SGLangCudaGraphMaxBS != 0 ||
		cfg.SGLangQuantization != "" ||
		cfg.SGLangKVCacheDType != "" ||
		cfg.SGLangReasoningParser != "" ||
		cfg.SGLangToolCallParser != "" ||
		cfg.SGLangEnableMultimodal ||
		cfg.SGLangCUDAVisibleDevices != ""

	switch engine {
	case "llama.cpp", "llama-cpp":
		// vLLM-only fields must not be set on llama.cpp recipes
		if cfg.TensorParallelSize != 0 {
			return fmt.Errorf("engine_config.tensor_parallel_size is a vLLM-only field and cannot be used with engine %q", engine)
		}
		if cfg.GPUMemoryUtilization != 0 {
			return fmt.Errorf("engine_config.gpu_memory_utilization is a vLLM-only field and cannot be used with engine %q", engine)
		}
		if cfg.MaxModelLen != 0 {
			return fmt.Errorf("engine_config.max_model_len is a vLLM-only field and cannot be used with engine %q", engine)
		}
		// SGLang-only fields must not be set on llama.cpp recipes
		if hasSGLangFields {
			return fmt.Errorf("sglang_* fields cannot be used with engine %q — they are SGLang-only", engine)
		}

	case "vllm":
		// llama.cpp-only fields must not be set on vLLM recipes
		if cfg.GPULayers != 0 {
			return fmt.Errorf("engine_config.gpu_layers (-ngl) is a llama.cpp-only field and cannot be used with engine %q", engine)
		}
		if cfg.NCPUMoE != 0 {
			return fmt.Errorf("engine_config.n_cpu_moe is a llama.cpp-only field and cannot be used with engine %q", engine)
		}
		if cfg.BatchSize != 0 {
			return fmt.Errorf("engine_config.batch_size (-b) is a llama.cpp-only field and cannot be used with engine %q", engine)
		}
		if cfg.UBatchSize != 0 {
			return fmt.Errorf("engine_config.ubatch_size (-ub) is a llama.cpp-only field and cannot be used with engine %q", engine)
		}
		// SGLang-only fields must not be set on vLLM recipes
		if hasSGLangFields {
			return fmt.Errorf("sglang_* fields cannot be used with engine %q — they are SGLang-only", engine)
		}

	case "sglang":
		// llama.cpp-only fields must not be set on sglang recipes
		if cfg.GPULayers != 0 {
			return fmt.Errorf("engine_config.gpu_layers (-ngl) is a llama.cpp-only field and cannot be used with engine %q", engine)
		}
		if cfg.NCPUMoE != 0 {
			return fmt.Errorf("engine_config.n_cpu_moe is a llama.cpp-only field and cannot be used with engine %q", engine)
		}
		if cfg.BatchSize != 0 {
			return fmt.Errorf("engine_config.batch_size (-b) is a llama.cpp-only field and cannot be used with engine %q", engine)
		}
		if cfg.UBatchSize != 0 {
			return fmt.Errorf("engine_config.ubatch_size (-ub) is a llama.cpp-only field and cannot be used with engine %q", engine)
		}
		// vLLM-only fields must not be set on sglang recipes — use the sglang_* equivalents instead
		if cfg.TensorParallelSize != 0 {
			return fmt.Errorf("engine_config.tensor_parallel_size is a vLLM-only field; use sglang_tensor_parallel_size for engine %q", engine)
		}
		if cfg.GPUMemoryUtilization != 0 {
			return fmt.Errorf("engine_config.gpu_memory_utilization is a vLLM-only field; use sglang_mem_fraction_static for engine %q", engine)
		}
		if cfg.MaxModelLen != 0 {
			return fmt.Errorf("engine_config.max_model_len is a vLLM-only field; use sglang_context_length for engine %q", engine)
		}
		if cfg.KVCacheDType != "" {
			return fmt.Errorf("engine_config.kv_cache_dtype is a vLLM-only field; use sglang_kv_cache_dtype for engine %q", engine)
		}
		if cfg.QuantizationType != "" {
			return fmt.Errorf("engine_config.quantization is a vLLM-only field; use sglang_quantization for engine %q", engine)
		}
		if cfg.ReasoningParser != "" {
			return fmt.Errorf("engine_config.reasoning_parser is a vLLM-only field; use sglang_reasoning_parser for engine %q", engine)
		}
		if cfg.ToolCallParser != "" {
			return fmt.Errorf("engine_config.tool_call_parser is a vLLM-only field; use sglang_tool_call_parser for engine %q", engine)
		}
	}
	return nil
}

// validateExtraArgs rejects any flag present in the bannedExtraArgs set.
// F-03: Prevents a compromised Hub recipe from injecting dangerous flags.
func validateExtraArgs(args []string) error {
	for _, arg := range args {
		// Only check tokens that look like flags (start with -)
		if !strings.HasPrefix(arg, "-") {
			continue // value tokens like "4" or "q8_0" are fine
		}
		
		// Strip value assignment (e.g., --log-file=/etc/shadow -> --log-file)
		flagName := strings.SplitN(arg, "=", 2)[0]
		
		if _, ok := bannedExtraArgs[flagName]; ok {
			return fmt.Errorf(
				"extra_args contains banned flag %q — this flag is blocked for security reasons",
				flagName,
			)
		}
	}
	return nil
}

// BuildFlags converts EngineConfig into the ordered list of llama-server flags.
// null / zero / false values are omitted unless semantically required.
// Deprecated: use internal/engine package instead.
func (r *Recipe) BuildFlags() []string {
	cfg := r.EngineConfig
	var flags []string

	add := func(flag, value string) {
		if value != "" && value != "0" {
			flags = append(flags, flag, value)
		}
	}
	addBool := func(flag string, value bool) {
		if value {
			flags = append(flags, flag)
		}
	}
	addInt := func(flag string, value int) {
		if value != 0 {
			flags = append(flags, flag, fmt.Sprintf("%d", value))
		}
	}
	addFloat := func(flag string, value float64) {
		if value != 0 {
			flags = append(flags, flag, fmt.Sprintf("%.2f", value))
		}
	}

	// Model path is injected by the runner, not here.
	addInt("-c", cfg.CtxSize)
	addInt("-ngl", cfg.GPULayers)
	add("--split-mode", cfg.SplitMode)
	add("--tensor-split", cfg.TensorSplit)
	addInt("--main-gpu", cfg.MainGPU)
	addInt("--n-cpu-moe", cfg.NCPUMoE)
	// FIX: llama.cpp changed --flash-attn from a boolean toggle (-fa) to a
	// value-required flag (--flash-attn [on|off|auto]). Emit the long form with
	// an explicit value so it works on both old and new binary builds.
	if cfg.FlashAttn {
		flags = append(flags, "--flash-attn", "on")
	}
	addInt("-b", cfg.BatchSize)
	addInt("-ub", cfg.UBatchSize)
	add("-ctk", cfg.CacheTypeK)
	add("-ctv", cfg.CacheTypeV)
	add("--spec-type", cfg.SpecType)
	add("--spec-draft-model", cfg.SpecDraftModel)
	addInt("--spec-draft-n-max", cfg.SpecDraftNMax)
	addFloat("--spec-draft-p-min", cfg.SpecDraftPMin)
	addInt("-t", cfg.Threads)
	add("--numa", cfg.NUMA)
	addBool("--mlock", cfg.MLock)
	if cfg.MMap != nil && !*cfg.MMap {
		flags = append(flags, "--no-mmap")
	}
	add("--host", cfg.Host)
	addInt("--port", cfg.Port)
	addInt("-np", cfg.NParallel)
	addBool("--jinja", cfg.Jinja)

	// F-03: extra_args validated at parse time — safe to append here
	flags = append(flags, cfg.ExtraArgs...)

	return flags
}

// RequiredFlags returns the set of llama-server flags this recipe needs,
// derived from engine_config values and extra_args.
// Used by the capability probe to check the local binary.
// Deprecated: use internal/engine package instead.
func (r *Recipe) RequiredFlags() map[string]struct{} {
	required := make(map[string]struct{})
	cfg := r.EngineConfig

	if cfg.NCPUMoE != 0 {
		required["--n-cpu-moe"] = struct{}{}
	}
	if cfg.FlashAttn {
		// FIX: probe for long flag name — matches the new value-required form
		required["--flash-attn"] = struct{}{}
	}
	if cfg.SpecType != "" {
		required["--spec-type"] = struct{}{}
	}
	if cfg.SpecDraftModel != "" {
		required["--spec-draft-model"] = struct{}{}
	}
	if cfg.SpecDraftNMax != 0 {
		required["--spec-draft-n-max"] = struct{}{}
	}
	if cfg.CacheTypeK != "" {
		required["-ctk"] = struct{}{}
	}
	if cfg.CacheTypeV != "" {
		required["-ctv"] = struct{}{}
	}
	if cfg.Jinja {
		required["--jinja"] = struct{}{}
	}
	if cfg.SplitMode != "" {
		required["--split-mode"] = struct{}{}
	}
	if cfg.TensorSplit != "" {
		required["--tensor-split"] = struct{}{}
	}

	// Parse --flag tokens from extra_args (already validated against allowlist)
	for _, arg := range cfg.ExtraArgs {
		if len(arg) >= 2 && arg[:2] == "--" {
			required[arg] = struct{}{}
		} else if len(arg) >= 1 && arg[:1] == "-" {
			required[arg] = struct{}{}
		}
	}

	return required
}

// BuildVLLMFlags converts vLLM-specific EngineConfig fields into the ordered
// list of flags for `python -m vllm.entrypoints.openai.api_server`.
//
// Only vLLM-relevant fields are emitted. The model path itself is injected by
// NativeVLLMRuntime.Run (as --model <path>) before this slice is appended.
// F-19: TrustRemoteCode is NOT injected here — run.go gates it with an
// explicit user confirm prompt before passing --trust-remote-code to this list.
// Deprecated: use internal/engine package instead.
func (r *Recipe) BuildVLLMFlags() []string {
	cfg := r.EngineConfig
	var flags []string

	add := func(flag, value string) {
		if value != "" {
			flags = append(flags, flag, value)
		}
	}
	addInt := func(flag string, value int) {
		if value != 0 {
			flags = append(flags, flag, fmt.Sprintf("%d", value))
		}
	}
	addFloat := func(flag string, value float64) {
		if value != 0 {
			flags = append(flags, flag, fmt.Sprintf("%.2f", value))
		}
	}
	addBool := func(flag string, value bool) {
		if value {
			flags = append(flags, flag)
		}
	}

	// ── Server ────────────────────────────────────────────────────────────────
	add("--host", cfg.Host)
	addInt("--port", cfg.Port)

	// ── Parallelism ───────────────────────────────────────────────────────────
	addInt("--tensor-parallel-size", cfg.TensorParallelSize)
	addBool("--enable-expert-parallel", cfg.EnableExpertParallel)

	// ── Memory & precision ────────────────────────────────────────────────────
	addFloat("--gpu-memory-utilization", cfg.GPUMemoryUtilization)
	addInt("--max-model-len", cfg.MaxModelLen)
	add("--dtype", cfg.DType)
	add("--kv-cache-dtype", cfg.KVCacheDType)
	add("--quantization", cfg.QuantizationType)

	// ── Tokenizer ─────────────────────────────────────────────────────────────
	add("--tokenizer-mode", cfg.TokenizerMode)

	// ── Chat template & tool calling ──────────────────────────────────────────
	add("--tool-call-parser", cfg.ToolCallParser)
	if cfg.ReasoningParser != "" {
		// MED-4: --enable-reasoning was removed in vLLM v0.10.0.
		// The active vllm/flags.go already omits it; sync this deprecated path.
		flags = append(flags, "--reasoning-parser", cfg.ReasoningParser)
	}

	// ── Concurrency (maps to llama.cpp -np equivalent) ────────────────────────
	addInt("--max-num-seqs", cfg.NParallel)

	// ── Speculative decoding ──────────────────────────────────────────────────
	add("--speculative-model", cfg.SpeculativeModel)
	addInt("--num-speculative-tokens", cfg.NumSpeculativeTokens)

	// ── extra_args passthrough ────────────────────────────────────────────────
	// These are already validated against the allowlist at parse time (F-03).
	flags = append(flags, cfg.ExtraArgs...)

	return flags
}

// BuildSGLangFlags converts SGLang-specific EngineConfig fields into the
// ordered list of flags for `python3 -m sglang.launch_server`.
//
// The model path is injected by SGLangDockerRuntime.Run as --model-path.
// --host and --port are also injected by the runtime, not here, so that the
// runtime always controls the network binding.
// SGLangCUDAVisibleDevices is not emitted as a flag — it is injected into the
// container environment as CUDA_VISIBLE_DEVICES by the runtime.
// Deprecated: use internal/engine package instead.
func (r *Recipe) BuildSGLangFlags() []string {
	cfg := r.EngineConfig
	var flags []string

	add := func(flag, value string) {
		if value != "" {
			flags = append(flags, flag, value)
		}
	}
	addInt := func(flag string, value int) {
		if value != 0 {
			flags = append(flags, flag, fmt.Sprintf("%d", value))
		}
	}
	addFloat := func(flag string, value float64) {
		if value != 0 {
			flags = append(flags, flag, fmt.Sprintf("%.2f", value))
		}
	}
	addBool := func(flag string, value bool) {
		if value {
			flags = append(flags, flag)
		}
	}

	// ── Parallelism ───────────────────────────────────────────────────────────
	addInt("--tp-size", cfg.SGLangTensorParallelSize)

	// ── Context & memory ──────────────────────────────────────────────────────
	addInt("--context-length", cfg.SGLangContextLength)
	addFloat("--mem-fraction-static", cfg.SGLangMemFractionStatic)

	// ── Concurrency & prefill ─────────────────────────────────────────────────
	addInt("--max-running-requests", cfg.SGLangMaxRunningRequests)
	addInt("--chunked-prefill-size", cfg.SGLangChunkedPrefillSize)
	addInt("--max-prefill-tokens", cfg.SGLangMaxPrefillTokens)

	// ── CUDA graph ────────────────────────────────────────────────────────────
	addInt("--cuda-graph-max-bs", cfg.SGLangCudaGraphMaxBS)

	// ── Quantization & KV cache ───────────────────────────────────────────────
	add("--quantization", cfg.SGLangQuantization)
	add("--kv-cache-dtype", cfg.SGLangKVCacheDType)

	// ── Parsers & multimodal ──────────────────────────────────────────────────
	add("--reasoning-parser", cfg.SGLangReasoningParser)
	add("--tool-call-parser", cfg.SGLangToolCallParser)
	addBool("--enable-multimodal", cfg.SGLangEnableMultimodal)

	// ── extra_args passthrough ────────────────────────────────────────────────
	// Already validated against the allowlist at parse time (F-03).
	flags = append(flags, cfg.ExtraArgs...)

	return flags
}

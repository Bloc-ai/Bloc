package recipe

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

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
}

type Hardware struct {
	MinVRAM        string `yaml:"min_vram"`
	TargetPlatform string `yaml:"target_platform"`
	GPUCount       int    `yaml:"gpu_count"`
	RecommendedRAM string `yaml:"recommended_ram"`
}

// EngineConfig maps to llama-server CLI flags.
// null / zero values are silently skipped during flag construction.
type EngineConfig struct {
	// Context
	CtxSize int `yaml:"ctx_size"`

	// GPU offloading
	GPULayers   int    `yaml:"gpu_layers"`
	SplitMode   string `yaml:"split_mode"`
	TensorSplit string `yaml:"tensor_split"`
	MainGPU     int    `yaml:"main_gpu"`

	// MoE expert routing
	NCPUMoE int `yaml:"n_cpu_moe"` // --n-cpu-moe

	// Attention & batching
	FlashAttn  bool `yaml:"flash_attn"`  // -fa
	BatchSize  int  `yaml:"batch_size"`  // -b
	UBatchSize int  `yaml:"ubatch_size"` // -ub

	// KV cache
	CacheTypeK string `yaml:"cache_type_k"` // -ctk
	CacheTypeV string `yaml:"cache_type_v"` // -ctv

	// Speculative decoding
	SpecType       string  `yaml:"spec_type"`        // --spec-type
	SpecDraftModel string  `yaml:"spec_draft_model"` // --model-draft
	SpecDraftNMax  int     `yaml:"spec_draft_n_max"` // --draft
	SpecDraftPMin  float64 `yaml:"spec_draft_p_min"` // --draft-p-min

	// CPU & threading
	Threads int    `yaml:"threads"` // -t
	NUMA    string `yaml:"numa"`    // --numa

	// Memory
	MLock bool `yaml:"mlock"` // --mlock
	MMap  *bool `yaml:"mmap"`  // (default true; --no-mmap to disable)

	// Server
	Host      string `yaml:"host"`       // --host
	Port      int    `yaml:"port"`       // --port
	NParallel int    `yaml:"n_parallel"` // -np
	Jinja     bool   `yaml:"jinja"`      // --jinja

	// F-03: extra_args escape hatch — validated against an allowlist before use.
	// Recipe authors may add flags not yet modelled above, but only from the
	// approved list below. Unknown flags are rejected at parse time.
	ExtraArgs []string `yaml:"extra_args"`
}

// allowedExtraArgs is the set of llama-server flags permitted in extra_args.
// F-03: This prevents recipe authors (or a compromised Hub) from injecting
// dangerous flags like --host 0.0.0.0, --api-key "", or future plugin flags.
// Add new flags here as llama.cpp adds them and they are reviewed safe.
var allowedExtraArgs = map[string]struct{}{
	// Speculative / MTP decoding (new llama.cpp features)
	"--draft-mtp":         {},
	"--draft-mtp-steps":   {},
	"--draft-max":         {},
	"--draft-min":         {},
	"--draft-p-min":       {},
	"--spec-type":         {},
	// KV cache quantisation variants
	"-ctk":               {},
	"-ctv":               {},
	"--cache-type-k":     {},
	"--cache-type-v":     {},
	// Rope scaling
	"--rope-scale":        {},
	"--rope-freq-base":    {},
	"--rope-freq-scale":   {},
	"--yarn-orig-ctx":     {},
	// Grammars / sampling
	"--grammar":           {},
	"--grammar-file":      {},
	"--json-schema":       {},
	// Embedding
	"--embedding":         {},
	"--reranking":         {},
	// Logging verbosity
	"--log-disable":       {},
	"--verbose":           {},
	"-v":                  {},
	// Flash attention variant
	"--flash-attn":        {},
	"-fa":                 {},
	// Jinja templating
	"--jinja":             {},
	// Batching
	"-b":                  {},
	"-ub":                 {},
	"--batch-size":        {},
	"--ubatch-size":       {},
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

// Parse unmarshals recipe YAML bytes.
// F-14: Caller must wrap the input in io.LimitReader before calling Parse
// to prevent memory DoS via huge YAML payloads. The 1 MB limit is enforced
// in cmd/deploy.go's fetchRecipe function.
func Parse(data []byte) (*Recipe, error) {
	var r Recipe
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if r.Schema != "bloc/v1" {
		return nil, fmt.Errorf("unsupported schema %q: only bloc/v1 is supported", r.Schema)
	}
	if r.Metadata.Name == "" {
		return nil, fmt.Errorf("recipe is missing metadata.name")
	}
	if r.Model.DownloadURL == "" {
		return nil, fmt.Errorf("recipe is missing model.download_url")
	}
	// F-03: Validate extra_args against the allowlist
	if err := validateExtraArgs(r.EngineConfig.ExtraArgs); err != nil {
		return nil, err
	}
	return &r, nil
}

// validateExtraArgs rejects any flag not in the allowedExtraArgs set.
// F-03: Prevents a compromised Hub recipe from injecting dangerous llama-server flags.
func validateExtraArgs(args []string) error {
	for _, arg := range args {
		// Only check tokens that look like flags (start with -)
		if !strings.HasPrefix(arg, "-") {
			continue // value tokens like "4" or "q8_0" are fine
		}
		if _, ok := allowedExtraArgs[arg]; !ok {
			return fmt.Errorf(
				"extra_args contains disallowed flag %q — contact the recipe author or open an issue at https://github.com/bloc-org/bloc",
				arg,
			)
		}
	}
	return nil
}

// BuildFlags converts EngineConfig into the ordered list of llama-server flags.
// null / zero / false values are omitted unless semantically required.
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
	addBool("-fa", cfg.FlashAttn)
	addInt("-b", cfg.BatchSize)
	addInt("-ub", cfg.UBatchSize)
	add("-ctk", cfg.CacheTypeK)
	add("-ctv", cfg.CacheTypeV)
	add("--spec-type", cfg.SpecType)
	add("--model-draft", cfg.SpecDraftModel)
	addInt("--draft", cfg.SpecDraftNMax)
	addFloat("--draft-p-min", cfg.SpecDraftPMin)
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
func (r *Recipe) RequiredFlags() map[string]struct{} {
	required := make(map[string]struct{})
	cfg := r.EngineConfig

	if cfg.NCPUMoE != 0 {
		required["--n-cpu-moe"] = struct{}{}
	}
	if cfg.FlashAttn {
		required["-fa"] = struct{}{}
	}
	if cfg.SpecType != "" {
		required["--spec-type"] = struct{}{}
	}
	if cfg.SpecDraftModel != "" {
		required["--model-draft"] = struct{}{}
	}
	if cfg.SpecDraftNMax != 0 {
		required["--draft"] = struct{}{}
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

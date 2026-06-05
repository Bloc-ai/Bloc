package vllm

import (
	"fmt"
	"github.com/bloc-org/bloc/internal/recipe"
)

// BuildFlags converts engine configuration into the ordered list of flags for vLLM.
func BuildFlags(cfg recipe.EngineConfig) []string {
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

	add("--host", cfg.Host)
	addInt("--port", cfg.Port)

	addInt("--tensor-parallel-size", cfg.TensorParallelSize)
	addBool("--enable-expert-parallel", cfg.EnableExpertParallel)

	addFloat("--gpu-memory-utilization", cfg.GPUMemoryUtilization)
	addInt("--max-model-len", cfg.MaxModelLen)
	add("--dtype", cfg.DType)
	add("--kv-cache-dtype", cfg.KVCacheDType)
	add("--quantization", cfg.QuantizationType)

	add("--tokenizer-mode", cfg.TokenizerMode)

	add("--tool-call-parser", cfg.ToolCallParser)
	// --reasoning-parser enables reasoning mode on vLLM (added ~v0.7.2).
	// NOTE: --enable-reasoning was paired with --reasoning-parser until v0.9.x,
	// but was REMOVED in vLLM v0.10.0. Specifying --reasoning-parser alone is
	// sufficient on all supported vLLM versions (v0.7.2+). We never emit
	// --enable-reasoning to stay compatible with v0.10+.
	if cfg.ReasoningParser != "" {
		flags = append(flags, "--reasoning-parser", cfg.ReasoningParser)
	}

	addInt("--max-num-seqs", cfg.NParallel)

	add("--speculative-model", cfg.SpeculativeModel)
	addInt("--num-speculative-tokens", cfg.NumSpeculativeTokens)

	flags = append(flags, cfg.ExtraArgs...)

	return flags
}

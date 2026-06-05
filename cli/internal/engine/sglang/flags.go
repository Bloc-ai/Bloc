package sglang

import (
	"fmt"
	"github.com/bloc-org/bloc/internal/recipe"
)

// BuildFlags converts engine configuration into the ordered list of flags for SGLang.
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

	addInt("--tp-size", cfg.SGLangTensorParallelSize)
	addInt("--context-length", cfg.SGLangContextLength)
	addFloat("--mem-fraction-static", cfg.SGLangMemFractionStatic)
	addInt("--max-running-requests", cfg.SGLangMaxRunningRequests)
	addInt("--chunked-prefill-size", cfg.SGLangChunkedPrefillSize)
	addInt("--max-prefill-tokens", cfg.SGLangMaxPrefillTokens)
	addInt("--cuda-graph-max-bs", cfg.SGLangCudaGraphMaxBS)
	add("--quantization", cfg.SGLangQuantization)
	add("--kv-cache-dtype", cfg.SGLangKVCacheDType)
	add("--reasoning-parser", cfg.SGLangReasoningParser)
	add("--tool-call-parser", cfg.SGLangToolCallParser)
	addBool("--enable-multimodal", cfg.SGLangEnableMultimodal)

	flags = append(flags, cfg.ExtraArgs...)

	return flags
}

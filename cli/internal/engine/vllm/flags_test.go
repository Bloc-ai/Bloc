package vllm

import (
	"regexp"
	"testing"
	"github.com/bloc-org/bloc/internal/recipe"
)

func TestBuildFlags_Empty(t *testing.T) {
	cfg := recipe.EngineConfig{}
	flags := BuildFlags(cfg)
	if len(flags) != 0 {
		t.Errorf("BuildFlags(empty) = %v, want []", flags)
	}
}

func TestBuildFlags_AllOptions(t *testing.T) {
	cfg := recipe.EngineConfig{
		Host:                 "0.0.0.0",
		Port:                 8080,
		TensorParallelSize:   4,
		EnableExpertParallel: true,
		GPUMemoryUtilization: 0.95,
		MaxModelLen:          8192,
		DType:                "half",
		KVCacheDType:         "fp8",
		QuantizationType:     "awq",
		TokenizerMode:        "auto",
		ToolCallParser:       "hermes",
		ReasoningParser:      "deepseek_r1",
		NParallel:            128,
		SpeculativeModel:     "ibm-granite/granite-3.0-2b-instruct",
		NumSpeculativeTokens: 5,
		ExtraArgs:            []string{"--trust-remote-code"},
	}
	flags := BuildFlags(cfg)
	if len(flags) == 0 {
		t.Errorf("expected flags to be built")
	}
}

func TestResolveVersion_Pinned(t *testing.T) {
	got := resolveVersion("0.8.5")
	if got != "0.8.5" {
		t.Errorf("expected 0.8.5, got %s", got)
	}
}

func TestResolveVersion_Default(t *testing.T) {
	got := resolveVersion("")
	if got != defaultVLLMVersion {
		t.Errorf("expected defaultVLLMVersion %q, got %q", defaultVLLMVersion, got)
	}
}

func TestParseVLLMStats_PromptThroughput(t *testing.T) {
	p := &VLLMLogParser{}
	m := p.ParseLine("Avg prompt throughput: 123.4 tokens/s, Avg generation throughput: 45.6 tokens/s, ...")
	if m.TokensPerSecPrefill != 123.4 {
		t.Errorf("TokensPerSecPrefill = %v, want 123.4", m.TokensPerSecPrefill)
	}
	if m.TokensPerSecGen != 45.6 {
		t.Errorf("TokensPerSecGeneration = %v, want 45.6", m.TokensPerSecGen)
	}
}

func TestParseVLLMStats_KVCacheUsage(t *testing.T) {
	p := &VLLMLogParser{}
	m := p.ParseLine("GPU KV cache usage: 72.5%, CPU KV cache usage: 0.0%.")
	if m.PeakVRAMMB != 7250 {
		t.Errorf("PeakVRAMMB = %v, want 7250", m.PeakVRAMMB)
	}
}

func TestParseVLLMStats_PeakOnly_Increases(t *testing.T) {
	p := &VLLMLogParser{}
	p.ParseLine("GPU KV cache usage: 50.0%.")
	p.ParseLine("GPU KV cache usage: 80.0%.")
	m := p.ParseLine("GPU KV cache usage: 60.0%.")
	if m.PeakVRAMMB != 8000 {
		t.Errorf("PeakVRAMMB = %v, want 8000 (peak)", m.PeakVRAMMB)
	}
}

func TestVLLMRegexPatterns(t *testing.T) {
	patterns := []string{
		vllmPromptRe.String(),
		vllmGenRe.String(),
		vllmKVRe.String(),
	}
	for _, p := range patterns {
		if _, err := regexp.Compile(p); err != nil {
			t.Errorf("regex pattern failed to compile: %q: %v", p, err)
		}
	}
}

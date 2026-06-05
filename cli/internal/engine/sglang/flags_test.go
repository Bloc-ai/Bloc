package sglang

import (
	"regexp"
	"strings"
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
		SGLangTensorParallelSize: 4,
		SGLangContextLength:      32768,
		SGLangMemFractionStatic:  0.9,
		SGLangMaxRunningRequests: 256,
		SGLangChunkedPrefillSize: 8192,
		SGLangMaxPrefillTokens:   16384,
		SGLangCudaGraphMaxBS:     128,
		SGLangQuantization:       "fp8",
		SGLangKVCacheDType:       "fp8_e5m2",
		SGLangReasoningParser:    "deepseek",
		SGLangToolCallParser:     "hermes",
		SGLangEnableMultimodal:   true,
		ExtraArgs:                []string{"--disable-radix-cache"},
	}
	flags := BuildFlags(cfg)
	if len(flags) == 0 {
		t.Errorf("expected flags to be built")
	}
}

func TestParseSGLangStats_GenThroughput(t *testing.T) {
	p := &SGLangLogParser{}
	m := p.ParseLine("throughput_output_token_per_s=47.3 throughput_input_token_per_s=912.1")
	if m.TokensPerSecGen != 47.3 {
		t.Errorf("TokensPerSecGeneration = %v, want 47.3", m.TokensPerSecGen)
	}
}

func TestParseSGLangStats_PrefillThroughput(t *testing.T) {
	p := &SGLangLogParser{}
	m := p.ParseLine("throughput_output_token_per_s=47.3 throughput_input_token_per_s=912.1")
	if m.TokensPerSecPrefill != 912.1 {
		t.Errorf("TokensPerSecPrefill = %v, want 912.1", m.TokensPerSecPrefill)
	}
}

func TestParseSGLangStats_VRAMUsage_MB(t *testing.T) {
	p := &SGLangLogParser{}
	m := p.ParseLine("Memory pool end size: 81920.00 MB")
	if m.PeakVRAMMB != 81920 {
		t.Errorf("PeakVRAMMB = %v, want 81920", m.PeakVRAMMB)
	}
}

func TestParseSGLangStats_VRAMUsage_GB(t *testing.T) {
	p := &SGLangLogParser{}
	m := p.ParseLine("gpu memory: 48.0 GB")
	// 48.0 GB * 1024 = 49152 MB
	if m.PeakVRAMMB != 49152 {
		t.Errorf("PeakVRAMMB = %v, want 49152 (48.0 GB in MB)", m.PeakVRAMMB)
	}
}

func TestParseSGLangStats_PeakVRAM_OnlyIncreases(t *testing.T) {
	p := &SGLangLogParser{}
	p.ParseLine("Memory pool end size: 40960.00 MB")
	p.ParseLine("Memory pool end size: 81920.00 MB")
	m := p.ParseLine("Memory pool end size: 20480.00 MB") // lower — must not replace peak
	if m.PeakVRAMMB != 81920 {
		t.Errorf("PeakVRAMMB = %v, want 81920 (peak should only increase)", m.PeakVRAMMB)
	}
}

func TestParseSGLangStats_RegexSafety(t *testing.T) {
	p := &SGLangLogParser{}
	longLine := strings.Repeat("throughput_output_token_per_s=", 1000) + "42.0"
	p.ParseLine(longLine)
	// If we reach here within the test timeout, no catastrophic backtracking occurred.
}

func TestSGLangRegexPatterns(t *testing.T) {
	patterns := []string{
		sglangGenRe.String(),
		sglangPrefillRe.String(),
		sglangVRAMRe.String(),
	}
	for _, pat := range patterns {
		if _, err := regexp.Compile(pat); err != nil {
			t.Errorf("regex pattern failed to compile: %q: %v", pat, err)
		}
	}
}

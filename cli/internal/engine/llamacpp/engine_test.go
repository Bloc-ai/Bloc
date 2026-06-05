package llamacpp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/recipe"
)

// ─── Fixture loading ──────────────────────────────────────────────────────────

// loadHelpFixture reads testdata/llama_help.txt and returns its contents.
// It fails the test immediately if the file cannot be read.
func loadHelpFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "llama_help.txt"))
	if err != nil {
		t.Fatalf("cannot read testdata/llama_help.txt: %v", err)
	}
	return string(data)
}

// capsFromFixture parses the fixture file and returns a CapabilitySet that
// mirrors what Capabilities() would produce from a real binary probe.
func capsFromFixture(t *testing.T) *engine.CapabilitySet {
	t.Helper()
	helpText := loadHelpFixture(t)
	rawFlags := parseFlagsFromHelp(helpText)
	return engine.BuildCapabilities("llama-server", "b4096", rawFlags)
}

// ─── parseFlagsFromHelp ───────────────────────────────────────────────────────

func TestParseFlagsFromHelp_CoreFlags(t *testing.T) {
	caps := capsFromFixture(t)

	required := []struct {
		feature string
		rawFlag string
	}{
		{"flash_attn", "-fa"},          // legacy short form in fixture
		{"gpu_layers", "-ngl"},
		{"ctx_size", "-c"},
		{"batch_size", "-b"},
		{"mmap", "--no-mmap"},
		{"mlock", "--mlock"},
		{"jinja", "--jinja"},
		{"split_mode", "--split-mode"},
		{"tensor_split", "--tensor-split"},
		{"parallel", "-np"},
		{"port", "--port"},
	}

	for _, tc := range required {
		t.Run(tc.feature, func(t *testing.T) {
			if !caps.HasFeature(tc.feature) {
				t.Errorf("feature %q not detected from help fixture (expected raw flag %s)", tc.feature, tc.rawFlag)
			}
		})
	}
}

func TestParseFlagsFromHelp_KVCacheTypes(t *testing.T) {
	caps := capsFromFixture(t)
	if !caps.HasFeature("kv_cache_types") {
		t.Error("kv_cache_types feature not detected — expected both -ctk and -ctv in fixture")
	}
}

func TestParseFlagsFromHelp_SpeculativeDecoding(t *testing.T) {
	caps := capsFromFixture(t)
	if !caps.HasFeature("speculative_decoding") {
		t.Error("speculative_decoding feature not detected from help fixture")
	}
}

func TestParseFlagsFromHelp_NCPUMoe(t *testing.T) {
	caps := capsFromFixture(t)
	if !caps.HasFeature("moe_cpu_offload") {
		t.Error("moe_cpu_offload feature not detected from help fixture (expected --n-cpu-moe)")
	}
}

// ─── BuildArgs — flash attention ──────────────────────────────────────────────

// TestBuildArgs_FlashAttn_OnOff verifies that when caps reports --flash-attn
// as a bool_on_off flag, BuildArgs emits ["--flash-attn", "on"].
func TestBuildArgs_FlashAttn_OnOff(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b4000", map[string]engine.FlagSpec{
		"--flash-attn": {Name: "--flash-attn", TakesValue: true, ValueType: engine.ValueTypeBoolOnOff},
		"-ngl":         {Name: "-ngl", TakesValue: true, ValueType: engine.ValueTypeInt},
		"-c":           {Name: "-c", TakesValue: true, ValueType: engine.ValueTypeInt},
	})

	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{FlashAttn: true})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}
	if !containsSeq(flags, "--flash-attn", "on") {
		t.Errorf("expected [--flash-attn on] in flags, got: %v", flags)
	}
}

// TestBuildArgs_FlashAttn_Implicit verifies that when caps reports -fa as a
// bool_implicit flag (legacy build), BuildArgs emits just ["-fa"].
func TestBuildArgs_FlashAttn_Implicit(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b2000", map[string]engine.FlagSpec{
		"-fa":  {Name: "-fa", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
		"-ngl": {Name: "-ngl", TakesValue: true, ValueType: engine.ValueTypeInt},
		"-c":   {Name: "-c", TakesValue: true, ValueType: engine.ValueTypeInt},
	})

	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{FlashAttn: true})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}
	if !contains(flags, "-fa") {
		t.Errorf("expected -fa in flags, got: %v", flags)
	}
	// Ensure we do NOT emit --flash-attn or "on" for the legacy form.
	for _, f := range flags {
		if f == "--flash-attn" {
			t.Errorf("unexpected --flash-attn emitted for legacy build; got: %v", flags)
		}
	}
}

// TestBuildArgs_FlashAttn_MissingCapability verifies that requesting FlashAttn
// when the binary does not support it returns a structured error.
func TestBuildArgs_FlashAttn_MissingCapability(t *testing.T) {
	// No flash attn flags in caps.
	caps := engine.BuildCapabilities("llama-server", "b999", map[string]engine.FlagSpec{
		"-c": {Name: "-c", TakesValue: true, ValueType: engine.ValueTypeInt},
	})

	eng := &Engine{}
	_, err := eng.BuildArgs(caps, recipe.EngineConfig{FlashAttn: true})
	if err == nil {
		t.Fatal("expected error when FlashAttn=true but flash_attn not in caps, got nil")
	}
	if !strings.Contains(err.Error(), "flash_attn") {
		t.Errorf("error message should mention flash_attn, got: %v", err)
	}
}

// ─── BuildArgs — KV cache types ───────────────────────────────────────────────

func TestBuildArgs_KVCacheTypes(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b4000", map[string]engine.FlagSpec{
		"-ctk": {Name: "-ctk", TakesValue: true, ValueType: engine.ValueTypeString},
		"-ctv": {Name: "-ctv", TakesValue: true, ValueType: engine.ValueTypeString},
		"-c":   {Name: "-c", TakesValue: true, ValueType: engine.ValueTypeInt},
	})

	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{
		CacheTypeK: "q8_0",
		CacheTypeV: "q8_0",
	})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}
	if !containsSeq(flags, "-ctk", "q8_0") {
		t.Errorf("expected [-ctk q8_0] in flags, got: %v", flags)
	}
	if !containsSeq(flags, "-ctv", "q8_0") {
		t.Errorf("expected [-ctv q8_0] in flags, got: %v", flags)
	}
}

// ─── BuildArgs — speculative decoding ─────────────────────────────────────────

func TestBuildArgs_SpecDraft(t *testing.T) {
	// Real llama.cpp flags: --model-draft (canonical), --spec-draft-model does NOT exist.
	// See: https://github.com/ggml-org/llama.cpp/releases (spec decoding launched ~Nov 2024)
	caps := engine.BuildCapabilities("llama-server", "b4000", map[string]engine.FlagSpec{
		"--spec-type":     {Name: "--spec-type", TakesValue: true, ValueType: engine.ValueTypeString},
		"--model-draft":   {Name: "--model-draft", TakesValue: true, ValueType: engine.ValueTypeString},
		"--spec-draft-n-max": {Name: "--spec-draft-n-max", TakesValue: true, ValueType: engine.ValueTypeInt},
		"--spec-draft-p-min": {Name: "--spec-draft-p-min", TakesValue: true, ValueType: engine.ValueTypeFloat},
		"-c":             {Name: "-c", TakesValue: true, ValueType: engine.ValueTypeInt},
	})

	p := 0.75
	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{
		SpecType:       "draft",
		SpecDraftModel: "/models/draft.gguf",
		SpecDraftNMax:  5,
		SpecDraftPMin:  p,
	})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}

	checks := [][2]string{
		{"--spec-type", "draft"},
		{"--model-draft", "/models/draft.gguf"}, // real flag name
		{"--spec-draft-n-max", "5"},
	}
	for _, pair := range checks {
		if !containsSeq(flags, pair[0], pair[1]) {
			t.Errorf("expected [%s %s] in flags, got: %v", pair[0], pair[1], flags)
		}
	}
	// p-min should appear as a float with 2 decimal places
	if !containsSeq(flags, "--spec-draft-p-min", "0.75") {
		t.Errorf("expected [--spec-draft-p-min 0.75] in flags, got: %v", flags)
	}
}

// ─── BuildArgs — MoE CPU offload ──────────────────────────────────────────────

func TestBuildArgs_NCPUMoe(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b4000", map[string]engine.FlagSpec{
		"--n-cpu-moe": {Name: "--n-cpu-moe", TakesValue: true, ValueType: engine.ValueTypeInt},
		"-c":          {Name: "-c", TakesValue: true, ValueType: engine.ValueTypeInt},
	})

	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{NCPUMoE: 99})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}
	if !containsSeq(flags, "--n-cpu-moe", "99") {
		t.Errorf("expected [--n-cpu-moe 99] in flags, got: %v", flags)
	}
}

// ─── BuildArgs — --no-mmap ────────────────────────────────────────────────────

func TestBuildArgs_MMapDisabled(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b4000", map[string]engine.FlagSpec{
		"--no-mmap": {Name: "--no-mmap", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
		"-c":        {Name: "-c", TakesValue: true, ValueType: engine.ValueTypeInt},
	})

	noMMap := false
	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{MMap: &noMMap})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}
	if !contains(flags, "--no-mmap") {
		t.Errorf("expected --no-mmap in flags when MMap=false, got: %v", flags)
	}
}

func TestBuildArgs_MMapEnabled_NoFlag(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b4000", map[string]engine.FlagSpec{
		"--no-mmap": {Name: "--no-mmap", TakesValue: false, ValueType: engine.ValueTypeBoolImplicit},
	})

	yesMap := true
	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{MMap: &yesMap})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}
	if contains(flags, "--no-mmap") {
		t.Errorf("expected --no-mmap NOT in flags when MMap=true, got: %v", flags)
	}
}

// ─── BuildArgs — extra_args ───────────────────────────────────────────────────

func TestBuildArgs_ExtraArgsAppendedLast(t *testing.T) {
	caps := engine.BuildCapabilities("llama-server", "b4000", map[string]engine.FlagSpec{
		"-c": {Name: "-c", TakesValue: true, ValueType: engine.ValueTypeInt},
	})

	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{
		CtxSize:   4096,
		ExtraArgs: []string{"--no-warmup", "--verbose"},
	})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}
	// extra_args must be last
	n := len(flags)
	if n < 4 {
		t.Fatalf("expected at least 4 tokens, got %d: %v", n, flags)
	}
	if flags[n-2] != "--no-warmup" || flags[n-1] != "--verbose" {
		t.Errorf("extra_args should be last in flags, got: %v", flags)
	}
}

// ─── BuildArgs — zero values omitted ─────────────────────────────────────────

func TestBuildArgs_ZeroValuesProduceNoFlags(t *testing.T) {
	caps := capsFromFixture(t)
	eng := &Engine{}
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{})
	if err != nil {
		t.Fatalf("BuildArgs returned error: %v", err)
	}
	if len(flags) != 0 {
		t.Errorf("zero-value EngineConfig should produce no flags, got: %v", flags)
	}
}

// ─── BuildArgs — full fixture round-trip ─────────────────────────────────────

func TestBuildArgs_FullConfig_NoError(t *testing.T) {
	caps := capsFromFixture(t)
	noMMap := false
	eng := &Engine{}
	_, err := eng.BuildArgs(caps, recipe.EngineConfig{
		CtxSize:        4096,
		GPULayers:      99,
		FlashAttn:      true,
		BatchSize:      2048,
		UBatchSize:     512,
		CacheTypeK:     "q8_0",
		CacheTypeV:     "q8_0",
		SpecType:       "draft",
		SpecDraftModel: "/tmp/draft.gguf",
		SpecDraftNMax:  5,
		SpecDraftPMin:  0.9,
		NCPUMoE:        2,
		MLock:          true,
		MMap:           &noMMap,
		Jinja:          true,
		NParallel:      4,
		Port:           8080,
	})
	if err != nil {
		t.Errorf("BuildArgs with full EngineConfig returned error: %v", err)
	}
}

// ─── Capabilities caching ─────────────────────────────────────────────────────

// TestCapabilities_ReturnsErrorWhenNoBinary verifies a clean error when
// llama-server is not in PATH. We simulate this by temporarily clearing PATH.
func TestCapabilities_ReturnsErrorWhenNoBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH manipulation not reliable on Windows")
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "/nonexistent_bin_dir")
	defer func() { _ = os.Setenv("PATH", origPath) }()

	eng := &Engine{}
	_, err := eng.Capabilities(context.Background())
	if err == nil {
		t.Fatal("expected error when llama-server not in PATH, got nil")
	}
	if !strings.Contains(err.Error(), "llama-server") {
		t.Errorf("error should mention llama-server, got: %v", err)
	}
}

// ─── Log parser ───────────────────────────────────────────────────────────────

func TestLlamaCppLogParser_GenTokens(t *testing.T) {
	p := &llamaCppLogParser{}
	line := "llama_print_timings:        eval time =    1234.56 ms /    10 runs   (   123.46 ms per token,     8.10 tokens per second)"
	m := p.ParseLine(line)
	if m.TokensPerSecGen == 0 {
		t.Errorf("expected non-zero TokensPerSecGen from line: %s", line)
	}
}

func TestLlamaCppLogParser_PromptTokens(t *testing.T) {
	p := &llamaCppLogParser{}
	line := "llama_print_timings: prompt eval time =    500.00 ms /   256 tokens (    1.95 ms per token,   512.00 tokens per second)"
	m := p.ParseLine(line)
	if m.TokensPerSecPrefill == 0 {
		t.Errorf("expected non-zero TokensPerSecPrefill from line: %s", line)
	}
}

func TestLlamaCppLogParser_VRAM_MB(t *testing.T) {
	p := &llamaCppLogParser{}
	line := "VRAM USED = 4096 MB"
	m := p.ParseLine(line)
	if m.PeakVRAMMB != 4096 {
		t.Errorf("expected PeakVRAMMB=4096, got %d", m.PeakVRAMMB)
	}
}

func TestLlamaCppLogParser_VRAM_GB(t *testing.T) {
	p := &llamaCppLogParser{}
	line := "VRAM USED: 8.0 GB"
	m := p.ParseLine(line)
	if m.PeakVRAMMB != 8192 {
		t.Errorf("expected PeakVRAMMB=8192 (8 GB → MB), got %d", m.PeakVRAMMB)
	}
}

func TestLlamaCppLogParser_NoMatchLine(t *testing.T) {
	p := &llamaCppLogParser{}
	m := p.ParseLine("some unrelated log line about tokenization")
	if m.TokensPerSecGen != 0 || m.TokensPerSecPrefill != 0 || m.PeakVRAMMB != 0 {
		t.Errorf("expected zero metrics for non-matching line, got: %+v", m)
	}
}

// ─── Engine interface compliance ─────────────────────────────────────────────

// TestEngineImplementsInterface is a compile-time assertion that Engine
// satisfies the engine.Engine interface. If it does not, this will not compile.
func TestEngineImplementsInterface(t *testing.T) {
	var _ engine.Engine = (*Engine)(nil)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// contains returns true if slice contains target.
func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

// containsSeq returns true if a and b appear consecutively (in that order) in slice.
func containsSeq(slice []string, a, b string) bool {
	for i := 0; i+1 < len(slice); i++ {
		if slice[i] == a && slice[i+1] == b {
			return true
		}
	}
	return false
}

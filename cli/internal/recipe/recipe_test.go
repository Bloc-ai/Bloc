package recipe

import (
	"strings"
	"testing"
)

// ─── BuildFlags (llama.cpp) tests ─────────────────────────────────────────────

func TestBuildFlags_Basic(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			CtxSize:   4096,
			GPULayers: 99,
			Port:      8080,
		},
	}
	flags := r.BuildFlags()

	contains := func(fs []string, flag, val string) bool {
		for i, f := range fs {
			if f == flag && i+1 < len(fs) && fs[i+1] == val {
				return true
			}
		}
		return false
	}

	if !contains(flags, "-c", "4096") {
		t.Error("expected -c 4096")
	}
	if !contains(flags, "-ngl", "99") {
		t.Error("expected -ngl 99")
	}
	if !contains(flags, "--port", "8080") {
		t.Error("expected --port 8080")
	}
}

func TestBuildFlags_ZeroValues_Omitted(t *testing.T) {
	r := &Recipe{Schema: "bloc/v1"}
	flags := r.BuildFlags()
	if len(flags) != 0 {
		t.Errorf("expected no flags for zero-value config, got %v", flags)
	}
}

func TestBuildFlags_FlashAttn_Bool(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			FlashAttn:  true,
			BatchSize:  512,
			UBatchSize: 512,
		},
	}
	flags := r.BuildFlags()

	// FIX: --flash-attn now requires a value (on|off|auto) — must NOT emit bare -fa.
	// Also verify the value token "on" immediately follows the flag (not -b).
	for i, f := range flags {
		if f == "-fa" {
			t.Error("must not emit bare -fa — llama.cpp now requires --flash-attn [on|off|auto]")
		}
		if f == "--flash-attn" {
			if i+1 >= len(flags) || flags[i+1] != "on" {
				t.Errorf("--flash-attn must be immediately followed by \"on\", got: %v", flags[i+1:])
			}
		}
	}

	hasFlashAttnOn := false
	for i, f := range flags {
		if f == "--flash-attn" && i+1 < len(flags) && flags[i+1] == "on" {
			hasFlashAttnOn = true
		}
	}
	if !hasFlashAttnOn {
		t.Error("expected --flash-attn on in flags for FlashAttn=true")
	}
}

func TestBuildFlags_MLock_Bool(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			MLock: true,
		},
	}
	flags := r.BuildFlags()
	found := false
	for _, f := range flags {
		if f == "--mlock" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --mlock for MLock=true")
	}
}

func TestBuildFlags_NoMmap(t *testing.T) {
	f := false
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			MMap: &f,
		},
	}
	flags := r.BuildFlags()
	found := false
	for _, f := range flags {
		if f == "--no-mmap" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --no-mmap when MMap is explicitly false")
	}
}

func TestBuildFlags_Mmap_TrueNotEmitted(t *testing.T) {
	tr := true
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			MMap: &tr,
		},
	}
	flags := r.BuildFlags()
	for _, f := range flags {
		if f == "--no-mmap" {
			t.Error("--no-mmap must not appear when MMap is true")
		}
	}
}

func TestBuildFlags_ExtraArgs_Appended(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			ExtraArgs: []string{"--verbose", "--jinja"},
		},
	}
	flags := r.BuildFlags()
	foundV, foundJ := false, false
	for _, f := range flags {
		if f == "--verbose" {
			foundV = true
		}
		if f == "--jinja" {
			foundJ = true
		}
	}
	if !foundV {
		t.Error("expected --verbose in flags from extra_args")
	}
	if !foundJ {
		t.Error("expected --jinja in flags from extra_args")
	}
}

func TestBuildFlags_SpecDraftModel(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SpecType:      "mtp",
			SpecDraftModel: "/path/to/draft.gguf",
			SpecDraftNMax: 5,
		},
	}
	flags := r.BuildFlags()

	has := func(flag, val string) bool {
		for i, f := range flags {
			if f == flag && i+1 < len(flags) && flags[i+1] == val {
				return true
			}
		}
		return false
	}
	if !has("--spec-type", "mtp") {
		t.Error("expected --spec-type mtp")
	}
	if !has("--model-draft", "/path/to/draft.gguf") {
		t.Error("expected --model-draft /path/to/draft.gguf")
	}
	if !has("--draft", "5") {
		t.Error("expected --draft 5")
	}
}

// ─── RequiredFlags tests ──────────────────────────────────────────────────────

func TestRequiredFlags_Empty(t *testing.T) {
	r := &Recipe{Schema: "bloc/v1"}
	req := r.RequiredFlags()
	if len(req) != 0 {
		t.Errorf("expected empty required flags for bare recipe, got %v", req)
	}
}

func TestRequiredFlags_NCPUMoE(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{NCPUMoE: 4},
	}
	req := r.RequiredFlags()
	if _, ok := req["--n-cpu-moe"]; !ok {
		t.Error("expected --n-cpu-moe in required flags when NCPUMoE != 0")
	}
}

func TestRequiredFlags_FlashAttn(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{FlashAttn: true},
	}
	req := r.RequiredFlags()
	// FIX: probe now checks for the long form --flash-attn, not -fa
	if _, ok := req["--flash-attn"]; !ok {
		t.Error("expected --flash-attn in required flags when FlashAttn=true")
	}
	if _, ok := req["-fa"]; ok {
		t.Error("must not add bare -fa to required flags — new llama.cpp requires value form")
	}
}

func TestRequiredFlags_SpecType(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{SpecType: "mtp", SpecDraftModel: "draft.gguf", SpecDraftNMax: 3},
	}
	req := r.RequiredFlags()
	if _, ok := req["--spec-type"]; !ok {
		t.Error("expected --spec-type in required flags")
	}
	if _, ok := req["--model-draft"]; !ok {
		t.Error("expected --model-draft in required flags")
	}
	if _, ok := req["--draft"]; !ok {
		t.Error("expected --draft in required flags")
	}
}

func TestRequiredFlags_CacheType(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{CacheTypeK: "q8_0", CacheTypeV: "q4_0"},
	}
	req := r.RequiredFlags()
	if _, ok := req["-ctk"]; !ok {
		t.Error("expected -ctk in required flags")
	}
	if _, ok := req["-ctv"]; !ok {
		t.Error("expected -ctv in required flags")
	}
}

func TestRequiredFlags_Jinja(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{Jinja: true},
	}
	req := r.RequiredFlags()
	if _, ok := req["--jinja"]; !ok {
		t.Error("expected --jinja in required flags when Jinja=true")
	}
}

func TestRequiredFlags_ExtraArgs_LongFlags(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			ExtraArgs: []string{"--embedding", "value", "--verbose"},
		},
	}
	req := r.RequiredFlags()
	if _, ok := req["--embedding"]; !ok {
		t.Error("expected --embedding in required flags from extra_args")
	}
	if _, ok := req["--verbose"]; !ok {
		t.Error("expected --verbose in required flags from extra_args")
	}
}

func TestRequiredFlags_ExtraArgs_ShortFlags(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			ExtraArgs: []string{"-fa", "-v"},
		},
	}
	req := r.RequiredFlags()
	if _, ok := req["-fa"]; !ok {
		t.Error("expected -fa in required flags from extra_args")
	}
	if _, ok := req["-v"]; !ok {
		t.Error("expected -v in required flags from extra_args")
	}
}

// ─── validateEngineConfig cross-engine tests ──────────────────────────────────

func TestValidateEngineConfig_LlamaCpp_RejectsTensorParallelSize(t *testing.T) {
	r := &Recipe{
		Schema:   "bloc/v1",
		Metadata: Metadata{Name: "test"},
		Model:    Model{DownloadURL: "https://example.com/model.gguf"},
		Engine:   Engine{Name: "llama.cpp"},
		EngineConfig: EngineConfig{
			TensorParallelSize: 2,
		},
	}
	_, err := Parse(marshalRecipe(t, r))
	if err == nil {
		t.Error("expected error when tensor_parallel_size is set on a llama.cpp recipe")
	}
	if !strings.Contains(err.Error(), "tensor_parallel_size") {
		t.Errorf("error should mention tensor_parallel_size, got: %v", err)
	}
}

func TestValidateEngineConfig_LlamaCpp_RejectsGPUMemUtilization(t *testing.T) {
	r := &Recipe{
		Schema:   "bloc/v1",
		Metadata: Metadata{Name: "test"},
		Model:    Model{DownloadURL: "https://example.com/model.gguf"},
		Engine:   Engine{Name: "llama.cpp"},
		EngineConfig: EngineConfig{
			GPUMemoryUtilization: 0.90,
		},
	}
	_, err := Parse(marshalRecipe(t, r))
	if err == nil {
		t.Error("expected error when gpu_memory_utilization is set on a llama.cpp recipe")
	}
}

func TestValidateEngineConfig_VLLM_RejectsGPULayers(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-vllm
model:
  hf_repo: meta-llama/Llama-3-8B
engine:
  name: vllm
engine_config:
  gpu_layers: 40
  tensor_parallel_size: 1
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected error when gpu_layers (-ngl) is set on a vLLM recipe")
	}
	if !strings.Contains(err.Error(), "gpu_layers") {
		t.Errorf("error should mention gpu_layers, got: %v", err)
	}
}

func TestValidateEngineConfig_VLLM_RejectsBatchSize(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-vllm
model:
  hf_repo: meta-llama/Llama-3-8B
engine:
  name: vllm
engine_config:
  batch_size: 512
  tensor_parallel_size: 1
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected error when batch_size (-b) is set on a vLLM recipe")
	}
}

func TestValidateEngineConfig_VLLM_RejectsNCPUMoE(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-vllm
model:
  hf_repo: meta-llama/Llama-3-8B
engine:
  name: vllm
engine_config:
  n_cpu_moe: 8
  tensor_parallel_size: 1
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected error when n_cpu_moe is set on a vLLM recipe")
	}
}

// ─── validateExtraArgs allowlist tests ────────────────────────────────────────

func TestValidateExtraArgs_AllowedFlag(t *testing.T) {
	err := validateExtraArgs([]string{"--verbose"})
	if err != nil {
		t.Errorf("--verbose should be allowed, got error: %v", err)
	}
}

func TestValidateExtraArgs_DeniedFlag(t *testing.T) {
	err := validateExtraArgs([]string{"--api-key", "secret"})
	if err == nil {
		t.Error("expected error for disallowed --api-key flag")
	}
	if !strings.Contains(err.Error(), "--api-key") {
		t.Errorf("error should mention the denied flag, got: %v", err)
	}
}

func TestValidateExtraArgs_MixedAllowedDenied(t *testing.T) {
	err := validateExtraArgs([]string{"--verbose", "--host", "0.0.0.0"})
	if err == nil {
		t.Error("expected error when --host appears in extra_args")
	}
}

func TestValidateExtraArgs_ValueTokensAllowed(t *testing.T) {
	// Non-flag tokens (values like "q8_0", "512") must pass through without error
	err := validateExtraArgs([]string{"-ctk", "q8_0", "-ctv", "q4_0"})
	if err != nil {
		t.Errorf("value tokens should pass through, got error: %v", err)
	}
}

func TestValidateExtraArgs_Empty(t *testing.T) {
	err := validateExtraArgs(nil)
	if err != nil {
		t.Errorf("nil extra_args should pass, got: %v", err)
	}
}

// ─── HFRepo / Semver validation tests ────────────────────────────────────────

func TestParseRecipe_ValidHFRepo(t *testing.T) {
	validRepos := []string{
		"meta-llama/Meta-Llama-3-8B-Instruct",
		"google/gemma-2-9b-it",
		"Qwen/Qwen3-30B-MoE",
		"microsoft/phi-3-mini-4k-instruct",
	}
	for _, repo := range validRepos {
		input := []byte("schema: bloc/v1\nmetadata:\n  name: test\nmodel:\n  hf_repo: " + repo + "\nengine:\n  name: vllm\n")
		if _, err := Parse(input); err != nil {
			t.Errorf("valid hf_repo %q was rejected: %v", repo, err)
		}
	}
}

func TestParseRecipe_InvalidHFRepo_PathTraversal(t *testing.T) {
	input := []byte("schema: bloc/v1\nmetadata:\n  name: test\nmodel:\n  hf_repo: ../../etc/passwd\nengine:\n  name: vllm\n")
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for path traversal in hf_repo")
	}
}

func TestParseRecipe_InvalidHFRepo_ShellChars(t *testing.T) {
	input := []byte("schema: bloc/v1\nmetadata:\n  name: test\nmodel:\n  hf_repo: org/model;rm -rf /\nengine:\n  name: vllm\n")
	_, err := Parse(input)
	if err == nil {
		t.Error("expected error for shell chars in hf_repo")
	}
}

func TestParseRecipe_ValidEngineVersion(t *testing.T) {
	validVersions := []string{"0.9.0", "1.2.3", "0.8.5-rc1", "1.0.0+build1"}
	for _, ver := range validVersions {
		input := []byte("schema: bloc/v1\nmetadata:\n  name: test\nmodel:\n  hf_repo: org/model\nengine:\n  name: vllm\n  version: " + ver + "\n")
		if _, err := Parse(input); err != nil {
			t.Errorf("valid engine.version %q was rejected: %v", ver, err)
		}
	}
}

func TestParseRecipe_InvalidEngineVersion(t *testing.T) {
	invalidVersions := []string{"latest", "v0.9.0", "$(whoami)", "1"}
	for _, ver := range invalidVersions {
		input := []byte("schema: bloc/v1\nmetadata:\n  name: test\nmodel:\n  hf_repo: org/model\nengine:\n  name: vllm\n  version: " + ver + "\n")
		if _, err := Parse(input); err == nil {
			t.Errorf("invalid engine.version %q was accepted — should be rejected", ver)
		}
	}
}

// ─── Schema validation tests ──────────────────────────────────────────────────

func TestParse_UnsupportedSchema(t *testing.T) {
	yaml := []byte(`
schema: bloc/v2
metadata:
  name: test
model:
  hf_repo: org/model
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected error for unsupported schema 'bloc/v2'")
	}
	if !strings.Contains(err.Error(), "bloc/v2") {
		t.Errorf("error should mention the bad schema, got: %v", err)
	}
}

func TestParse_MissingMetadataName(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
model:
  hf_repo: org/model
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected error when metadata.name is missing")
	}
}

// ─── Helper: marshal a Recipe to minimal YAML for round-trip testing ──────────

func marshalRecipe(t *testing.T, r *Recipe) []byte {
	t.Helper()
	// Build a minimal YAML manually to avoid importing gopkg.in/yaml.v3 in tests.
	// We only use this for a few cross-engine validation round-trip tests.
	var sb strings.Builder
	sb.WriteString("schema: " + r.Schema + "\n")
	sb.WriteString("metadata:\n  name: " + r.Metadata.Name + "\n")
	if r.Model.DownloadURL != "" {
		sb.WriteString("model:\n  download_url: " + r.Model.DownloadURL + "\n")
	} else if r.Model.HFRepo != "" {
		sb.WriteString("model:\n  hf_repo: " + r.Model.HFRepo + "\n")
	}
	if r.Engine.Name != "" {
		sb.WriteString("engine:\n  name: " + r.Engine.Name + "\n")
	}
	cfg := r.EngineConfig
	sb.WriteString("engine_config:\n")
	if cfg.TensorParallelSize != 0 {
		sb.WriteString("  tensor_parallel_size: 2\n")
	}
	if cfg.GPUMemoryUtilization != 0 {
		sb.WriteString("  gpu_memory_utilization: 0.90\n")
	}
	if cfg.GPULayers != 0 {
		sb.WriteString("  gpu_layers: 40\n")
	}
	if cfg.BatchSize != 0 {
		sb.WriteString("  batch_size: 512\n")
	}
	if cfg.NCPUMoE != 0 {
		sb.WriteString("  n_cpu_moe: 8\n")
	}
	return []byte(sb.String())
}

func TestParseLocal_BypassAllowlist(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: local-bypass-test
model:
  download_url: https://huggingface.co/org/model/resolve/main/model.gguf
engine:
  name: llama.cpp
engine_config:
  extra_args:
    - "--log-file"
    - "/etc/shadow"
    - "--api-key"
    - "secret"
`)

	// 1. Standard Parse should fail because --log-file and --api-key are strictly banned
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected Parse to fail under strict registry rules for banned flags")
	}

	// 2. ParseLocal should succeed and bypass deny-list checks
	r, err := ParseLocal(yaml)
	if err != nil {
		t.Errorf("expected ParseLocal to succeed by bypassing strict allowlist checks, got: %v", err)
	}

	if r.Metadata.Name != "local-bypass-test" {
		t.Errorf("expected name 'local-bypass-test', got '%s'", r.Metadata.Name)
	}

	foundLogFile := false
	foundApiKey := false
	for _, arg := range r.EngineConfig.ExtraArgs {
		if arg == "--log-file" {
			foundLogFile = true
		}
		if arg == "--api-key" {
			foundApiKey = true
		}
	}
	if !foundLogFile || !foundApiKey {
		t.Error("expected extra_args to successfully preserve all bypassed flags")
	}
}

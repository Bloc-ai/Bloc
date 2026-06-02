package recipe

import (
	"fmt"
	"testing"
)

// ─── BuildVLLMFlags tests ─────────────────────────────────────────────────────

func TestBuildVLLMFlags_Basic(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			Port:                 8000,
			TensorParallelSize:   2,
			GPUMemoryUtilization: 0.90,
			MaxModelLen:          131072,
			DType:                "bfloat16",
		},
	}
	flags := r.BuildVLLMFlags()
	contains := func(flags []string, flag, val string) bool {
		for i, f := range flags {
			if f == flag {
				if val == "" {
					return true
				}
				if i+1 < len(flags) && flags[i+1] == val {
					return true
				}
			}
		}
		return false
	}
	if !contains(flags, "--port", "8000") {
		t.Error("expected --port 8000")
	}
	if !contains(flags, "--tensor-parallel-size", "2") {
		t.Error("expected --tensor-parallel-size 2")
	}
	if !contains(flags, "--gpu-memory-utilization", "0.90") {
		t.Error("expected --gpu-memory-utilization 0.90")
	}
	if !contains(flags, "--max-model-len", "131072") {
		t.Error("expected --max-model-len 131072")
	}
	if !contains(flags, "--dtype", "bfloat16") {
		t.Error("expected --dtype bfloat16")
	}
}

func TestBuildVLLMFlags_SpeculativeDecoding(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SpeculativeModel:     "Hikari07jp/Step-3.7-Flash-MTP-draft",
			NumSpeculativeTokens: 5,
		},
	}
	flags := r.BuildVLLMFlags()

	contains := func(flags []string, flag, val string) bool {
		for i, f := range flags {
			if f == flag && i+1 < len(flags) && flags[i+1] == val {
				return true
			}
		}
		return false
	}

	if !contains(flags, "--speculative-model", "Hikari07jp/Step-3.7-Flash-MTP-draft") {
		t.Error("expected --speculative-model to be set")
	}
	if !contains(flags, "--num-speculative-tokens", "5") {
		t.Error("expected --num-speculative-tokens 5")
	}
}

func TestBuildVLLMFlags_TrustRemoteCodeNotInjected(t *testing.T) {
	// F-19: BuildVLLMFlags must NOT inject --trust-remote-code.
	// It is only added by run.go AFTER explicit user confirmation.
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			TrustRemoteCode: true,
		},
	}
	flags := r.BuildVLLMFlags()
	for _, f := range flags {
		if f == "--trust-remote-code" {
			t.Error("BuildVLLMFlags must NOT inject --trust-remote-code — F-19 gate must do it in run.go")
		}
	}
}

func TestBuildVLLMFlags_ZeroValues_Omitted(t *testing.T) {
	// All zero-value fields should produce an empty flag slice
	r := &Recipe{Schema: "bloc/v1"}
	flags := r.BuildVLLMFlags()
	if len(flags) != 0 {
		t.Errorf("expected empty flags for all-zero config, got %v", flags)
	}
}

func TestBuildVLLMFlags_EnableExpertParallel(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			EnableExpertParallel: true,
		},
	}
	flags := r.BuildVLLMFlags()
	found := false
	for _, f := range flags {
		if f == "--enable-expert-parallel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --enable-expert-parallel in flags")
	}
}

// ─── Parse vLLM recipe tests ──────────────────────────────────────────────────

func TestParseVLLMRecipe_Valid(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: llama3-8b-vllm
model:
  source: huggingface
  hf_repo: meta-llama/Meta-Llama-3-8B-Instruct
  size_gb: 16
engine:
  name: vllm
  runtime: native
  version: "0.9.0"
hardware:
  min_vram: 16GB
engine_config:
  port: 8000
  gpu_memory_utilization: 0.90
  dtype: bfloat16
  tensor_parallel_size: 1
`)
	r, err := Parse(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if r.Engine.Name != "vllm" {
		t.Errorf("Engine.Name = %q, want vllm", r.Engine.Name)
	}
	if r.Engine.Version != "0.9.0" {
		t.Errorf("Engine.Version = %q, want 0.9.0", r.Engine.Version)
	}
	if r.Model.HFRepo != "meta-llama/Meta-Llama-3-8B-Instruct" {
		t.Errorf("Model.HFRepo = %q, want meta-llama/Meta-Llama-3-8B-Instruct", r.Model.HFRepo)
	}
}

func TestTrustRemoteCodeBlockedInExtraArgs(t *testing.T) {
	// F-19: --trust-remote-code in extra_args must be rejected at parse time.
	// It is only permitted as the first-class trust_remote_code: true field.
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: unsafe-recipe
model:
  hf_repo: some/model
engine:
  name: vllm
engine_config:
  tensor_parallel_size: 1
  extra_args:
    - "--trust-remote-code"
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected parse error when --trust-remote-code is in extra_args")
	}
}

func TestParseVLLMRecipe_MissingBothDownloadSources(t *testing.T) {
	// Must fail if neither download_url nor hf_repo is provided
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-recipe
engine:
  name: vllm
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected parse error when both download_url and hf_repo are missing")
	}
}

// ─── F-15: Docker image tag validation ───────────────────────────────────────

func TestParseDockerRecipe_ValidImageTags(t *testing.T) {
	validImages := []string{
		"vllm/vllm-openai:v0.9.0",
		"vllm/vllm-openai:latest",
		"nvcr.io/nvidia/cuda:12.0-base",
		"vllm/vllm-openai:v0.9.0.post1",
		"a",
	}
	for _, img := range validImages {
		input := []byte("schema: bloc/v1\nmetadata:\n  name: docker-recipe\nmodel:\n  hf_repo: some/model\nengine:\n  name: vllm\n  runtime: docker\n  image: " + img + "\n")
		if _, err := Parse(input); err != nil {
			t.Errorf("F-15: valid image %q rejected: %v", img, err)
		}
	}
}

func TestParseDockerRecipe_InvalidImageTags(t *testing.T) {
	invalidImages := []string{
		"UPPER/case:tag",
		"image with spaces",
		"../../../etc/passwd",
	}
	for _, img := range invalidImages {
		input := []byte("schema: bloc/v1\nmetadata:\n  name: docker-recipe\nmodel:\n  hf_repo: some/model\nengine:\n  name: vllm\n  runtime: docker\n  image: " + img + "\n")
		if _, err := Parse(input); err == nil {
			t.Errorf("F-15: invalid image %q was accepted — should be rejected", img)
		}
	}
}

// ─── F-17: Port range validation ─────────────────────────────────────────────

func TestParseRecipe_ValidPorts(t *testing.T) {
	validPorts := []int{1024, 8000, 8080, 11434, 65535}
	for _, port := range validPorts {
		input := []byte(fmt.Sprintf("schema: bloc/v1\nmetadata:\n  name: p\nmodel:\n  hf_repo: a/b\nengine:\n  name: vllm\nengine_config:\n  port: %d\n  tensor_parallel_size: 1\n", port))
		if _, err := Parse(input); err != nil {
			t.Errorf("F-17: valid port %d rejected: %v", port, err)
		}
	}
}

func TestParseRecipe_InvalidPorts(t *testing.T) {
	invalidPorts := []int{1, 80, 443, 1023, 65536, 99999}
	for _, port := range invalidPorts {
		input := []byte(fmt.Sprintf("schema: bloc/v1\nmetadata:\n  name: p\nmodel:\n  hf_repo: a/b\nengine:\n  name: vllm\nengine_config:\n  port: %d\n  tensor_parallel_size: 1\n", port))
		if _, err := Parse(input); err == nil {
			t.Errorf("F-17: invalid port %d accepted — should be rejected", port)
		}
	}
}

func TestParseRecipe_ZeroPort_Allowed(t *testing.T) {
	input := []byte("schema: bloc/v1\nmetadata:\n  name: p\nmodel:\n  hf_repo: a/b\nengine:\n  name: vllm\nengine_config:\n  tensor_parallel_size: 1\n")
	if _, err := Parse(input); err != nil {
		t.Errorf("F-17: recipe with no port (zero value) should be accepted: %v", err)
	}
}

package recipe

import (
	"testing"
)

// ─── BuildSGLangFlags tests ───────────────────────────────────────────────────

func TestBuildSGLangFlags_Basic(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SGLangTensorParallelSize: 4,
			SGLangContextLength:      262144,
			SGLangMemFractionStatic:  0.94,
		},
	}
	flags := r.BuildSGLangFlags()
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
	if !contains(flags, "--tp-size", "4") {
		t.Error("expected --tp-size 4")
	}
	if !contains(flags, "--context-length", "262144") {
		t.Error("expected --context-length 262144")
	}
	if !contains(flags, "--mem-fraction-static", "0.94") {
		t.Error("expected --mem-fraction-static 0.94")
	}
}

func TestBuildSGLangFlags_Parsers(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SGLangReasoningParser: "kimi_k2",
			SGLangToolCallParser:  "kimi_k2",
		},
	}
	flags := r.BuildSGLangFlags()
	contains := func(flags []string, flag, val string) bool {
		for i, f := range flags {
			if f == flag && i+1 < len(flags) && flags[i+1] == val {
				return true
			}
		}
		return false
	}
	if !contains(flags, "--reasoning-parser", "kimi_k2") {
		t.Error("expected --reasoning-parser kimi_k2")
	}
	if !contains(flags, "--tool-call-parser", "kimi_k2") {
		t.Error("expected --tool-call-parser kimi_k2")
	}
}

func TestBuildSGLangFlags_EnableMultimodal(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SGLangEnableMultimodal: true,
		},
	}
	flags := r.BuildSGLangFlags()
	found := false
	for _, f := range flags {
		if f == "--enable-multimodal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected --enable-multimodal flag when SGLangEnableMultimodal=true")
	}
}

func TestBuildSGLangFlags_EnableMultimodal_False(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SGLangEnableMultimodal: false,
		},
	}
	flags := r.BuildSGLangFlags()
	for _, f := range flags {
		if f == "--enable-multimodal" {
			t.Error("--enable-multimodal must not be emitted when SGLangEnableMultimodal=false")
		}
	}
}

func TestBuildSGLangFlags_Quantization(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SGLangQuantization: "modelopt_fp4",
			SGLangKVCacheDType: "fp8_e4m3",
		},
	}
	flags := r.BuildSGLangFlags()
	contains := func(flags []string, flag, val string) bool {
		for i, f := range flags {
			if f == flag && i+1 < len(flags) && flags[i+1] == val {
				return true
			}
		}
		return false
	}
	if !contains(flags, "--quantization", "modelopt_fp4") {
		t.Error("expected --quantization modelopt_fp4")
	}
	if !contains(flags, "--kv-cache-dtype", "fp8_e4m3") {
		t.Error("expected --kv-cache-dtype fp8_e4m3")
	}
}

func TestBuildSGLangFlags_ConcurrencyAndPrefill(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SGLangMaxRunningRequests: 4,
			SGLangChunkedPrefillSize: 8192,
			SGLangMaxPrefillTokens:   16384,
			SGLangCudaGraphMaxBS:     4,
		},
	}
	flags := r.BuildSGLangFlags()
	contains := func(flags []string, flag, val string) bool {
		for i, f := range flags {
			if f == flag && i+1 < len(flags) && flags[i+1] == val {
				return true
			}
		}
		return false
	}
	if !contains(flags, "--max-running-requests", "4") {
		t.Error("expected --max-running-requests 4")
	}
	if !contains(flags, "--chunked-prefill-size", "8192") {
		t.Error("expected --chunked-prefill-size 8192")
	}
	if !contains(flags, "--max-prefill-tokens", "16384") {
		t.Error("expected --max-prefill-tokens 16384")
	}
	if !contains(flags, "--cuda-graph-max-bs", "4") {
		t.Error("expected --cuda-graph-max-bs 4")
	}
}

func TestBuildSGLangFlags_ZeroValues_Omitted(t *testing.T) {
	// All zero-value fields should produce an empty flag slice.
	r := &Recipe{Schema: "bloc/v1"}
	flags := r.BuildSGLangFlags()
	if len(flags) != 0 {
		t.Errorf("expected empty flags for all-zero config, got %v", flags)
	}
}

func TestBuildSGLangFlags_ExtraArgs_Passthrough(t *testing.T) {
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			ExtraArgs: []string{"--served-model-name", "kimi-k2.6"},
		},
	}
	flags := r.BuildSGLangFlags()
	found := false
	for i, f := range flags {
		if f == "--served-model-name" && i+1 < len(flags) && flags[i+1] == "kimi-k2.6" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected extra_args to be passed through to flags")
	}
}

func TestBuildSGLangFlags_NoHostOrPort(t *testing.T) {
	// --host and --port must NOT be emitted by BuildSGLangFlags —
	// the runtime injects them. If Port is set it should still not appear.
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			Port: 8000,
		},
	}
	flags := r.BuildSGLangFlags()
	for _, f := range flags {
		if f == "--host" {
			t.Error("BuildSGLangFlags must not emit --host; the runtime injects it")
		}
		if f == "--port" {
			t.Error("BuildSGLangFlags must not emit --port; the runtime injects it")
		}
	}
}

func TestBuildSGLangFlags_CUDADevicesNotEmitted(t *testing.T) {
	// SGLangCUDAVisibleDevices is injected as an env var by the runtime,
	// not as a flag. It must never appear in the flags slice.
	r := &Recipe{
		Schema: "bloc/v1",
		EngineConfig: EngineConfig{
			SGLangCUDAVisibleDevices: "0,2,3,4",
		},
	}
	flags := r.BuildSGLangFlags()
	for _, f := range flags {
		if f == "--cuda-visible-devices" || f == "CUDA_VISIBLE_DEVICES" || f == "0,2,3,4" {
			t.Errorf("SGLangCUDAVisibleDevices must not appear as a flag, got %q in flags %v", f, flags)
		}
	}
}

// ─── Parse SGLang recipe tests ────────────────────────────────────────────────

func TestParseSGLangRecipe_Valid(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: kimi-k2-6-nvfp4
model:
  source: huggingface
  hf_repo: "0xSero/Kimi-K2.6-519B-NVFP4"
  size_gb: 260.0
engine:
  name: sglang
  runtime: docker
  image: "lmsysorg/sglang:v0.5.12.post1"
hardware:
  min_vram: 300GB
engine_config:
  port: 8000
  sglang_tensor_parallel_size: 4
  sglang_context_length: 262144
  sglang_mem_fraction_static: 0.94
  sglang_max_running_requests: 4
  sglang_chunked_prefill_size: 8192
  sglang_max_prefill_tokens: 16384
  sglang_cuda_graph_max_bs: 4
  sglang_quantization: "modelopt_fp4"
  sglang_kv_cache_dtype: "fp8_e4m3"
  sglang_reasoning_parser: "kimi_k2"
  sglang_tool_call_parser: "kimi_k2"
  sglang_enable_multimodal: true
  sglang_cuda_visible_devices: "0,2,3,4"
`)
	r, err := Parse(yaml)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if r.Engine.Name != "sglang" {
		t.Errorf("Engine.Name = %q, want sglang", r.Engine.Name)
	}
	if r.Engine.Runtime != "docker" {
		t.Errorf("Engine.Runtime = %q, want docker", r.Engine.Runtime)
	}
	if r.EngineConfig.SGLangTensorParallelSize != 4 {
		t.Errorf("SGLangTensorParallelSize = %d, want 4", r.EngineConfig.SGLangTensorParallelSize)
	}
	if r.EngineConfig.SGLangContextLength != 262144 {
		t.Errorf("SGLangContextLength = %d, want 262144", r.EngineConfig.SGLangContextLength)
	}
	if r.EngineConfig.SGLangQuantization != "modelopt_fp4" {
		t.Errorf("SGLangQuantization = %q, want modelopt_fp4", r.EngineConfig.SGLangQuantization)
	}
	if r.EngineConfig.SGLangCUDAVisibleDevices != "0,2,3,4" {
		t.Errorf("SGLangCUDAVisibleDevices = %q, want 0,2,3,4", r.EngineConfig.SGLangCUDAVisibleDevices)
	}
	if !r.EngineConfig.SGLangEnableMultimodal {
		t.Error("SGLangEnableMultimodal should be true")
	}
}

func TestParseSGLangRecipe_MissingImage_StillParses(t *testing.T) {
	// Parse should succeed even without engine.image —
	// the missing-image validation happens in Resolve(), not Parse().
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: sglang-no-image
model:
  hf_repo: some/model
engine:
  name: sglang
  runtime: docker
`)
	_, err := Parse(yaml)
	if err != nil {
		t.Errorf("Parse should not reject missing image (Resolve does): %v", err)
	}
}

// ─── Cross-engine validation tests ───────────────────────────────────────────

func TestParseSGLangRecipe_RejectsvLLMTensorParallel(t *testing.T) {
	// engine_config.tensor_parallel_size is vLLM-only;
	// sglang recipes must use sglang_tensor_parallel_size.
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-sglang
model:
  hf_repo: some/model
engine:
  name: sglang
  runtime: docker
  image: "lmsysorg/sglang:latest"
engine_config:
  tensor_parallel_size: 4
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected parse error: tensor_parallel_size is vLLM-only on sglang recipe")
	}
}

func TestParseSGLangRecipe_RejectsvLLMGPUMemUtil(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-sglang
model:
  hf_repo: some/model
engine:
  name: sglang
  runtime: docker
  image: "lmsysorg/sglang:latest"
engine_config:
  gpu_memory_utilization: 0.90
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected parse error: gpu_memory_utilization is vLLM-only on sglang recipe")
	}
}

func TestParseSGLangRecipe_RejectsLlamaCppGPULayers(t *testing.T) {
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-sglang
model:
  hf_repo: some/model
engine:
  name: sglang
  runtime: docker
  image: "lmsysorg/sglang:latest"
engine_config:
  gpu_layers: 35
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected parse error: gpu_layers is llama.cpp-only on sglang recipe")
	}
}

func TestParseLlamaCppRecipe_RejectsSGLangFields(t *testing.T) {
	// sglang_* fields must not be accepted on a llama.cpp recipe.
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-llama
model:
  download_url: https://example.com/model.gguf
  file: model.gguf
engine:
  name: llama.cpp
engine_config:
  sglang_tensor_parallel_size: 4
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected parse error: sglang_* fields must not be accepted on llama.cpp recipe")
	}
}

func TestParseVLLMRecipe_RejectsSGLangFields(t *testing.T) {
	// sglang_* fields must not be accepted on a vllm recipe.
	yaml := []byte(`
schema: bloc/v1
metadata:
  name: bad-vllm
model:
  hf_repo: some/model
engine:
  name: vllm
engine_config:
  sglang_context_length: 131072
`)
	_, err := Parse(yaml)
	if err == nil {
		t.Error("expected parse error: sglang_* fields must not be accepted on vllm recipe")
	}
}

// ─── F-15: Docker image tag validation for sglang ────────────────────────────

func TestParseSGLangDockerRecipe_ValidImageTags(t *testing.T) {
	validImages := []string{
		"lmsysorg/sglang:v0.5.12.post1",
		"lmsysorg/sglang:latest",
		"nvcr.io/nvidia/sglang:0.5.12",
		"a",
	}
	for _, img := range validImages {
		input := []byte("schema: bloc/v1\nmetadata:\n  name: sglang-recipe\nmodel:\n  hf_repo: some/model\nengine:\n  name: sglang\n  runtime: docker\n  image: " + img + "\n")
		if _, err := Parse(input); err != nil {
			t.Errorf("F-15: valid image %q rejected for sglang: %v", img, err)
		}
	}
}

func TestParseSGLangDockerRecipe_InvalidImageTags(t *testing.T) {
	invalidImages := []string{
		"UPPER/case:tag",
		"image with spaces",
		"../../../etc/passwd",
	}
	for _, img := range invalidImages {
		input := []byte("schema: bloc/v1\nmetadata:\n  name: sglang-recipe\nmodel:\n  hf_repo: some/model\nengine:\n  name: sglang\n  runtime: docker\n  image: " + img + "\n")
		if _, err := Parse(input); err == nil {
			t.Errorf("F-15: invalid image %q was accepted for sglang — should be rejected", img)
		}
	}
}

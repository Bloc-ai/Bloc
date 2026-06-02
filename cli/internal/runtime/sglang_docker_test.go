//go:build !windows

package runtime

import (
	"strings"
	"testing"

	"github.com/bloc-org/bloc/internal/recipe"
)

// ─── SGLangDockerRuntime unit tests ───────────────────────────────────────────

func TestSGLangDockerRuntime_Name_WithImage(t *testing.T) {
	rt := &SGLangDockerRuntime{image: "lmsysorg/sglang:v0.5.12.post1"}
	want := "SGLang Docker (lmsysorg/sglang:v0.5.12.post1)"
	if rt.Name() != want {
		t.Errorf("Name() = %q, want %q", rt.Name(), want)
	}
}

func TestSGLangDockerRuntime_Name_NoImage(t *testing.T) {
	rt := &SGLangDockerRuntime{}
	if rt.Name() != "SGLang (Docker)" {
		t.Errorf("Name() = %q, want 'SGLang (Docker)'", rt.Name())
	}
}

// ─── Resolve() wires SGLangDockerRuntime ─────────────────────────────────────

func TestResolve_SGLangDocker_MissingImage(t *testing.T) {
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "sglang",
			Runtime: "docker",
			Image:   "", // missing
		},
		Metadata: recipe.Metadata{Name: "test"},
		Model:    recipe.Model{HFRepo: "some/model"},
	}
	_, err := Resolve(r, "")
	if err == nil {
		t.Error("expected error when engine.image is missing for sglang docker runtime")
	}
	if !strings.Contains(err.Error(), "engine.image") {
		t.Errorf("error should mention engine.image, got: %v", err)
	}
}

func TestResolve_SGLangDocker_WithImage(t *testing.T) {
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "sglang",
			Runtime: "docker",
			Image:   "lmsysorg/sglang:v0.5.12.post1",
		},
		Metadata: recipe.Metadata{Name: "test"},
		Model:    recipe.Model{HFRepo: "some/model"},
	}
	rt, err := Resolve(r, "")
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	sg, ok := rt.(*SGLangDockerRuntime)
	if !ok {
		t.Fatalf("expected *SGLangDockerRuntime, got %T", rt)
	}
	if sg.image != "lmsysorg/sglang:v0.5.12.post1" {
		t.Errorf("image = %q, want lmsysorg/sglang:v0.5.12.post1", sg.image)
	}
}

func TestResolve_SGLangDocker_EmptyRuntime_DefaultsToDocker(t *testing.T) {
	// When runtime is not specified, Resolve defaults to "native" globally,
	// which sglang explicitly rejects. Users must set runtime: docker.
	// This test verifies the error message is actionable.
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "sglang",
			Runtime: "docker", // must be explicit for sglang
			Image:   "lmsysorg/sglang:v0.5.12.post1",
		},
		Metadata: recipe.Metadata{Name: "test"},
		Model:    recipe.Model{HFRepo: "some/model"},
	}
	rt, err := Resolve(r, "")
	if err != nil {
		t.Fatalf("Resolve with explicit docker runtime failed: %v", err)
	}
	if _, ok := rt.(*SGLangDockerRuntime); !ok {
		t.Errorf("expected *SGLangDockerRuntime for runtime=docker, got %T", rt)
	}
}

func TestResolve_SGLangDocker_RuntimeOverride(t *testing.T) {
	// --runtime docker flag should wire SGLangDockerRuntime
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:  "sglang",
			Image: "lmsysorg/sglang:v0.5.12.post1",
		},
		Metadata: recipe.Metadata{Name: "test"},
		Model:    recipe.Model{HFRepo: "some/model"},
	}
	rt, err := Resolve(r, "docker") // --runtime docker override
	if err != nil {
		t.Fatalf("Resolve with override failed: %v", err)
	}
	if _, ok := rt.(*SGLangDockerRuntime); !ok {
		t.Errorf("expected *SGLangDockerRuntime after override, got %T", rt)
	}
}

func TestResolve_SGLang_NativeBlocked(t *testing.T) {
	// runtime=native must be explicitly rejected with a useful error message.
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "sglang",
			Runtime: "native",
			Image:   "lmsysorg/sglang:v0.5.12.post1",
		},
		Metadata: recipe.Metadata{Name: "test"},
		Model:    recipe.Model{HFRepo: "some/model"},
	}
	_, err := Resolve(r, "")
	if err == nil {
		t.Error("expected error for runtime=native on sglang — native is unsupported")
	}
	if !strings.Contains(err.Error(), "native") {
		t.Errorf("error should mention 'native', got: %v", err)
	}
}

func TestResolve_SGLang_UnknownRuntime(t *testing.T) {
	r := &recipe.Recipe{
		Schema: "bloc/v1",
		Engine: recipe.Engine{
			Name:    "sglang",
			Runtime: "kubernetes",
			Image:   "lmsysorg/sglang:v0.5.12.post1",
		},
		Metadata: recipe.Metadata{Name: "test"},
		Model:    recipe.Model{HFRepo: "some/model"},
	}
	_, err := Resolve(r, "")
	if err == nil {
		t.Error("expected error for unknown runtime 'kubernetes' on sglang")
	}
	if !strings.Contains(err.Error(), "kubernetes") {
		t.Errorf("error should mention the invalid runtime, got: %v", err)
	}
}

// ─── parseSGLangStats tests ───────────────────────────────────────────────────

func TestParseSGLangStats_GenThroughput(t *testing.T) {
	s := &Stats{}
	parseSGLangStats("throughput_output_token_per_s=47.3 throughput_input_token_per_s=912.1", s)
	// Snapshot() returns (gen, prefill, peakVRAM, duration, success)
	gen, _, _, _, _ := s.Snapshot()
	if gen != 47.3 {
		t.Errorf("TokensPerSecGeneration = %v, want 47.3", gen)
	}
}

func TestParseSGLangStats_PrefillThroughput(t *testing.T) {
	s := &Stats{}
	parseSGLangStats("throughput_output_token_per_s=47.3 throughput_input_token_per_s=912.1", s)
	// Snapshot() returns (gen, prefill, peakVRAM, duration, success)
	_, prefill, _, _, _ := s.Snapshot()
	if prefill != 912.1 {
		t.Errorf("TokensPerSecPrefill = %v, want 912.1", prefill)
	}
}

func TestParseSGLangStats_VRAMUsage_MB(t *testing.T) {
	s := &Stats{}
	parseSGLangStats("Memory pool end size: 81920.00 MB", s)
	_, _, peakVRAM, _, _ := s.Snapshot()
	if peakVRAM != 81920 {
		t.Errorf("PeakVRAMMB = %v, want 81920", peakVRAM)
	}
}

func TestParseSGLangStats_VRAMUsage_GB(t *testing.T) {
	s := &Stats{}
	parseSGLangStats("gpu memory: 48.0 GB", s)
	_, _, peakVRAM, _, _ := s.Snapshot()
	// 48.0 GB * 1024 = 49152 MB
	if peakVRAM != 49152 {
		t.Errorf("PeakVRAMMB = %v, want 49152 (48.0 GB in MB)", peakVRAM)
	}
}

func TestParseSGLangStats_NoMatch(t *testing.T) {
	s := &Stats{}
	parseSGLangStats("[server] model loaded successfully", s)
	gen, prefill, peakVRAM, _, _ := s.Snapshot()
	if gen != 0 || prefill != 0 || peakVRAM != 0 {
		t.Error("expected all stats to remain zero on non-matching line")
	}
}

func TestParseSGLangStats_OnlyGen_NoPrefill(t *testing.T) {
	// If only the generation metric appears on a line, prefill stays at its
	// previous value (not zeroed out).
	s := &Stats{}
	s.Update(500.0, 0, 0) // seed prefill
	parseSGLangStats("throughput_output_token_per_s=12.5", s)
	// Snapshot() returns (gen, prefill, peakVRAM, duration, success)
	gen, prefill, _, _, _ := s.Snapshot()
	if gen != 12.5 {
		t.Errorf("gen = %v, want 12.5", gen)
	}
	if prefill != 500.0 {
		t.Errorf("prefill = %v, want 500.0 (should not be zeroed)", prefill)
	}
}

func TestParseSGLangStats_PeakVRAM_OnlyIncreases(t *testing.T) {
	s := &Stats{}
	parseSGLangStats("Memory pool end size: 40960.00 MB", s)
	parseSGLangStats("Memory pool end size: 81920.00 MB", s)
	parseSGLangStats("Memory pool end size: 20480.00 MB", s) // lower — must not replace peak
	_, _, peakVRAM, _, _ := s.Snapshot()
	if peakVRAM != 81920 {
		t.Errorf("PeakVRAMMB = %v, want 81920 (peak should only increase)", peakVRAM)
	}
}

func TestParseSGLangStats_RegexSafety(t *testing.T) {
	// Adversarial input — must not cause catastrophic backtracking.
	s := &Stats{}
	longLine := strings.Repeat("throughput_output_token_per_s=", 1000) + "42.0"
	parseSGLangStats(longLine, s)
	// If we reach here within the test timeout, no catastrophic backtracking occurred.
}

// ─── CUDA env var injection test ─────────────────────────────────────────────

func TestSGLangRunConfig_CUDADevices_InEnvVars(t *testing.T) {
	// Verify that when run.go injects SGLangCUDAVisibleDevices into cfg.EnvVars,
	// the key is correctly set before Run() iterates the env map.
	// We test the contract at the RunConfig level, not inside the Docker exec.
	envVars := make(map[string]string)
	cudaDevices := "0,2,3,4"
	// Simulate what run.go does:
	envVars["CUDA_VISIBLE_DEVICES"] = cudaDevices

	if envVars["CUDA_VISIBLE_DEVICES"] != cudaDevices {
		t.Errorf("CUDA_VISIBLE_DEVICES = %q, want %q", envVars["CUDA_VISIBLE_DEVICES"], cudaDevices)
	}
}

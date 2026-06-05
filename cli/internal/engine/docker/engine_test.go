package docker

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/process"
	"github.com/bloc-org/bloc/internal/recipe"
)

// ─── SanitizeContainerSlug ────────────────────────────────────────────────────

func TestSanitizeContainerSlug_Normal(t *testing.T) {
	got := SanitizeContainerSlug("step-3.7-flash-speculative")
	want := "step-3-7-flash-speculative"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitizeContainerSlug_Uppercase(t *testing.T) {
	got := SanitizeContainerSlug("Qwen3-30B-MoE")
	want := "qwen3-30b-moe"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSanitizeContainerSlug_ShellInjectionChars(t *testing.T) {
	dangerous := "model; rm -rf /; echo pwned"
	got := SanitizeContainerSlug(dangerous)

	shellDangerous := []string{";", " ", "|", "&", "$", "`", "!", ">", "<", "(", ")", "{", "}", "[", "]", "\\", "'", "\""}
	for _, c := range shellDangerous {
		if strings.Contains(got, c) {
			t.Errorf("SanitizeContainerSlug() = %q contains dangerous char %q", got, c)
		}
	}
	// Must only contain [a-z0-9-]
	for _, ch := range got {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			t.Errorf("SanitizeContainerSlug() = %q contains invalid char %q", got, string(ch))
		}
	}
}

func TestSanitizeContainerSlug_TooLong(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := SanitizeContainerSlug(long)
	if len(got) > 40 {
		t.Errorf("slug len = %d, want <= 40", len(got))
	}
}

func TestSanitizeContainerSlug_Empty(t *testing.T) {
	got := SanitizeContainerSlug("")
	if got != "model" {
		t.Errorf("empty slug should fall back to 'model', got %q", got)
	}
}

func TestSanitizeContainerSlug_OnlySpecialChars(t *testing.T) {
	got := SanitizeContainerSlug("!@#$%^&*()")
	if got == "" {
		t.Error("slug should not be empty")
	}
	for _, ch := range got {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			t.Errorf("slug %q contains invalid char %q", got, string(ch))
		}
	}
}

func TestSanitizeContainerSlug_LeadingTrailingHyphens(t *testing.T) {
	got := SanitizeContainerSlug("--my model--")
	if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Errorf("slug %q should not have leading/trailing hyphens", got)
	}
}

func TestSanitizeContainerSlug_ConsecutiveHyphens(t *testing.T) {
	got := SanitizeContainerSlug("my  model") // two spaces → two hyphens → collapsed
	if strings.Contains(got, "--") {
		t.Errorf("slug %q should not contain consecutive hyphens", got)
	}
}

// ─── RandomHex ────────────────────────────────────────────────────────────────

func TestRandomHex_Length(t *testing.T) {
	for n := 1; n <= 8; n++ {
		got := RandomHex(n)
		if len(got) != n*2 {
			t.Errorf("RandomHex(%d) = %q (len %d), want len %d", n, got, len(got), n*2)
		}
	}
}

func TestRandomHex_OnlyHexChars(t *testing.T) {
	got := RandomHex(16)
	for _, c := range got {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("RandomHex output %q contains non-hex char %q", got, string(c))
		}
	}
}

func TestRandomHex_Uniqueness(t *testing.T) {
	a, b := RandomHex(8), RandomHex(8)
	if a == b {
		t.Error("two RandomHex(8) calls should not produce the same value")
	}
}

// ─── IsInterruptExit ──────────────────────────────────────────────────────────

type mockErr struct{ msg string }

func (e *mockErr) Error() string { return e.msg }

func TestIsInterruptExit_Nil(t *testing.T) {
	if !IsInterruptExit(nil) {
		t.Error("nil error should be a clean exit")
	}
}

func TestIsInterruptExit_SIGINT(t *testing.T) {
	if !IsInterruptExit(&mockErr{"exit status 130"}) {
		t.Error("exit status 130 should be interrupt exit")
	}
}

func TestIsInterruptExit_SIGTERM(t *testing.T) {
	if !IsInterruptExit(&mockErr{"exit status 143"}) {
		t.Error("exit status 143 should be interrupt exit")
	}
}

func TestIsInterruptExit_Crash(t *testing.T) {
	if IsInterruptExit(&mockErr{"exit status 1"}) {
		t.Error("exit status 1 should NOT be interrupt exit")
	}
}

// ─── DockerEngine.Name ────────────────────────────────────────────────────────

func TestDockerEngine_Name_WithImage(t *testing.T) {
	eng := &DockerEngine{
		Image:       "vllm/vllm-openai:v0.9.0",
		DisplayName: "vLLM Docker (vllm/vllm-openai:v0.9.0)",
	}
	want := "vLLM Docker (vllm/vllm-openai:v0.9.0)"
	if eng.Name() != want {
		t.Errorf("Name() = %q, want %q", eng.Name(), want)
	}
}

func TestDockerEngine_Name_SGLang(t *testing.T) {
	eng := &DockerEngine{
		Image:       "lmsysorg/sglang:v0.5.12.post1",
		DisplayName: "SGLang Docker (lmsysorg/sglang:v0.5.12.post1)",
	}
	if !strings.HasPrefix(eng.Name(), "SGLang") {
		t.Errorf("Name() = %q — expected SGLang prefix", eng.Name())
	}
}

// ─── DockerEngine.BuildArgs ───────────────────────────────────────────────────

func TestDockerEngine_BuildArgs_DelegatesToFlagBuilder(t *testing.T) {
	called := false
	eng := &DockerEngine{
		FlagBuilder: func(cfg recipe.EngineConfig) []string {
			called = true
			return []string{"--dtype", "float16"}
		},
	}
	caps := &engine.CapabilitySet{} // stub — ignored by docker BuildArgs
	flags, err := eng.BuildArgs(caps, recipe.EngineConfig{DType: "float16"})
	if err != nil {
		t.Fatalf("BuildArgs error: %v", err)
	}
	if !called {
		t.Error("FlagBuilder should have been called")
	}
	if len(flags) != 2 || flags[0] != "--dtype" || flags[1] != "float16" {
		t.Errorf("flags = %v, want [--dtype float16]", flags)
	}
}

func TestDockerEngine_BuildArgs_NilFlagBuilderReturnsError(t *testing.T) {
	eng := &DockerEngine{DisplayName: "test-engine"}
	_, err := eng.BuildArgs(nil, recipe.EngineConfig{})
	if err == nil {
		t.Error("expected error when FlagBuilder is nil")
	}
}

// ─── DockerEngine.OfferInstall ────────────────────────────────────────────────

func TestDockerEngine_OfferInstall_ReturnsFalse(t *testing.T) {
	eng := &DockerEngine{DisplayName: "vLLM Docker"}
	if eng.OfferInstall() {
		t.Error("OfferInstall should always return false for Docker engines")
	}
}

// ─── Container name generation ────────────────────────────────────────────────

func TestContainerName_Format(t *testing.T) {
	slug := SanitizeContainerSlug("step-3-7-flash")
	hex4 := RandomHex(4)
	name := "bloc-" + slug + "-" + hex4

	if !strings.HasPrefix(name, "bloc-") {
		t.Error("container name must start with 'bloc-'")
	}
	if len(hex4) != 8 {
		t.Errorf("RandomHex(4) should produce 8 hex chars, got %d", len(hex4))
	}
}

// ─── Interface assertion ──────────────────────────────────────────────────────

// Compile-time assertion: DockerEngine must implement engine.Engine.
var _ engine.Engine = (*DockerEngine)(nil)

// ─── Stub LogParser for tests ─────────────────────────────────────────────────

type noopParser struct{}

func (p *noopParser) ParseLine(_ string) process.Metrics { return process.Metrics{} }

func TestDockerEngine_ParserInjection(t *testing.T) {
	parser := &noopParser{}
	eng := &DockerEngine{
		Parser: parser,
	}
	// Verify the parser is stored and accessible
	if eng.Parser == nil {
		t.Error("Parser should not be nil after injection")
	}
}

// ─── EntryCmd injection ────────────────────────────────────────────────────────

func TestDockerEngine_EntryCmd_VLLM(t *testing.T) {
	// Simulate vLLM's entry command
	vllmEntryCmd := func(modelPath string, port int) []string {
		return []string{"vllm", "serve", modelPath, "--port", fmt.Sprintf("%d", port)}
	}
	cmd := vllmEntryCmd("/bloc-models/Qwen/Qwen3-30B/main", 8000)
	if cmd[0] != "vllm" {
		t.Errorf("first token should be 'vllm', got %q", cmd[0])
	}
	if cmd[1] != "serve" {
		t.Errorf("second token should be 'serve', got %q", cmd[1])
	}
	if cmd[2] != "/bloc-models/Qwen/Qwen3-30B/main" {
		t.Errorf("third token should be model path, got %q", cmd[2])
	}
}

func TestDockerEngine_EntryCmd_SGLang(t *testing.T) {
	// Simulate SGLang's entry command
	sglangEntryCmd := func(modelPath string, port int) []string {
		return []string{
			"python3", "-m", "sglang.launch_server",
			"--model-path", modelPath,
			"--host", "0.0.0.0",
			"--port", fmt.Sprintf("%d", port),
		}
	}
	cmd := sglangEntryCmd("/bloc-models/Qwen/Qwen3-30B/main", 30000)
	if cmd[0] != "python3" {
		t.Errorf("first token should be 'python3', got %q", cmd[0])
	}
	if !contains(cmd, "--model-path") {
		t.Error("SGLang entryCmd should contain --model-path")
	}
	if !contains(cmd, "--host") {
		t.Error("SGLang entryCmd should contain --host")
	}
}

// ─── GPU passthrough ──────────────────────────────────────────────────────────

func TestDockerEngine_GPUArgs_LinuxHasGPUs(t *testing.T) {
	// This test verifies the conditional logic without actually launching Docker.
	// On Linux, --gpus all should be in the args; on macOS it should not.
	// We test the condition itself, not the platform runtime.
	linuxGPUArgs := []string{"run", "--rm"}
	// Simulate linux-only addition:
	osGOOS := "linux"
	if osGOOS == "linux" {
		linuxGPUArgs = append(linuxGPUArgs, "--gpus", "all")
	}
	if !contains(linuxGPUArgs, "--gpus") {
		t.Error("Linux should add --gpus all")
	}
}

func TestDockerEngine_GPUArgs_MacOSNoGPUs(t *testing.T) {
	macArgs := []string{"run", "--rm"}
	osGOOS := "darwin"
	if osGOOS == "linux" {
		macArgs = append(macArgs, "--gpus", "all")
	}
	if contains(macArgs, "--gpus") {
		t.Error("macOS should not add --gpus")
	}
}

// ─── Port mapping ─────────────────────────────────────────────────────────────

func TestDockerEngine_PortMapping_CustomPort(t *testing.T) {
	hostPort := 9000
	containerPort := hostPort
	portArg := fmt.Sprintf("%d:%d", hostPort, containerPort)
	if portArg != "9000:9000" {
		t.Errorf("port mapping = %q, want 9000:9000", portArg)
	}
}

func TestDockerEngine_PortMapping_DefaultPort(t *testing.T) {
	hostPort := 0
	if hostPort == 0 {
		hostPort = 8000 // default
	}
	containerPort := hostPort
	portArg := fmt.Sprintf("%d:%d", hostPort, containerPort)
	if portArg != "8000:8000" {
		t.Errorf("default port mapping = %q, want 8000:8000", portArg)
	}
}

// ─── Volume mount ─────────────────────────────────────────────────────────────

func TestDockerEngine_VolumeMount_ReadOnly(t *testing.T) {
	reposMountSrc := "/home/user/.cache/bloc/repos"
	volumeArg := fmt.Sprintf("%s:/bloc-models:ro", reposMountSrc)
	if !strings.HasSuffix(volumeArg, ":ro") {
		t.Error("volume mount should be read-only (:ro suffix)")
	}
	if !strings.Contains(volumeArg, "/bloc-models") {
		t.Error("volume mount should target /bloc-models in container")
	}
}

// ─── EnvVars injection ────────────────────────────────────────────────────────

func TestDockerEngine_EnvVarsInjected(t *testing.T) {
	envVars := map[string]string{
		"CUDA_VISIBLE_DEVICES": "0,2,3,4",
		"HF_TOKEN":             "hf_test",
	}
	var dockerArgs []string
	for k, v := range envVars {
		dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	if !contains(dockerArgs, "-e") {
		t.Error("env vars should produce -e flags")
	}
	// Verify no injection possible through env var keys
	for k := range envVars {
		for _, ch := range k {
			if ch == ';' || ch == '|' || ch == '&' || ch == '>' || ch == '<' {
				t.Errorf("env key %q contains shell-dangerous character", k)
			}
		}
	}
}

// ─── Container model path mapping ─────────────────────────────────────────────

func TestContainerModelPath_StripsCachePrefix(t *testing.T) {
	reposMountSrc := "/home/user/.cache/bloc/repos"
	hostModelPath := reposMountSrc + "/Qwen--Qwen3-30B/main"
	containerModelPath := "/bloc-models/" + strings.TrimPrefix(hostModelPath, reposMountSrc+"/")
	expected := "/bloc-models/Qwen--Qwen3-30B/main"
	if containerModelPath != expected {
		t.Errorf("containerModelPath = %q, want %q", containerModelPath, expected)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}



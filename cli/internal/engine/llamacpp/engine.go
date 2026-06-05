// Package llamacpp implements the engine.Engine interface for llama-server,
// the llama.cpp inference server binary.
//
// Key design decisions:
//  1. Capabilities() runs `llama-server --help`, parses flag lines with a
//     compiled regex, and passes the raw FlagSpec map to engine.BuildCapabilities().
//     The result is cached in a sync.Once — the binary is never run twice.
//  2. BuildArgs() calls caps.FlagFor("feature_name") to resolve the current
//     flag string for each EngineConfig field. It never hardcodes flag names.
//     If a required feature is absent from caps, it returns a structured error
//     naming the missing feature — not the raw flag — so the message stays
//     stable even when llama.cpp renames flags.
//  3. NewSupervisor() constructs the *exec.Cmd and delegates process lifecycle
//     to process.Supervisor (log fan-out, signal handling, readiness polling).
//  4. OfferInstall() mirrors the logic from runtime/llama_cpp.go verbatim.
package llamacpp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/process"
	"github.com/bloc-org/bloc/internal/recipe"
)

// Engine implements engine.Engine for llama-server.
type Engine struct {
	// capsOnce ensures the binary is probed exactly once per Engine instance.
	capsOnce   sync.Once
	caps       *engine.CapabilitySet
	capsErr    error
	// PERF-20 (PM-9): Cache the resolved binary path from probe() so we don't
	// do a second filesystem LookPath in NewSupervisor.
	binaryPath string
}

// New constructs a LlamaCppEngine for the given recipe.
// Called from the engine registry — recipe is available if needed for version
// pin checks in future phases.
func New(_ *recipe.Recipe) (engine.Engine, error) {
	return &Engine{}, nil
}

// Name returns the display label shown in CLI step headers.
func (e *Engine) Name() string { return "llama-server" }

// Capabilities probes the llama-server binary via --help and returns a
// CapabilitySet describing what this specific build supports.
//
// The probe runs at most once per Engine instance (sync.Once). Subsequent
// calls return the cached result without re-executing the binary.
//
// The context is honoured: if it is cancelled before the binary exits,
// Capabilities returns ctx.Err().
func (e *Engine) Capabilities(ctx context.Context) (*engine.CapabilitySet, error) {
	e.capsOnce.Do(func() {
		e.caps, e.capsErr = e.probe(ctx)
	})
	return e.caps, e.capsErr
}

// probe is the internal implementation of Capabilities.
func (e *Engine) probe(ctx context.Context) (*engine.CapabilitySet, error) {
	path, err := exec.LookPath("llama-server")
	if err != nil {
		return nil, fmt.Errorf(
			"llama-server not found in PATH\n" +
				"  Install llama.cpp and make sure llama-server is in your PATH.\n" +
				"  macOS: brew install llama.cpp\n" +
				"  Linux: https://github.com/ggml-org/llama.cpp/releases",
		)
	}
	e.binaryPath = path

	// Use a 10-second timeout for the --help probe so a hung binary does
	// not stall the CLI indefinitely.
	probeCtx, cancel := context.WithTimeout(ctx, 10e9) // 10 * time.Second
	defer cancel()

	cmd := exec.CommandContext(probeCtx, path, "--help")
	out, _ := cmd.CombinedOutput() // llama-server exits 1 on --help; ignore exit code
	if len(out) == 0 {
		return nil, fmt.Errorf("llama-server --help produced no output (binary may be corrupt)")
	}

	rawFlags := parseFlagsFromHelp(string(out))

	// Extract version string from the help output or binary name.
	// llama-server --version is not universally supported; fall back to "".
	version := extractVersion(string(out))

	return engine.BuildCapabilities("llama-server", version, rawFlags), nil
}

// BuildArgs converts a recipe's EngineConfig into the ordered list of
// llama-server CLI flags, using caps to resolve the correct flag name and
// syntax for this specific binary version.
//
// Every non-zero / non-false field in cfg emits the corresponding flag token(s)
// via caps.FlagFor("feature") + spec.Emit(value). This means BuildArgs never
// hardcodes a flag string — the flag name comes from the probed binary.
//
// Returns a structured error if cfg requires a feature that caps does not support.
func (e *Engine) BuildArgs(caps *engine.CapabilitySet, cfg recipe.EngineConfig) ([]string, error) {
	var flags []string

	// ── Helpers ──────────────────────────────────────────────────────────────

	// addInt emits a flag + integer value when value != 0.
	addInt := func(feature string, value int) {
		if value == 0 {
			return
		}
		spec, ok := caps.FlagFor(feature)
		if !ok {
			return // feature not supported by this build — silently skip
		}
		// PERF-23 (PL-1): Use strconv.Itoa instead of fmt.Sprintf for integer formatting.
		flags = append(flags, spec.Emit(strconv.Itoa(value))...)
	}

	// addStr emits a flag + string value when value is non-empty.
	addStr := func(feature, value string) {
		if value == "" {
			return
		}
		spec, ok := caps.FlagFor(feature)
		if !ok {
			return
		}
		flags = append(flags, spec.Emit(value)...)
	}

	// addFloat emits a flag + float value when value != 0.
	addFloat := func(feature string, value float64) {
		if value == 0 {
			return
		}
		spec, ok := caps.FlagFor(feature)
		if !ok {
			return
		}
		flags = append(flags, spec.Emit(fmt.Sprintf("%.2f", value))...)
	}

	// addBool emits a flag (no value) when value is true.
	addBool := func(feature string, value bool) {
		if !value {
			return
		}
		spec, ok := caps.FlagFor(feature)
		if !ok {
			return
		}
		flags = append(flags, spec.Emit("")...)
	}

	// ── Context ───────────────────────────────────────────────────────────────
	addInt("ctx_size", cfg.CtxSize)

	// ── GPU offloading ────────────────────────────────────────────────────────
	addInt("gpu_layers", cfg.GPULayers)
	addStr("split_mode", cfg.SplitMode)
	addStr("tensor_split", cfg.TensorSplit)
	// main_gpu: only emit when > 0 (default 0 is the implicit default)
	if cfg.MainGPU > 0 {
		addInt("main_gpu", cfg.MainGPU)
	}

	// ── MoE CPU offload ────────────────────────────────────────────────────────
	addInt("moe_cpu_offload", cfg.NCPUMoE)

	// ── Flash attention ────────────────────────────────────────────────────────
	// Flash attention is a required feature when the recipe requests it.
	// Emit the correct form — old builds use bare -fa (bool_implicit),
	// new builds use --flash-attn on (bool_on_off). caps resolves this.
	if cfg.FlashAttn {
		spec, ok := caps.FlagFor("flash_attn")
		if !ok {
			return nil, fmt.Errorf(
				"recipe requires flash_attn but llama-server build does not support it\n" +
					"  Upgrade llama.cpp: brew upgrade llama.cpp",
			)
		}
		switch spec.ValueType {
		case engine.ValueTypeBoolOnOff:
			flags = append(flags, spec.Name, "on")
		default: // bool_implicit: bare -fa
			flags = append(flags, spec.Name)
		}
	}

	// ── Batching ──────────────────────────────────────────────────────────────
	addInt("batch_size", cfg.BatchSize)
	// ubatch_size: look up via raw flag name since it was added later
	if cfg.UBatchSize != 0 {
		if spec, ok := caps.RawFlag("-ub"); ok {
			flags = append(flags, spec.Emit(strconv.Itoa(cfg.UBatchSize))...)
		} else if spec, ok := caps.RawFlag("--ubatch-size"); ok {
			flags = append(flags, spec.Emit(strconv.Itoa(cfg.UBatchSize))...)
		}
	}

	// ── KV cache types ────────────────────────────────────────────────────────
	addStr("kv_cache_type_k", cfg.CacheTypeK)
	addStr("kv_cache_type_v", cfg.CacheTypeV)

	// ── Speculative decoding ──────────────────────────────────────────────────
	addStr("spec_type", cfg.SpecType)
	addStr("spec_draft_model", cfg.SpecDraftModel)
	addInt("spec_draft_n_max", cfg.SpecDraftNMax)
	addFloat("spec_draft_p_min", cfg.SpecDraftPMin)

	// ── Threading ─────────────────────────────────────────────────────────────
	addInt("threads", cfg.Threads)
	addStr("numa", cfg.NUMA)

	// ── Memory ────────────────────────────────────────────────────────────────
	addBool("mlock", cfg.MLock)
	if cfg.MMap != nil && !*cfg.MMap {
		// --no-mmap is a flag-only toggle; emit directly from caps.
		if spec, ok := caps.FlagFor("mmap"); ok {
			flags = append(flags, spec.Emit("")...)
		}
	}

	// ── Server ────────────────────────────────────────────────────────────────
	addStr("host", cfg.Host)
	addInt("port", cfg.Port)
	addInt("parallel", cfg.NParallel)
	addBool("jinja", cfg.Jinja)

	// ── Extra args (verbatim, validated at recipe.Parse time) ─────────────────
	flags = append(flags, cfg.ExtraArgs...)

	return flags, nil
}

// NewSupervisor constructs and returns a process.Supervisor ready to launch
// llama-server with the given LaunchConfig.
//
// The caller is responsible for calling Supervisor.Run(ctx).
func (e *Engine) NewSupervisor(cfg engine.LaunchConfig) (*process.Supervisor, error) {
	binary := e.binaryPath
	if binary == "" {
		binary = "llama-server" // Fallback if probe was somehow skipped
	}

	// Model path is always the first argument.
	allArgs := append([]string{"-m", cfg.ModelPath}, cfg.Flags...)
	cmd := exec.Command(binary, allArgs...)

	// Inherit env (minus dangerous loader vars) and inject recipe env vars.
	// M-2: Use SafeEnviron() to strip LD_PRELOAD, DYLD_INSERT_LIBRARIES, etc.
	cmd.Env = engine.SafeEnviron()
	for k, v := range cfg.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Platform-specific process group setup (see sysproc_unix.go / sysproc_windows.go).
	setSysProcAttr(cmd)

	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	fmt.Fprintf(os.Stderr, "\n\033[32m✅ llama-server starting...\033[0m\n")
	fmt.Fprintf(os.Stderr, "\033[36m   Chat UI: http://127.0.0.1:%d\033[0m\n", port)
	fmt.Fprintf(os.Stderr, "\033[90m   Press Ctrl+C to stop\033[0m\n\n")

	return process.New(process.Config{
		Cmd:       cmd,
		LogWriter: cfg.LogWriter,
		Parser:    &llamaCppLogParser{},
		Silent:    cfg.Silent,
		KillFunc:  func() { killProcessGroup(cmd.Process) },
	})
}

// OfferInstall prompts the user to install llama.cpp and attempts installation.
// Returns true if llama-server is available after the call.
// Migrated from runtime/llama_cpp.go with no logic changes.
func (e *Engine) OfferInstall() bool {
	switch runtime.GOOS {
	case "darwin":
		fmt.Print("\n  Would you like to install llama.cpp via Homebrew now? [Y/n]: ")
		var ans string
		fmt.Scanln(&ans)
		ans = strings.ToLower(strings.TrimSpace(ans))
		if ans != "" && ans != "y" && ans != "yes" {
			fmt.Println("  Skipped. Re-run after installing manually:")
			fmt.Println("    brew install llama.cpp")
			return false
		}
		fmt.Println("  Running: brew install llama.cpp ...")
		brewPath, brewErr := exec.LookPath("brew")
		if brewErr != nil {
			fmt.Fprintln(os.Stderr, "\033[31m✗  brew not found in PATH\033[0m")
			return false
		}
		if !strings.HasPrefix(brewPath, "/opt/homebrew/") && !strings.HasPrefix(brewPath, "/usr/local/") {
			fmt.Fprintf(os.Stderr, "\033[31m✗  brew binary at unexpected path %q — refusing to execute\033[0m\n", brewPath)
			return false
		}
		cmd := exec.Command(brewPath, "install", "llama.cpp")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "\n\033[31m✗  brew install failed: %v\033[0m\n", err)
			return false
		}
		if _, err := exec.LookPath("llama-server"); err != nil {
			fmt.Fprintln(os.Stderr, "\033[33m⚠  llama-server still not found after install. Try opening a new terminal.\033[0m")
			return false
		}
		fmt.Println("  \033[32m✓\033[0m  llama.cpp installed successfully.")
		return true

	case "linux":
		fmt.Println("  Auto-install is not supported on Linux.")
		fmt.Println("  Download a prebuilt binary from:")
		fmt.Println("    https://github.com/ggml-org/llama.cpp/releases")
		fmt.Println("  or build from source: https://bloc-theta.vercel.app/install")
		return false

	default:
		fmt.Println("  Auto-install is not supported on this platform.")
		fmt.Println("  Install guide: https://bloc-theta.vercel.app/install")
		return false
	}
}

// ─── Flag parsing ──────────────────────────────────────────────────────────────

// inlineFlagRe catches additional flag tokens on the same line.
var inlineFlagRe = regexp.MustCompile(`(-{1,2}[a-zA-Z][a-zA-Z0-9\-]*)`)

// boolOnOffRe detects [on|off|auto] value patterns — indicates bool_on_off.
var boolOnOffRe = regexp.MustCompile(`\[(on|off|auto)[|\]]`)

// intArgRe detects integer value patterns — indicates ValueTypeInt.
var intArgRe = regexp.MustCompile(`(?i)\b(N|INT|COUNT|SIZE)\b`)

// floatArgRe detects float value patterns — indicates ValueTypeFloat.
var floatArgRe = regexp.MustCompile(`(?i)\b(P|FLOAT)\b`)

// parseFlagsFromHelp parses llama-server --help output into a raw FlagSpec map.
// Each flag gets a ValueType inferred from the description line.
func parseFlagsFromHelp(helpText string) map[string]engine.FlagSpec {
	flags := make(map[string]engine.FlagSpec)

	// PERF-19 (PM-2): Use bufio.Scanner instead of strings.Split to avoid
	// allocating a massive slice of all lines in memory.
	scanner := bufio.NewScanner(strings.NewReader(helpText))
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(trimmed, "-") {
			continue
		}

		// Infer ValueType from description clues on this line.
		vt := inferValueType(trimmed)
		takesValue := vt != engine.ValueTypeBoolImplicit

		for _, m := range inlineFlagRe.FindAllString(trimmed, -1) {
			if _, exists := flags[m]; !exists {
				flags[m] = engine.FlagSpec{
					Name:       m,
					TakesValue: takesValue,
					ValueType:  vt,
				}
			}
		}
	}

	return flags
}

// inferValueType determines the ValueType for a flag from its help line text.
func inferValueType(line string) string {
	if boolOnOffRe.MatchString(line) {
		return engine.ValueTypeBoolOnOff
	}
	if floatArgRe.MatchString(line) {
		return engine.ValueTypeFloat
	}
	if intArgRe.MatchString(line) {
		return engine.ValueTypeInt
	}
	// PERF-18 (PM-1): strings.ContainsAny checks character sets, not substrings.
	// Replaced with loop over strings.Contains for proper substring matching.
	for _, token := range []string{"FNAME", "PATH", "TYPE", "STRATEGY", "PROMPT", "NAME", "TEMPLATE", "HOST", "ADDR", "IP"} {
		if strings.Contains(line, token) {
			return engine.ValueTypeString
		}
	}
	// Default: bool_implicit (bare flag, no value required).
	return engine.ValueTypeBoolImplicit
}

// versionRe is used to extract the build number from llama.cpp's --help output.
// PERF-10 (PH-1): Compiled at package level rather than inside extractVersion.
var versionRe = regexp.MustCompile(`build[:\s]+(\d+)`)

// extractVersion attempts to parse a build number from --help output.
// llama.cpp reports something like "build: 1234 (abc1234)" in early lines.
// Returns "" if not found — callers treat "" as "unknown version".
func extractVersion(helpText string) string {
	if m := versionRe.FindStringSubmatch(helpText); len(m) > 1 {
		return "b" + m[1]
	}
	return ""
}

// ─── Stats parsing ─────────────────────────────────────────────────────────────

// llamaCppLogParser adapts parseLlamaStats to the process.LogParser interface.
// Kept here (next to its tests) while letting the Supervisor call it generically.
type llamaCppLogParser struct{}

func (p *llamaCppLogParser) ParseLine(line string) process.Metrics {
	var m process.Metrics
	if match := llamaGenRe.FindStringSubmatch(line); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			m.TokensPerSecGen = val
		}
	}
	if match := llamaPromptRe.FindStringSubmatch(line); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			m.TokensPerSecPrefill = val
		}
	}
	if match := llamaVRAMRe.FindStringSubmatch(line); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			unit := strings.ToUpper(match[2])
			if unit == "GB" || unit == "GIB" {
				val *= 1024
			}
			m.PeakVRAMMB = int64(val)
		}
	}
	return m
}

// P-10: All regexes compiled once at package init — not per log line.
var (
	llamaGenRe    = regexp.MustCompile(`eval time\s*=.*?([\d.]+)\s*tokens per second`)
	llamaPromptRe = regexp.MustCompile(`prompt eval time\s*=.*?([\d.]+)\s*tokens per second`)
	// SEC-00: (?i) flag avoids strings.ToUpper(line) on every log line.
	llamaVRAMRe = regexp.MustCompile(`(?i)VRAM\s+USED\s*[=:]\s*([\d.]+)\s*(MB|MIB|GB|GIB)`)
)

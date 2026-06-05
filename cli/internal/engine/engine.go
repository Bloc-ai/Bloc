// Package engine defines the core interfaces and types for Bloc's inference
// engine system. Every supported backend (llama.cpp, vLLM, SGLang, …) must
// implement the Engine interface. Orchestration code (cmd/run.go, pipeline/)
// depends only on this package — no engine-specific imports leak upward.
//
// Dependency graph (no cycles):
//
//	recipe → engine → process
//	engine/llamacpp → engine
//	engine/vllm     → engine
//	engine/sglang   → engine
//	engine/docker   → engine
//	pipeline        → engine
//
// Canonical usage:
//
//	eng, err := engine.Resolve(recipe)
//	caps, err := eng.Capabilities(ctx)
//	args, err := eng.BuildArgs(caps, recipe.EngineConfig)
//	sv, err  := eng.NewSupervisor(engine.LaunchConfig{...})
//	stats, err := sv.Run(ctx)
package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bloc-org/bloc/internal/process"
	"github.com/bloc-org/bloc/internal/recipe"
)

// ─── FlagSpec ─────────────────────────────────────────────────────────────────

// ValueType describes how a flag receives its value on the command line.
// This determines what Emit() generates.
const (
	// ValueTypeBoolOnOff: the flag takes an explicit "on"/"off"/"auto" argument.
	// Example: --flash-attn on
	ValueTypeBoolOnOff = "bool_on_off"

	// ValueTypeBoolImplicit: the flag is a boolean toggle with no value.
	// Presence alone enables the feature.
	// Example: -fa
	ValueTypeBoolImplicit = "bool_implicit"

	// ValueTypeString: the flag takes a free-form string argument.
	// Example: --model /path/to/model.gguf
	ValueTypeString = "string"

	// ValueTypeInt: the flag takes an integer argument.
	// Example: --ctx-size 4096
	ValueTypeInt = "int"

	// ValueTypeFloat: the flag takes a floating-point argument.
	// Example: --temperature 0.7
	ValueTypeFloat = "float"
)

// FlagSpec describes one flag as seen in an engine's --help output.
type FlagSpec struct {
	// Name is the canonical flag string as emitted on the command line,
	// e.g. "--flash-attn", "-fa", "--ctx-size".
	Name string

	// TakesValue is true when the flag requires a separate value token after it.
	TakesValue bool

	// ValueType classifies the value's type. See ValueType* constants.
	// Ignored when TakesValue is false.
	ValueType string
}

// Emit returns the ordered list of command-line tokens for this flag with the
// given value string. The caller passes an empty string for implicit booleans.
//
// Examples:
//
//	FlagSpec{Name:"--flash-attn", ValueType:ValueTypeBoolOnOff}.Emit("on")
//	  → ["--flash-attn", "on"]
//
//	FlagSpec{Name:"-fa", ValueType:ValueTypeBoolImplicit}.Emit("")
//	  → ["-fa"]
//
//	FlagSpec{Name:"--ctx-size", ValueType:ValueTypeInt}.Emit("4096")
//	  → ["--ctx-size", "4096"]
func (f FlagSpec) Emit(value string) []string {
	if !f.TakesValue || f.ValueType == ValueTypeBoolImplicit {
		return []string{f.Name}
	}
	return []string{f.Name, value}
}

// ─── CapabilitySet ────────────────────────────────────────────────────────────

// CapabilitySet is what a specific engine binary/image actually supports
// right now. It is always produced by probing the real binary — never
// constructed manually in production code.
//
// Consumers use HasFeature / FlagFor / RequireFeatures to build flag lists.
// Direct access to the raw flags and features maps is intentionally unexported
// to enforce the semantic feature API as the stable surface.
type CapabilitySet struct {
	// EngineName is the self-reported engine name (e.g. "llama-server").
	EngineName string

	// Version is the self-reported version string (e.g. "b3901").
	// May be empty if the binary does not report its version.
	Version string

	// flags is the raw flag set parsed from --help output.
	// Keyed by exact flag string (e.g. "--flash-attn", "-fa").
	flags map[string]FlagSpec

	// features maps semantic feature names to whether they are supported.
	// e.g. "flash_attn" → true
	// This is the stable API that BuildArgs() consumers use.
	features map[string]bool

	// featureFlags maps semantic feature names to their concrete FlagSpec.
	// Populated by BuildCapabilities alongside features.
	featureFlags map[string]FlagSpec
}

// HasFeature returns true if the named semantic feature is supported by this
// engine binary. Feature names are snake_case constants, e.g. "flash_attn".
func (c *CapabilitySet) HasFeature(name string) bool {
	if c == nil {
		return false
	}
	return c.features[name]
}

// validFeatureRe is used to validate a semantic feature name before synthesizing
// a fallback flag (SEC-17 / L-4). Feature names are always snake_case alphanumeric.
var validFeatureRe = regexp.MustCompile(`^[a-z0-9_]+$`)

// FlagFor returns the FlagSpec for the current flag name of a semantic feature.
// Returns (FlagSpec{}, false) if the feature is not supported or has no flag.
func (c *CapabilitySet) FlagFor(feature string) (FlagSpec, bool) {
	if c == nil {
		// SEC-17 (L-4): Synthesize a best-effort flag for dry-run display only
		// if the feature name is valid, preventing malformed flags.
		if !validFeatureRe.MatchString(feature) {
			return FlagSpec{}, false
		}
		flag := "--" + strings.ReplaceAll(feature, "_", "-")
		return FlagSpec{Name: flag, TakesValue: true}, true
	}
	f, ok := c.featureFlags[feature]
	return f, ok
}

// HasRawFlag returns true if the raw flag string (e.g. "--flash-attn") exists
// in the --help output. Use HasFeature for semantic checks; HasRawFlag is for
// engine-internal use during capability building.
func (c *CapabilitySet) HasRawFlag(flag string) bool {
	if c == nil {
		return false
	}
	_, ok := c.flags[flag]
	return ok
}

// RawFlag returns the FlagSpec for a raw flag string. Returns (FlagSpec{}, false)
// if the flag does not exist. Use FlagFor for semantic feature access.
func (c *CapabilitySet) RawFlag(flag string) (FlagSpec, bool) {
	if c == nil {
		return FlagSpec{}, false
	}
	f, ok := c.flags[flag]
	return f, ok
}

// RequireFeatures returns a structured error if any named feature is not
// supported by this engine. Use at the top of BuildArgs() to fail fast
// before any flags are constructed.
//
// The error message names all missing features at once (not just the first),
// so users see the complete picture in one shot.
func (c *CapabilitySet) RequireFeatures(features ...string) error {
	if c == nil {
		return nil // Bypass validation if capabilities are unknown (e.g. dry-run)
	}
	// PERF-24 (PL-4): Pre-allocate slice capacity to avoid reallocation
	missing := make([]string, 0, len(features))
	for _, f := range features {
		if !c.features[f] {
			missing = append(missing, f)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf(
		"engine %q (version %q) is missing required features: %s\n"+
			"  Upgrade %s or remove these features from the recipe.",
		c.EngineName, c.Version,
		strings.Join(missing, ", "),
		c.EngineName,
	)
}

// ─── LaunchConfig ─────────────────────────────────────────────────────────────

// LaunchConfig carries all parameters needed to construct the engine subprocess.
// It is passed to Engine.NewSupervisor() after BuildArgs() has resolved the flags.
type LaunchConfig struct {
	// ModelPath is the absolute local path to the model file or directory.
	ModelPath string

	// Flags is the ordered list of engine-specific CLI flags produced by BuildArgs().
	Flags []string

	// EnvVars are additional environment variables to inject into the subprocess
	// (from recipe.pre_run.env).
	EnvVars map[string]string

	// Port is the resolved server port.
	Port int

	// Recipe is the full parsed recipe. Needed by Docker-based engines to build
	// the container name and volume mount path. May be nil for non-Docker engines.
	Recipe *recipe.Recipe

	// LogWriter is an open file for engine log output (Phase 1 Fix 4).
	// If non-nil, the Supervisor writes stdout/stderr here instead of a temp file.
	LogWriter interface{ Write([]byte) (int, error) }

	// Silent suppresses TTY output (used when the TUI is active).
	Silent bool
}

// ─── Engine interface ─────────────────────────────────────────────────────────

// Engine is the contract every inference backend must implement.
// Each engine package (llamacpp, vllm, sglang, docker) provides exactly one
// implementation. The orchestration layer (pipeline, cmd/run.go) depends only
// on this interface — no engine-specific logic leaks upward.
type Engine interface {
	// Name returns a human-readable label for CLI output.
	// e.g. "llama-server", "vLLM 0.9.1 (native)", "vLLM Docker (vllm/vllm-openai:v0.9.0)"
	Name() string

	// Capabilities probes the engine and returns what it currently supports.
	// The result is internally cached (sync.Once) — safe to call multiple times.
	// ctx is passed so the probe can respect cancellation/timeout.
	Capabilities(ctx context.Context) (*CapabilitySet, error)

	// BuildArgs converts an EngineConfig into the ordered list of CLI flags,
	// consulting caps for the correct flag name/syntax for this binary version.
	//
	// Returns a structured error if cfg requires a capability not in caps,
	// so the user sees what is missing before the engine is ever launched.
	BuildArgs(caps *CapabilitySet, cfg recipe.EngineConfig) ([]string, error)

	// NewSupervisor constructs a process.Supervisor configured to launch this
	// engine with the given LaunchConfig. The Supervisor handles process exec,
	// stdout/stderr fan-out, signal handling, and readiness polling.
	//
	// The caller is responsible for calling Supervisor.Run(ctx).
	NewSupervisor(cfg LaunchConfig) (*process.Supervisor, error)

	// OfferInstall prompts/attempts installation of the missing engine.
	// Returns true if the engine is available after the call.
	// Called when Capabilities() returns an error indicating the binary is absent.
	OfferInstall() bool
}

// DefaultCacheDir returns the platform cache directory for Bloc
// (~/.cache/bloc on Linux/macOS). Exported for use by engine subpackages.
// PERF-27 (PL-2, PL-7): Consolidated from vllm and docker packages.
func DefaultCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// PL-2: Use filepath.Join instead of string concat
	return filepath.Join(home, ".cache", "bloc"), nil
}

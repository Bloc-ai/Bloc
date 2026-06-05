package pipeline

import (
	"os"
	"time"

	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/hardware"
	"github.com/bloc-org/bloc/internal/process"
	"github.com/bloc-org/bloc/internal/recipe"
)

// RunState carries all data between pipeline stages.
// Stages read from and write to this struct. Fields are populated progressively
// as stages run — earlier stages guarantee later stages have what they need.
//
// Zero-value is safe: all fields are set by the relevant stage.
type RunState struct {
	// ── Inputs (set by cmd/run.go before pipeline.Execute) ───────────────────

	// RecipeID is the raw argument from the CLI (author/name or local path).
	RecipeID string

	// IsDryRun — print the server command without launching it.
	IsDryRun bool

	// IsYes — auto-confirm all prompts (--yes flag).
	IsYes bool

	// RuntimeOverride — overrides recipe.Engine.Runtime (--runtime flag).
	RuntimeOverride string

	// APIBase is the Hub API base URL (injected so stages don't import cmd/).
	APIBase string

	// CacheDir is the local ~/.cache/bloc path (injected so stages can be tested
	// without touching the real filesystem).
	CacheDir string

	// NoTelemetry disables anonymous benchmark sharing for this run.
	NoTelemetry bool

	// ── Populated by FetchRecipeStage ──────────────────────────────────────────

	// Recipe is the fully parsed and validated recipe.
	Recipe *recipe.Recipe

	// IsLocal is true when RecipeID resolved to a local YAML file.
	IsLocal bool

	// ── Populated by ResolveEngineStage ───────────────────────────────────────

	// Engine is the resolved engine implementation.
	Engine engine.Engine

	// ── Populated by HardwareProbeStage ───────────────────────────────────────

	// Hardware holds the probed system info.
	// Nil if the probe failed (non-fatal — pipeline continues).
	Hardware *hardware.SystemInfo

	// ── Populated by CapabilityProbeStage ────────────────────────────────────

	// Caps holds the engine capability set returned by Engine.Capabilities().
	Caps *engine.CapabilitySet

	// ── Populated by DownloadModelStage ──────────────────────────────────────

	// ModelPath is the absolute local path to the downloaded (or cached) model.
	ModelPath string

	// ── Populated by BuildFlagsStage ────────────────────────────────────────

	// Flags is the ordered list of engine CLI flags built from the recipe config.
	Flags []string

	// ── Populated by LaunchStage ─────────────────────────────────────────────

	// LogFile is the open engine log file. Closed by LaunchStage after the
	// engine exits.
	LogFile *os.File

	// LogPath is the absolute path to the engine log file. Printed to stderr
	// after the run so users can find it for debugging.
	LogPath string

	// Port is the resolved server port (recipe.EngineConfig.Port or default).
	Port int

	// Stats holds the final performance metrics from the engine run.
	Stats *process.Stats

	// StartTime is recorded by LaunchStage to compute session duration.
	StartTime time.Time
}

// EngineName returns the canonical engine name from the recipe, or "llama.cpp"
// when the recipe doesn't specify one (backward-compatible default).
func (s *RunState) EngineName() string {
	if s.Recipe == nil {
		return "llama.cpp"
	}
	name := s.Recipe.Engine.Name
	if name == "" {
		return "llama.cpp"
	}
	return name
}

// ResolvedPort returns the server port, defaulting to 8080 if unset.
func (s *RunState) ResolvedPort() int {
	if s.Recipe == nil || s.Recipe.EngineConfig.Port == 0 {
		return 8080
	}
	return s.Recipe.EngineConfig.Port
}

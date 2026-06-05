// Package pipeline implements the Bloc run pipeline — a linear sequence of
// stages that transforms a recipe ID into a running engine + TUI.
//
// Each stage is a small, individually-testable unit. RunState carries all data
// between stages so every stage receives exactly what it needs and no more.
//
// Dependency diagram:
//
//	cmd/run.go → pipeline.New(...stages) → pipeline.Execute(ctx, state)
//	             ↓
//	             FetchRecipeStage
//	             ResolveEngineStage
//	             HardwareProbeStage
//	             CapabilityProbeStage
//	             DownloadModelStage
//	             PreRunStage
//	             SecurityGateStage
//	             BuildFlagsStage
//	             LaunchStage
package pipeline

import (
	"context"
	"fmt"
	"os"
)

// ─── Stage interface ──────────────────────────────────────────────────────────

// Stage is a single step in the run pipeline.
// Each Stage has a name (used in printStep output) and a Run method.
// If Run returns a non-nil error, the pipeline stops immediately.
type Stage interface {
	// Name returns a human-readable label for this step.
	// Printed as "[N] <Name>" before the stage executes.
	Name() string

	// Run executes this stage, reading from and writing to state.
	// ctx carries the cancellation signal from the top-level signal handler.
	Run(ctx context.Context, state *RunState) error
}

// ─── Pipeline ─────────────────────────────────────────────────────────────────

// Pipeline is an ordered sequence of Stages. It executes them in order,
// stopping at the first error.
type Pipeline struct {
	stages []Stage
	stepN  int // current step counter for display
}

// New creates a Pipeline with the given stages in order.
func New(stages ...Stage) *Pipeline {
	return &Pipeline{stages: stages}
}

// Execute runs all stages in order. Stops and returns the first error.
// stepN is reset per Execute call so the pipeline can be re-used in tests.
func (p *Pipeline) Execute(ctx context.Context, state *RunState) error {
	p.stepN = 0
	for _, stage := range p.stages {
		p.printStep(stage.Name())
		if err := stage.Run(ctx, state); err != nil {
			return fmt.Errorf("[%s] %w", stage.Name(), err)
		}
		// Honour context cancellation between stages (e.g. Ctrl+C during download).
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// printStep increments the step counter and prints the stage header.
func (p *Pipeline) printStep(label string) {
	p.stepN++
	fmt.Fprintf(os.Stderr, "\n\033[1m[%d] %s\033[0m\n", p.stepN, label)
}

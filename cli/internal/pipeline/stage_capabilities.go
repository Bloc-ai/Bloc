package pipeline

import (
	"context"
	"fmt"
	"os"
)

// CapabilityProbeStage checks that the resolved engine binary supports
// all flags required by the recipe. Skipped in --dry-run mode.
//
// If the binary is not found, OfferInstall() is called.
// If after install the binary is still missing, the pipeline fails.
// If the binary is present but missing required flags, the pipeline fails
// with a clear message listing what is absent.
//
// Sets: state.Caps
type CapabilityProbeStage struct{}

func (s *CapabilityProbeStage) Name() string {
	return "Checking engine capabilities"
}

func (s *CapabilityProbeStage) Run(ctx context.Context, state *RunState) error {
	eng := state.Engine

	if state.IsDryRun {
		// In dry-run mode we skip capability probing — BuildArgs will still
		// work because engine.BuildArgs() is called with a nil caps for engines
		// that don't require capability checking (vllm, sglang).
		// For llama.cpp, BuildArgs with nil caps is safe (it returns empty flags).
		state.Caps = nil
		return nil
	}

	caps, err := eng.Capabilities(ctx)
	if err != nil {
		// Binary not found or probe failed — offer to install.
		fmt.Fprintf(os.Stderr, "\n\033[31m✗  %s not found\033[0m\n", eng.Name())
		if eng.OfferInstall() {
			// Re-probe after successful install.
			caps, err = eng.Capabilities(ctx)
			if err != nil {
				return fmt.Errorf("%s still unavailable after install: %w", eng.Name(), err)
			}
		} else {
			return fmt.Errorf("%s is required but not installed", eng.Name())
		}
	}

	state.Caps = caps
	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  %s — capabilities verified\n", eng.Name())
	return nil
}

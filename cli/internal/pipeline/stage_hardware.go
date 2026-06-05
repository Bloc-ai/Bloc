package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/bloc-org/bloc/internal/hardware"
)

// HardwareProbeStage probes the system for GPU/CPU capabilities.
// Failure is non-fatal — the pipeline continues with state.Hardware = nil.
// If VRAM is insufficient for the recipe, the user is warned and asked to
// confirm (unless --yes was passed or it's a dry run).
//
// Sets: state.Hardware (may remain nil on probe failure)
type HardwareProbeStage struct{}

func (s *HardwareProbeStage) Name() string { return "Probing hardware" }

func (s *HardwareProbeStage) Run(_ context.Context, state *RunState) error {
	hw, err := hardware.Probe()
	if err != nil {
		// Non-fatal: warn and proceed. The engine will fail more clearly if
		// hardware is truly incompatible.
		fmt.Fprintf(os.Stderr, "  ⚠  Could not probe hardware: %v\n", err)
		return nil
	}

	state.Hardware = hw
	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  %s\n", hw.Summary())

	// VRAM check — warn the user if the model requires more VRAM than available.
	ok, detectedGB, requiredGB := hw.CheckVRAMRequirement(state.Recipe.Hardware.MinVRAM)
	if !ok {
		fmt.Fprintf(os.Stderr, "\n  \033[33m⚠  VRAM warning:\033[0m This recipe requires %.0f GB VRAM.\n", requiredGB)
		fmt.Fprintf(os.Stderr, "     Your system has %.1f GB available.\n", detectedGB)
		if state.IsDryRun {
			fmt.Fprintln(os.Stderr, "     [Dry Run] Proceeding with dry-run command display.")
			return nil
		}
		if !confirmPrompt("     Continue anyway? [y/N]: ", state.IsYes) {
			return fmt.Errorf("aborted by user (insufficient VRAM)")
		}
	}

	return nil
}

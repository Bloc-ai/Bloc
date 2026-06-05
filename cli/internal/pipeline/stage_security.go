package pipeline

import (
	"context"
	"fmt"
	"os"
)

// SecurityGateStage implements the trust_remote_code confirmation gate (F-19).
// It is only relevant for vLLM recipes that set trust_remote_code: true.
//
// The gate requires an EXPLICIT "y" — pressing Enter alone is treated as "no"
// (unlike normal confirmations where Enter = yes). This is intentional:
// executing arbitrary Python code from a model repository is a high-risk action.
//
// Skipped in --dry-run mode and for non-vLLM engines.
type SecurityGateStage struct{}

func (s *SecurityGateStage) Name() string { return "Security confirmation" }

func (s *SecurityGateStage) Run(_ context.Context, state *RunState) error {
	if !state.Recipe.EngineConfig.TrustRemoteCode {
		return nil // fast path: most recipes don't set this
	}
	if state.EngineName() != "vllm" {
		return nil // only vLLM executes Python model code
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  \033[33m⚠  This recipe sets trust_remote_code: true\033[0m")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  This allows vLLM to execute custom Python code bundled with the model.")
	fmt.Fprintln(os.Stderr, "  Only proceed if you trust the model author and have reviewed the code at:")
	fmt.Fprintf(os.Stderr, "  \033[36mhttps://huggingface.co/%s/tree/main\033[0m\n", state.Recipe.Model.HFRepo)
	fmt.Fprintln(os.Stderr)

	if state.IsDryRun {
		fmt.Fprintln(os.Stderr, "  [Dry Run] Skipping trust_remote_code confirmation.")
		return nil
	}

	// confirmYesExplicit: Enter alone = "no". Must type "y" or "yes".
	if !confirmYesExplicit("  Allow execution of custom model code? [y/N]: ", state.IsYes) {
		return fmt.Errorf("trust_remote_code rejected by user — aborting")
	}

	return nil
}

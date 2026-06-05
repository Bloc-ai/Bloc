package pipeline

import (
	"context"
	"fmt"
	"os"
)

// BuildFlagsStage converts the recipe EngineConfig into the ordered list of
// engine CLI flags by delegating to Engine.BuildArgs(). This replaces the old
// engine-name switch that called recipe-level BuildVLLMFlags/BuildSGLangFlags.
//
// Also injects CUDA_VISIBLE_DEVICES into PreRun.Env for SGLang (the Docker
// runtime passes it as -e CUDA_VISIBLE_DEVICES=... to docker run).
//
// In --dry-run mode this stage runs normally and then DryRunStage prints the
// resolved command. The flags are still built so the dry-run output is accurate.
//
// Sets: state.Flags
type BuildFlagsStage struct{}

func (s *BuildFlagsStage) Name() string { return "Building engine flags" }

func (s *BuildFlagsStage) Run(_ context.Context, state *RunState) error {
	r := state.Recipe
	eng := state.Engine

	flags, err := eng.BuildArgs(state.Caps, r.EngineConfig)
	if err != nil {
		return fmt.Errorf("cannot build engine flags: %w", err)
	}

	// Inject CUDA device pinning into the env map so the Docker runtime
	// can pass it as -e CUDA_VISIBLE_DEVICES=... to docker run.
	if devs := r.EngineConfig.SGLangCUDAVisibleDevices; devs != "" {
		if r.PreRun.Env == nil {
			r.PreRun.Env = make(map[string]string)
		}
		r.PreRun.Env["CUDA_VISIBLE_DEVICES"] = devs
	}

	state.Flags = flags

	if state.IsDryRun {
		printDryRun(state)
		return errDryRunDone
	}

	return nil
}

// errDryRunDone is a sentinel returned by BuildFlagsStage in --dry-run mode.
// pipeline.Execute treats this as a clean stop (not an error). run.go checks
// for it specifically and returns nil to the user.
var errDryRunDone = &dryRunSentinel{}

type dryRunSentinel struct{}

func (e *dryRunSentinel) Error() string { return "dry-run complete" }

// IsDryRunDone returns true when the pipeline stopped due to --dry-run.
// cmd/run.go uses this to distinguish a clean dry-run stop from a real error.
func IsDryRunDone(err error) bool {
	_, ok := err.(*dryRunSentinel)
	return ok
}

// printDryRun prints the resolved engine command in a human-readable format.
func printDryRun(state *RunState) {
	eng := state.Engine
	engineName := state.EngineName()
	flags := state.Flags
	modelPath := state.ModelPath

	fmt.Fprintf(os.Stderr, "\n\033[36m── Dry run: %s command ──────────────────────────────────────────\033[0m\n", eng.Name())

	switch engineName {
	case "vllm":
		fmt.Fprintln(os.Stderr, "python3 -m vllm.entrypoints.openai.api_server \\")
		fmt.Fprintf(os.Stderr, "  --model %s \\\n", modelPath)
	case "sglang":
		fmt.Fprintln(os.Stderr, "python3 -m sglang.launch_server \\")
		fmt.Fprintf(os.Stderr, "  --model-path %s \\\n", modelPath)
		fmt.Fprintln(os.Stderr, "  --host 0.0.0.0 \\")
		fmt.Fprintf(os.Stderr, "  --port %d \\\n", state.Recipe.EngineConfig.Port)
	default:
		fmt.Fprintf(os.Stderr, "%s -m %s \\\n", eng.Name(), modelPath)
	}

	for i, f := range flags {
		if i < len(flags)-1 {
			fmt.Fprintf(os.Stderr, "  %s \\\n", f)
		} else {
			fmt.Fprintf(os.Stderr, "  %s\n", f)
		}
	}
}

package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/bloc-org/bloc/internal/engine"
	_ "github.com/bloc-org/bloc/internal/engine/llamacpp"
	_ "github.com/bloc-org/bloc/internal/engine/sglang"
	_ "github.com/bloc-org/bloc/internal/engine/vllm"
)

// ResolveEngineStage resolves the engine implementation for the recipe using
// the engine registry. Blank imports above trigger init() registration of all
// built-in engines (llama.cpp, vllm, sglang).
//
// Sets: state.Engine
type ResolveEngineStage struct{}

func (s *ResolveEngineStage) Name() string { return "Resolving engine" }

func (s *ResolveEngineStage) Run(_ context.Context, state *RunState) error {
	// Apply --runtime override to the recipe before resolving.
	if state.RuntimeOverride != "" {
		state.Recipe.Engine.Runtime = state.RuntimeOverride
	}

	eng, err := engine.Resolve(state.Recipe)
	if err != nil {
		return fmt.Errorf("cannot resolve engine: %w", err)
	}
	state.Engine = eng
	fmt.Fprintf(os.Stderr, "  \033[32m✓\033[0m  Engine: %s\n", eng.Name())
	return nil
}

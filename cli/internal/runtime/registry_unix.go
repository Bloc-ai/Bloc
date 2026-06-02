//go:build !windows

package runtime

import (
	"fmt"

	"github.com/bloc-org/bloc/internal/recipe"
)

// Resolve returns the correct Runtime implementation for the given recipe.
// This is the single dispatch point — cmd/run.go calls Resolve() and
// then interacts only with the Runtime interface, never with concrete types.
//
// The runtimeOverride parameter maps to the --runtime CLI flag.
// When non-empty, it overrides the runtime declared in recipe.Engine.Runtime.
func Resolve(r *recipe.Recipe, runtimeOverride string) (Runtime, error) {
	engineName := r.Engine.Name
	if engineName == "" {
		engineName = "llama.cpp" // zero-value default
	}

	engineRuntime := r.Engine.Runtime
	if runtimeOverride != "" {
		engineRuntime = runtimeOverride
	}
	if engineRuntime == "" {
		engineRuntime = "native" // default for all engines
	}

	switch engineName {
	case "llama.cpp", "llama-cpp":
		return &LlamaCppRuntime{}, nil

	case "vllm":
		// Resolve version once here so Name(), Probe(), and Run() all use
		// the same pinned string throughout the run lifecycle.
		version := resolveVLLMVersion(r.Engine.Version)

		switch engineRuntime {
		case "native", "":
			return &NativeVLLMRuntime{version: version}, nil

		case "docker":
			// F-15: image tag format validated at recipe.Parse() time
			// (^[a-z0-9][a-z0-9/_:.\-]{0,199}$). Empty check here is a belt-and-
			// suspenders guard in case Resolve() is called with a pre-parsed recipe
			// that somehow bypassed Parse() (e.g. unit tests constructing Recipe{} directly).
			image := r.Engine.Image
			if image == "" {
				return nil, fmt.Errorf(
					"engine.image is required when runtime=docker\n" +
						"  Example: image: vllm/vllm-openai:v0.9.0",
				)
			}
			return &DockerVLLMRuntime{image: image}, nil

		default:
			return nil, fmt.Errorf("unknown runtime %q for engine %q — valid options: native, docker", engineRuntime, engineName)
		}

	case "sglang":
		// SGLang is Docker-only in Bloc v1.
		// Native mode is explicitly unsupported: it requires CUDA toolkit and
		// Python virtualenv management that exceeds the scope of a local dev
		// tool. SGLang is primarily a multi-GPU CUDA workload.
		switch engineRuntime {
		case "native":
			return nil, fmt.Errorf(
				"engine=sglang does not support runtime=native in Bloc\n" +
					"  SGLang requires a multi-GPU CUDA environment best managed via Docker.\n" +
					"  Remove 'runtime: native' from your recipe, or set 'runtime: docker'.",
			)

		case "docker", "":
			// F-15: image tag format validated at recipe.Parse() time.
			// Belt-and-suspenders empty check for recipes bypassing Parse().
			image := r.Engine.Image
			if image == "" {
				return nil, fmt.Errorf(
					"engine.image is required for the sglang engine\n" +
						"  Example: image: lmsysorg/sglang:v0.5.12.post1",
				)
			}
			return &SGLangDockerRuntime{image: image}, nil

		default:
			return nil, fmt.Errorf("unknown runtime %q for engine %q — valid option: docker", engineRuntime, engineName)
		}

	default:
		return nil, fmt.Errorf("unsupported engine %q — supported engines: llama.cpp, vllm, sglang", engineName)
	}
}

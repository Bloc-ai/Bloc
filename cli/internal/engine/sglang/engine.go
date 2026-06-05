package sglang

import (
	"fmt"
	"strconv"

	"github.com/bloc-org/bloc/internal/engine"
	"github.com/bloc-org/bloc/internal/engine/docker"
	"github.com/bloc-org/bloc/internal/recipe"
)

// New creates a new SGLang engine.
// SGLang is strictly Docker-only.
func New(r *recipe.Recipe) (engine.Engine, error) {
	if r.Engine.Runtime != "docker" {
		return nil, fmt.Errorf("engine.runtime 'native' is not supported for sglang. SGLang requires runtime 'docker'")
	}
	if r.Engine.Image == "" {
		return nil, fmt.Errorf("engine.image is required for sglang docker runtime")
	}

	entryCmd := func(modelPath string, port int) []string {
		// Modern SGLang entrypoint: `sglang serve <model_path>`
		return []string{"sglang", "serve", modelPath, "--host", "0.0.0.0", "--port", strconv.Itoa(port)}
	}

	return &docker.DockerEngine{
		Image:       r.Engine.Image,
		EntryCmd:    entryCmd,
		FlagBuilder: BuildFlags,
		Parser:      &SGLangLogParser{},
		DisplayName: fmt.Sprintf("SGLang Docker (%s)", r.Engine.Image),
	}, nil
}

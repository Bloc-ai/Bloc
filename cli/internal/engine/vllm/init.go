package vllm

import "github.com/bloc-org/bloc/internal/engine"

func init() {
	engine.Register("vllm", New)
}

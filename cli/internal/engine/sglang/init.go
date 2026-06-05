package sglang

import "github.com/bloc-org/bloc/internal/engine"

func init() {
	engine.Register("sglang", New)
}

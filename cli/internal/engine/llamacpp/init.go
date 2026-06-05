package llamacpp

import "github.com/bloc-org/bloc/internal/engine"

// init registers the llamacpp engine under both canonical names so recipes
// using either "llama.cpp" or "llama-cpp" are routed here.
// The normalisation to "llama.cpp" happens in engine.normalizeEngineName;
// registering both aliases here is belt-and-suspenders for direct calls.
//
// Called automatically by the Go runtime when this package is imported.
// The pipeline imports _ "github.com/bloc-org/bloc/internal/engine/llamacpp"
// (blank import) in stage_resolve.go to trigger registration without a direct
// dependency on the concrete type.
func init() {
	engine.Register("llama.cpp", New)
	engine.Register("llama-cpp", New)
}

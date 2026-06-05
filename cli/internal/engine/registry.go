package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/bloc-org/bloc/internal/recipe"
)

// ─── Registry ─────────────────────────────────────────────────────────────────

// Factory is a constructor function that builds an Engine for a given recipe.
// Registered factories receive the full recipe so they can extract image tags,
// version pins, runtime preferences, etc. at construction time.
type Factory func(r *recipe.Recipe) (Engine, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register makes an engine available under name.
// Convention: name is lowercase, matching recipe engine.name values
// (e.g. "llama.cpp", "vllm", "sglang").
//
// Called from each engine package's init() function:
//
//	func init() { engine.Register("llama.cpp", llamacpp.New) }
//
// Panics on duplicate registration — this is intentional, because duplicates
// indicate a programming error (two packages registering the same name) that
// should be caught immediately at startup, not silently ignored.
func Register(name string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("engine: duplicate registration for %q", name))
	}
	registry[name] = f
}

// Resolve returns the Engine for the given recipe.
//
// Resolution order:
//  1. recipe.Engine.Name (canonical engine name, e.g. "llama.cpp")
//  2. Aliases: "llama-cpp" → "llama.cpp"
//  3. Zero value → "llama.cpp" (backward-compatible default)
//
// Returns a descriptive error listing all registered engine names if the
// recipe requests an unknown engine.
func Resolve(r *recipe.Recipe) (Engine, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	name := normalizeEngineName(r.Engine.Name)

	f, ok := registry[name]
	if !ok {
		var registered []string
		for k := range registry {
			registered = append(registered, k)
		}
		sort.Strings(registered)
		return nil, fmt.Errorf(
			"unsupported engine %q — registered engines: %s\n"+
				"  Check the 'engine.name' field in your recipe YAML.",
			name,
			strings.Join(registered, ", "),
		)
	}
	return f(r)
}


// RegisteredEngines returns the sorted list of all registered engine names.
// Used for error messages and help text.
func RegisteredEngines() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// normalizeEngineName maps aliases and empty values to canonical engine names.
// All new aliases must be added here — nowhere else.
func normalizeEngineName(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "llama.cpp", "llama-cpp":
		return "llama.cpp"
	case "vllm":
		return "vllm"
	case "sglang":
		return "sglang"
	default:
		return raw // pass through unknown names so Resolve() can report them
	}
}

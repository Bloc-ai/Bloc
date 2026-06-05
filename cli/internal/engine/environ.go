package engine

import (
	"os"
	"strings"
)

// dangerousEnvVars is the set of environment variable names that can hijack
// the dynamic linker or interpreter before user code runs.
// M-2: Strip these from the environment passed to engine subprocesses to
// prevent the user's shell environment from interfering with inference engines.
var dangerousEnvVars = map[string]bool{
	// Linux dynamic linker hooks
	"LD_PRELOAD":      true,
	"LD_LIBRARY_PATH": true,
	"LD_AUDIT":        true,
	"LD_DEBUG":        true,
	// macOS dynamic linker hooks
	"DYLD_INSERT_LIBRARIES": true,
	"DYLD_LIBRARY_PATH":     true,
	"DYLD_FRAMEWORK_PATH":   true,
	"DYLD_FORCE_FLAT_NAMESPACE": true,
	// Python interpreter hijack
	"PYTHONPATH":    true,
	"PYTHONSTARTUP": true,
	// Shell startup file injection
	"BASH_ENV": true,
	"ENV":      true,
	"CDPATH":   true,
	// Node / Perl / Ruby interpreter hijack
	"NODE_OPTIONS": true,
	"PERL5OPT":     true,
	"RUBYOPT":      true,
}

// SafeEnviron returns the host process environment (os.Environ()) with all
// dangerous loader and interpreter hijack variables removed.
//
// This is used by native engine launchers (llama.cpp, vLLM) to inherit the
// useful parts of the user's environment (PATH, HOME, CUDA_VISIBLE_DEVICES,
// HF_HOME, etc.) while blocking loader injection attacks such as LD_PRELOAD.
func SafeEnviron() []string {
	raw := os.Environ()
	safe := make([]string, 0, len(raw))
	for _, kv := range raw {
		// Split only on the first '=' — values may contain '='.
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			safe = append(safe, kv)
			continue
		}
		key := kv[:idx]
		if !dangerousEnvVars[key] {
			safe = append(safe, kv)
		}
	}
	return safe
}

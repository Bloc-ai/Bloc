package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// preRunCmdAllowedRe is the strict allowlist for pre-run commands.
// SEC-02 (H-3): We cannot parse shell syntax in Go, so we only allow
// alphanumeric characters, spaces, dashes, underscores, dots, and slashes.
// This prevents bypassing a blocklist using exotic whitespace, unicode
// homoglyphs, or unhandled shell metacharacters.
// Authors who need complex commands must use a script file.
var preRunCmdAllowedRe = regexp.MustCompile(`^[A-Za-z0-9_.\-/ ]+$`)

// PreRunStage executes recipe.pre_run.commands with user confirmation.
// Commands are validated against the strict allowlist before display
// or execution. Skipped entirely if the recipe has no pre-run commands.
//
// The stage is skipped (with a dry-run notice) when state.IsDryRun is true.
// All commands are rejected without confirmation on a non-TTY unless --yes is set.
type PreRunStage struct{}

func (s *PreRunStage) Name() string { return "Pre-run setup" }

func (s *PreRunStage) Run(_ context.Context, state *RunState) error {
	cmds := state.Recipe.PreRun.Commands
	if len(cmds) == 0 {
		// Nothing to do — print nothing and move on silently.
		return nil
	}

	// Validate all commands before displaying any of them.
	for _, c := range cmds {
		if !preRunCmdAllowedRe.MatchString(c) {
			return fmt.Errorf(
				"pre-run command %q contains forbidden characters — only alphanumeric, space, dash, underscore, dot, and slash are allowed. Use a script file for complex commands", c)
		}
	}

	fmt.Fprintln(os.Stderr, "  This recipe will execute the following commands before starting:")
	for _, c := range cmds {
		fmt.Fprintf(os.Stderr, "    \033[33m%s\033[0m\n", c)
	}

	if state.IsDryRun {
		fmt.Fprintln(os.Stderr, "  [Dry Run] Skipping pre-run command execution.")
		return nil
	}

	if !confirmPrompt("  Allow? [Y/n]: ", state.IsYes) {
		return fmt.Errorf("pre-run commands rejected by user")
	}

	for _, c := range cmds {
		if err := runShellCommand(c, state.Recipe.PreRun.Env); err != nil {
			return fmt.Errorf("pre-run command failed: %w", err)
		}
	}

	return nil
}

// dangerousEnvKeys lists environment variable names that can be exploited to
// hijack dynamic linker loading or interpreter startup before user code runs.
// These are stripped from the safe environment even if the parent shell has them.
// SEC-03: Blocks LD_PRELOAD / DYLD_INSERT_LIBRARIES / PYTHONPATH attacks.
var dangerousEnvKeys = map[string]bool{
	"LD_PRELOAD":              true,
	"LD_LIBRARY_PATH":         true,
	"LD_AUDIT":                true,
	"LD_DEBUG":                true,
	"DYLD_INSERT_LIBRARIES":   true,
	"DYLD_LIBRARY_PATH":       true,
	"DYLD_FRAMEWORK_PATH":     true,
	"PYTHONPATH":              true,
	"PYTHONSTARTUP":           true,
	"RUBYOPT":                 true,
	"BASH_ENV":                true,
	"ENV":                     true,
	"CDPATH":                  true,
	"NODE_OPTIONS":            true,
	"PERL5OPT":                true,
}

// envKeyRe validates pre_run.env key names.
// SEC-03: Only [A-Za-z_][A-Za-z0-9_]* names are allowed.
// Rejects keys with = signs, spaces, or special characters that
// could corrupt the environment array.
var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// runShellCommand executes a single shell command token-split by whitespace.
// env is a map of additional KEY=VALUE pairs to inject.
//
// SEC-02: No shell is invoked — exec.Command splits on whitespace and runs directly.
// SEC-03: A minimal, safe environment is constructed rather than inheriting
// os.Environ(). Dynamic linker and interpreter hijack variables are stripped.
func runShellCommand(command string, env map[string]string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}
	bin, err := exec.LookPath(parts[0])
	if err != nil {
		return fmt.Errorf("command %q not found: %w", parts[0], err)
	}

	// Validate all recipe-supplied env keys before touching the environment.
	for k := range env {
		if !envKeyRe.MatchString(k) {
			return fmt.Errorf("pre_run.env key %q is invalid: must match [A-Za-z_][A-Za-z0-9_]*", k)
		}
		if dangerousEnvKeys[k] {
			return fmt.Errorf("pre_run.env key %q is not allowed for security reasons", k)
		}
	}

	// Build a safe environment: inherit only safe variables from the parent,
	// stripping any dangerous linker/interpreter hijack variables.
	safeEnv := make([]string, 0, 16+len(env))
	for _, kv := range os.Environ() {
		key := kv
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			key = kv[:idx]
		}
		if !dangerousEnvKeys[key] {
			safeEnv = append(safeEnv, kv)
		}
	}

	// Overlay validated recipe env vars.
	for k, v := range env {
		// Values are not shell-interpreted (direct exec), but strip null bytes
		// and newlines to prevent environment array corruption.
		if strings.ContainsAny(v, "\x00\n\r") {
			return fmt.Errorf("pre_run.env value for %q contains illegal characters (null byte or newline)", k)
		}
		safeEnv = append(safeEnv, k+"="+v)
	}

	cmd := exec.Command(bin, parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = safeEnv

	return cmd.Run()
}

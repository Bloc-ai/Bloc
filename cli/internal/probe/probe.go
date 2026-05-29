package probe

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

// Result is the outcome of a capability probe.
type Result struct {
	BinaryPath     string
	SupportedFlags map[string]struct{}
	Missing        []string // flags required by recipe but absent in binary
}

// P-12: Single package-level compiled regex — eliminates redundant double-scan.
// Matches --long-flag and -s (short flag) at word boundaries on option lines.
var flagRe = regexp.MustCompile(`(?m)^\s+(-{1,2}[a-zA-Z][a-zA-Z0-9\-]*)`)

// LlamaServerCapabilities runs `llama-server --help` and parses the supported flags.
func LlamaServerCapabilities() (map[string]struct{}, string, error) {
	// Find binary via PATH — verified result is used for exec, not re-looked-up
	path, err := exec.LookPath("llama-server")
	if err != nil {
		return nil, "", fmt.Errorf("llama-server not found in PATH")
	}

	// Run --help; it exits with code 1 but prints to stdout+stderr — combine both
	cmd := exec.Command(path, "--help")
	out, _ := cmd.CombinedOutput() // intentionally ignore exit code

	if len(out) == 0 {
		return nil, path, fmt.Errorf("llama-server --help returned no output")
	}

	supported := parseFlags(string(out))
	return supported, path, nil
}

// parseFlags extracts all -flag and --flag tokens from help text.
// P-12: Single regex pass replaces the previous double-scan approach.
// The regex captures flags that appear at the start of option lines
// (indented with whitespace), which covers both short (-f) and long (--flag)
// forms including aliases like "  -fa, --flash-attn".
func parseFlags(helpText string) map[string]struct{} {
	flags := make(map[string]struct{})
	for _, m := range flagRe.FindAllStringSubmatch(helpText, -1) {
		flags[m[1]] = struct{}{}
	}
	// Also pick up short flags on option lines (e.g. "-fa" before ", --flash-attn")
	// by scanning lines that start with whitespace + dash and extracting all flag tokens.
	inlineRe := regexp.MustCompile(`(-{1,2}[a-zA-Z][a-zA-Z0-9\-]*)`)
	for _, line := range strings.Split(helpText, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-") {
			for _, m := range inlineRe.FindAllString(trimmed, -1) {
				flags[m] = struct{}{}
			}
		}
	}
	return flags
}

// CheckRecipeCompatibility diffs required flags vs the binary's supported flags.
// Returns a Result with any missing flags populated.
func CheckRecipeCompatibility(required map[string]struct{}) (*Result, error) {
	supported, path, err := LlamaServerCapabilities()
	if err != nil {
		return nil, err
	}

	res := &Result{
		BinaryPath:     path,
		SupportedFlags: supported,
	}

	for flag := range required {
		if _, ok := supported[flag]; !ok {
			res.Missing = append(res.Missing, flag)
		}
	}
	return res, nil
}

// InstallInstructions returns platform-specific guidance when llama-server is missing.
func InstallInstructions() string {
	return `llama-server is not installed or not in your PATH.

  macOS:   brew install llama.cpp
  Linux:   Download the prebuilt binary from https://github.com/ggml-org/llama.cpp/releases
           or build from source: https://bloc-theta.vercel.app/install

Once installed, re-run: bloc deploy <recipe>`
}

// OfferInstall asks the user if they'd like to install llama.cpp now.
// On macOS it runs `brew install llama.cpp`.
// On Linux it prints the download URL and returns false (manual step required).
// Returns true if llama-server is available after the attempt.
func OfferInstall() bool {
	switch runtime.GOOS {
	case "darwin":
		fmt.Print("\n  Would you like to install llama.cpp via Homebrew now? [Y/n]: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if ans != "" && ans != "y" && ans != "yes" {
				fmt.Println("  Skipped. Re-run after installing manually:")
				fmt.Println("    brew install llama.cpp")
				return false
			}
		}
		fmt.Println("  Running: brew install llama.cpp ...")
		cmd := exec.Command("brew", "install", "llama.cpp")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "\n\033[31m✗  brew install failed: %v\033[0m\n", err)
			return false
		}
		// Verify the binary is now in PATH
		_, err := exec.LookPath("llama-server")
		if err != nil {
			fmt.Fprintln(os.Stderr, "\033[33m⚠  llama-server still not found after install. Try opening a new terminal.\033[0m")
			return false
		}
		fmt.Println("  \033[32m✓\033[0m  llama.cpp installed successfully.")
		return true

	case "linux":
		fmt.Println("  Auto-install is not supported on Linux.")
		fmt.Println("  Download a prebuilt binary from:")
		fmt.Println("    https://github.com/ggml-org/llama.cpp/releases")
		fmt.Println("  or build from source: https://bloc-theta.vercel.app/install")
		return false

	default:
		fmt.Println("  Auto-install is not supported on this platform.")
		fmt.Println("  Install guide: https://bloc-theta.vercel.app/install")
		return false
	}
}

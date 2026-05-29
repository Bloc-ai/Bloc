package runner

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Stats collects runtime performance metrics from llama-server stdout.
type Stats struct {
	TokensPerSecGeneration float64
	TokensPerSecPrefill    float64
	PeakVRAMMB             int64
	Duration               time.Duration
	Success                bool
}

// P-10: Hoist regexes to package level — compiled once at init, not per log line.
// parseStats is called for every line of llama-server output (potentially hundreds/sec).
var (
	genRe    = regexp.MustCompile(`eval time\s*=.*?([\d.]+)\s*tokens per second`)
	promptRe = regexp.MustCompile(`prompt eval time\s*=.*?([\d.]+)\s*tokens per second`)
	vramRe   = regexp.MustCompile(`VRAM\s+USED\s*[=:]\s*([\d.]+)\s*(MB|MIB|GB|GIB)`)
)

// parseStats extracts performance metrics from llama-server log lines.
// P-10: Uses package-level compiled regexes — not re-compiled per call.
// llama-server logs lines like:
//
//	llama_print_timings:        eval time =   234.56 ms /    12 runs   (   19.55 ms per token,    51.15 tokens per second)
//	llama_print_timings: prompt eval time =  1234.56 ms /   256 tokens  (    4.82 ms per token,   207.46 tokens per second)
func parseStats(line string, s *Stats) {
	// Generation speed
	if m := genRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			s.TokensPerSecGeneration = val
		}
	}

	// Prompt processing speed
	if m := promptRe.FindStringSubmatch(line); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			s.TokensPerSecPrefill = val
		}
	}

	// VRAM usage (uppercase for case-insensitive match via ToUpper)
	if m := vramRe.FindStringSubmatch(strings.ToUpper(line)); len(m) > 1 {
		val, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			if m[2] == "GB" || m[2] == "GIB" {
				val *= 1024
			}
			if int64(val) > s.PeakVRAMMB {
				s.PeakVRAMMB = int64(val)
			}
		}
	}
}

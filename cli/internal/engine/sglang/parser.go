package sglang

import (
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/bloc-org/bloc/internal/process"
)

var (
	sglangGenRe     = regexp.MustCompile(`throughput_output_token_per_s=([\d.]+)`)
	sglangPrefillRe = regexp.MustCompile(`throughput_input_token_per_s=([\d.]+)`)
	sglangVRAMRe    = regexp.MustCompile(`(?i)(?:memory pool end size|gpu\s+mem(?:ory)?)\s*[=:]\s*([\d.]+)\s*(GB|MB|MiB|GiB)`)
)

// SGLangLogParser implements process.LogParser for SGLang logs.
// PM-6: peakVRAMMB is mutable state guarded by mu so ParseLine() is safe for
// concurrent callers, as required by the LogParser interface contract.
type SGLangLogParser struct {
	mu         sync.Mutex
	peakVRAMMB int64
}

func (p *SGLangLogParser) ParseLine(line string) process.Metrics {
	var m process.Metrics

	if match := sglangGenRe.FindStringSubmatch(line); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			m.TokensPerSecGen = val
		}
	}
	if match := sglangPrefillRe.FindStringSubmatch(line); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			m.TokensPerSecPrefill = val
		}
	}
	p.mu.Lock()
	if match := sglangVRAMRe.FindStringSubmatch(line); len(match) > 2 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			unit := strings.ToLower(match[2])
			var vram int64
			switch unit {
			case "gb", "gib":
				vram = int64(val * 1024)
			default:
				vram = int64(val)
			}
			if vram > p.peakVRAMMB {
				p.peakVRAMMB = vram
			}
		}
	}
	peak := p.peakVRAMMB
	p.mu.Unlock()
	m.PeakVRAMMB = peak
	return m
}

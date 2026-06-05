package vllm

import (
	"regexp"
	"strconv"
	"sync"

	"github.com/bloc-org/bloc/internal/process"
)

var (
	vllmPromptRe = regexp.MustCompile(`(?i)avg prompt throughput:\s*([\d.]+)\s*tokens/s`)
	vllmGenRe    = regexp.MustCompile(`(?i)avg generation throughput:\s*([\d.]+)\s*tokens/s`)
	vllmKVRe     = regexp.MustCompile(`(?i)gpu kv cache usage:\s*([\d.]+)%`)
)

// VLLMLogParser implements process.LogParser for vLLM logs.
// PM-6: peakVRAMMB is mutable state guarded by mu so ParseLine() is safe for
// concurrent callers, as required by the LogParser interface contract.
type VLLMLogParser struct {
	mu         sync.Mutex
	peakVRAMMB int64
}

func (p *VLLMLogParser) ParseLine(line string) process.Metrics {
	var m process.Metrics

	if match := vllmPromptRe.FindStringSubmatch(line); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			m.TokensPerSecPrefill = val
		}
	}
	if match := vllmGenRe.FindStringSubmatch(line); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			m.TokensPerSecGen = val
		}
	}
	p.mu.Lock()
	if match := vllmKVRe.FindStringSubmatch(line); len(match) > 1 {
		if val, err := strconv.ParseFloat(match[1], 64); err == nil {
			proxy := int64(val * 100)
			if proxy > p.peakVRAMMB {
				p.peakVRAMMB = proxy
			}
		}
	}
	peak := p.peakVRAMMB
	p.mu.Unlock()
	m.PeakVRAMMB = peak

	return m
}

package telemetry

import (
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type Metrics struct {
	DecodeRate  float64
	TTFT        int
	PrefillRate float64
	Requests    int
	MaxRequests int
	
	VRAMUsed    float64
	VRAMTotal   float64
	PowerUsed   float64
	PowerTotal  float64
	Temp        int
	
	TotalTokens int
	PromptTokens int
	CompletionTokens int
	Duration    float64
}

type TelemetryUpdateMsg Metrics

func Tick() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg {
		// Mock metrics generation for the prototype
		decode := 20.0 + rand.Float64()*10.0
		prefill := 300.0 + rand.Float64()*50.0
		
		return TelemetryUpdateMsg{
			DecodeRate:  decode,
			TTFT:        100 + rand.Intn(40),
			PrefillRate: prefill,
			Requests:    1 + rand.Intn(3),
			MaxRequests: 5,
			
			VRAMUsed:    8.2 + rand.Float64()*0.5,
			VRAMTotal:   16.0,
			PowerUsed:   120.0 + rand.Float64()*20.0,
			PowerTotal:  250.0,
			Temp:        55 + rand.Intn(10),
			
			TotalTokens: 45210 + rand.Intn(100),
			PromptTokens: 20120,
			CompletionTokens: 25090,
			Duration:    2.4,
		}
	})
}

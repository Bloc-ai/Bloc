package tui

import (
	"dashboard/styles"
	"dashboard/telemetry"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DashboardModel struct {
	metrics        telemetry.Metrics
	width          int
	height         int
	historyDecode  []float64
	historyPrefill []float64
	port           int
	engineName     string
	modelName      string
	hardware       string
}

func NewDashboardModel(port int, engineName, modelName, hardware string) DashboardModel {
	return DashboardModel{
		metrics:        telemetry.Metrics{},
		historyDecode:  make([]float64, 0),
		historyPrefill: make([]float64, 0),
		port:           port,
		engineName:     engineName,
		modelName:      modelName,
		hardware:       hardware,
	}
}

func (m DashboardModel) Init() tea.Cmd {
	return telemetry.Tick()
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case telemetry.TelemetryUpdateMsg:
		m.metrics = telemetry.Metrics(msg)

		boxWidth := m.width - 4
		if boxWidth < 62 {
			boxWidth = 62
		}
		if boxWidth > 100 {
			boxWidth = 100
		}
		maxHistory := boxWidth - 30
		if maxHistory < 15 {
			maxHistory = 15
		}

		// Update sparkline history
		m.historyDecode = append(m.historyDecode, m.metrics.DecodeRate)
		if len(m.historyDecode) > maxHistory {
			m.historyDecode = m.historyDecode[len(m.historyDecode)-maxHistory:]
		}

		m.historyPrefill = append(m.historyPrefill, m.metrics.PrefillRate)
		if len(m.historyPrefill) > maxHistory {
			m.historyPrefill = m.historyPrefill[len(m.historyPrefill)-maxHistory:]
		}

		return m, telemetry.Tick()
	}
	return m, nil
}

func mMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m DashboardModel) renderProgressBar(used, total float64, width int) string {
	ratio := used / total
	if ratio > 1 {
		ratio = 1
	}
	filledWidth := int(ratio * float64(width))
	emptyWidth := width - filledWidth

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", emptyWidth)

	return lipgloss.NewStyle().Foreground(styles.BlocBlue).Render(fmt.Sprintf("[%s%s]", filled, empty))
}

func (m DashboardModel) renderSparkline(data []float64) string {
	if len(data) == 0 {
		return ""
	}

	bars := []string{" ", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	if max == min {
		max = min + 1
	}

	result := ""
	for _, v := range data {
		normalized := (v - min) / (max - min)
		idx := int(normalized * 7)
		if idx < 0 {
			idx = 0
		}
		if idx > 7 {
			idx = 7
		}
		result += bars[idx]
	}

	return lipgloss.NewStyle().Foreground(styles.BlocBlue).Render(result)
}

func createBox(title, content string, width int) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.DimWhite).
		Width(width - 2). // adjust for borders
		Padding(0, 1)

	titleText := styles.HighlightText.Render("─ " + title + " ")
	
	// Add title inside the box top
	return boxStyle.Render(titleText + "\n" + content)
}

func (m DashboardModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	statusText := fmt.Sprintf("● ACTIVE  [%s]  [%s]  :%d\n%s\nAPI Endpoint: http://localhost:%d/v1", m.engineName, m.hardware, m.port, m.modelName, m.port)
	status := styles.DimText.Render(statusText)
	
	boxWidth := m.width - 4
	if boxWidth < 62 {
		boxWidth = 62
	}
	if boxWidth > 100 {
		boxWidth = 100
	}

	colWidth := (boxWidth - 4) / 4
	colFmt := fmt.Sprintf("%%-%ds%%-%ds%%-%ds%%-%ds", colWidth, colWidth, colWidth, colWidth)

	speedContent := fmt.Sprintf(colFmt+"\n"+colFmt,
		"DECODE", "TTFT", "PREFILL", "REQUESTS",
		fmt.Sprintf("%.1f tok/s", m.metrics.DecodeRate),
		fmt.Sprintf("%d ms", m.metrics.TTFT),
		fmt.Sprintf("%.1f t/s", m.metrics.PrefillRate),
		fmt.Sprintf("%d / %d", m.metrics.Requests, m.metrics.MaxRequests))
	speedBox := createBox("SPEED & LATENCY", speedContent, boxWidth)

	hwContent := fmt.Sprintf("%-15s %s %.1f / %.1f GB\n%-15s %s %.1f / %.1f W\n%-15s %d°C",
		"VRAM Memory", m.renderProgressBar(m.metrics.VRAMUsed, m.metrics.VRAMTotal, 15), m.metrics.VRAMUsed, m.metrics.VRAMTotal,
		"Power Draw", m.renderProgressBar(m.metrics.PowerUsed, m.metrics.PowerTotal, 15), m.metrics.PowerUsed, m.metrics.PowerTotal,
		"GPU Temp", m.metrics.Temp)
	hwBox := createBox("HARDWARE RESOURCES", hwContent, boxWidth)

	countersContent := fmt.Sprintf("Total Tokens:  %-15d Duration:   %.1fs\nPrompt Tokens: %-15d Completion: %d",
		m.metrics.TotalTokens, m.metrics.Duration,
		m.metrics.PromptTokens, m.metrics.CompletionTokens)
	countersBox := createBox("SESSION COUNTERS", countersContent, boxWidth)

	trendContent := fmt.Sprintf("Prefill (t/s):  %s\nDecode (tok/s): %s",
		m.renderSparkline(m.historyPrefill),
		m.renderSparkline(m.historyDecode))
	trendBox := createBox("THROUGHPUT TREND", trendContent, boxWidth)

	content := lipgloss.JoinVertical(lipgloss.Left,
		status,
		"",
		speedBox,
		hwBox,
		countersBox,
		trendBox,
	)

	return lipgloss.NewStyle().Padding(0, 2).Render(content)
}

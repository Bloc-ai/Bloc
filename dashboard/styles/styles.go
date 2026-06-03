package styles

import (
	"strings"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	BlocBlue  = lipgloss.Color("#2563EB")
	White     = lipgloss.Color("#FFFFFF")
	DimWhite  = lipgloss.Color("#6B7280")
	LightGray = lipgloss.Color("#9CA3AF")
	Red       = lipgloss.Color("#EF4444")
	Green     = lipgloss.Color("#10B981")

	// Global Layout Styles
	AppStyle = lipgloss.NewStyle().Padding(0, 1)
	
	TabInactive = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(DimWhite)
	
	TabActive = lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(BlocBlue).
		Bold(true).
		Underline(true)
	
	TabDivider = lipgloss.NewStyle().
		Foreground(DimWhite).
		SetString("│")

	// Component Styles
	BorderBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BlocBlue).
		Padding(0, 1)

	ThinkingBox = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(DimWhite).
		Foreground(DimWhite).
		PaddingLeft(1)

	TitleText = lipgloss.NewStyle().
		Foreground(White).
		Bold(true)

	HighlightText = lipgloss.NewStyle().
		Foreground(BlocBlue).
		Bold(true)

	DimText = lipgloss.NewStyle().
		Foreground(DimWhite)

	// Custom layouts
	HeaderBox = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(DimWhite).
		PaddingBottom(1).
		MarginBottom(1)
)

func RenderTabs(activeTab int, tabNames []string, width int) string {
	var renderedTabs []string
	for i, name := range tabNames {
		if i == activeTab {
			renderedTabs = append(renderedTabs, TabActive.Render(name))
		} else {
			renderedTabs = append(renderedTabs, TabInactive.Render(name))
		}
	}
	
	result := ""
	for i, t := range renderedTabs {
		result += t
		if i < len(renderedTabs)-1 {
			result += TabDivider.String()
		}
	}
	
	wInner := width - 2
	wContent := lipgloss.Width(result)
	
	var paddedResult string
	if wContent >= wInner {
		paddedResult = result
	} else {
		left := (wInner - wContent) / 2
		right := wInner - wContent - left
		paddedResult = strings.Repeat(" ", left) + result + strings.Repeat(" ", right)
	}
	
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, true, true, true).
		BorderForeground(DimWhite).
		Render(paddedResult)
}

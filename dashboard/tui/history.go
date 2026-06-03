package tui

import (
	"dashboard/data"
	"dashboard/styles"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HistoryItem struct {
	SessionID string
	Model     string
	Summary   string
	Timestamp string
}

type HistoryModel struct {
	items       []HistoryItem
	selectedIdx int
	loadedChat  string
	width       int
	height      int
	modelName   string
}

func NewHistoryModel(modelName string) HistoryModel {
	return HistoryModel{
		items:       []HistoryItem{},
		selectedIdx: 0,
		modelName:   modelName,
	}
}

func (m HistoryModel) Init() tea.Cmd {
	return func() tea.Msg {
		sessions, _ := data.ListSessions()
		return sessions
	}
}

func (m HistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case []data.Session:
		var items []HistoryItem
		for _, s := range msg {
			summary := s.Title
			if summary == "" && len(s.Messages) > 0 {
				summary = s.Messages[0].Content
			}
			items = append(items, HistoryItem{
				SessionID: s.ID,
				Model:     m.modelName,
				Summary:   summary,
				Timestamp: s.UpdatedAt.Format("2006-01-02 15:04"),
			})
		}
		m.items = items
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				m.loadedChat = ""
			}
		case "down", "j":
			if m.selectedIdx < len(m.items)-1 {
				m.selectedIdx++
				m.loadedChat = ""
			}
		case "x", "delete":
			if len(m.items) > 0 {
				data.DeleteSession(m.items[m.selectedIdx].SessionID)
				m.items = append(m.items[:m.selectedIdx], m.items[m.selectedIdx+1:]...)
				if m.selectedIdx >= len(m.items) && m.selectedIdx > 0 {
					m.selectedIdx--
				}
				m.loadedChat = ""
			}
		case "enter":
			if len(m.items) > 0 {
				m.loadedChat = m.items[m.selectedIdx].Summary
				return m, func() tea.Msg { return LoadSessionMsg{SessionID: m.items[m.selectedIdx].SessionID} }
			}
		}
	}
	return m, nil
}

func (m HistoryModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	boxWidth := m.width - 4
	if boxWidth < 62 {
		boxWidth = 62
	}
	if boxWidth > 100 {
		boxWidth = 100
	}

	var builder strings.Builder
	builder.WriteString(styles.HighlightText.Render("   PAST CONVERSATIONS") + "\n\n")

	for i, item := range m.items {
		var line string
		meta := fmt.Sprintf("[%s] %s", item.Timestamp, item.Model)
		summary := item.Summary

		// Truncate summary if too long for current screen width
		availWidth := boxWidth - len(meta) - 8
		if availWidth > 10 && len(summary) > availWidth {
			summary = summary[:availWidth-3] + "..."
		}

		if i == m.selectedIdx {
			line = " › " + lipgloss.NewStyle().Foreground(styles.White).Bold(true).Render(summary) + "  " + styles.DimText.Render(meta)
		} else {
			line = "   " + styles.DimText.Render(summary) + "  " + styles.DimText.Render(meta)
		}
		builder.WriteString(line + "\n")
	}

	// Selection detail footer inside the page
	var statusMsg string
	if m.loadedChat != "" {
		statusMsg = lipgloss.NewStyle().
			Foreground(styles.Green).
			Bold(true).
			Render(fmt.Sprintf("✔ Loaded: \"%s\"", m.loadedChat))
	} else {
		statusMsg = styles.DimText.Render("Use Up/Down (k/j) to navigate • Enter to load • 'x' to delete")
	}

	boxContent := builder.String() + "\n\n" + statusMsg
	
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.DimWhite).
		Width(boxWidth - 2).
		Padding(1, 2).
		Render(boxContent)

	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, box)
}

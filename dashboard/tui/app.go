package tui

import (
	"dashboard/data"
	"dashboard/engine"
	"dashboard/styles"
	"dashboard/telemetry"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type LoadSessionMsg struct {
	SessionID string
}

type NewChatMsg struct{}

type AppModel struct {
	activeTab int
	tabs      []string
	
	// Sub-models
	chatTab      tea.Model
	dashboardTab tea.Model
	historyTab   tea.Model
	
	width      int
	height     int
	port       int
	engineName string
	modelName  string
	hardware   string
	logPath    string
	// PERF-6: cached log content — refreshed via periodic tick, not every frame
	logCache   string
}

func NewApp(version, logPath string, port int, engineName, modelName, hardware string) AppModel {
	return AppModel{
		activeTab:    0,
		tabs:         []string{"CHAT", "STATUS", "HISTORY", "MODELS", "LOGS"},
		chatTab:      NewChatModel(version, port, engineName, modelName, hardware),
		dashboardTab: NewDashboardModel(port, engineName, modelName, hardware),
		historyTab:   NewHistoryModel(modelName),
		port:         port,
		engineName:   engineName,
		modelName:    modelName,
		hardware:     hardware,
		logPath:      logPath,
	}
}

// logRefreshMsg is sent by the periodic tick to refresh the log file cache.
type logRefreshMsg struct{}

func tickLogRefresh() tea.Cmd {
	// PERF-6: refresh log cache every 500ms instead of reading the file on every frame
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return logRefreshMsg{}
	})
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("Bloc Studio"),
		m.chatTab.Init(),
		m.dashboardTab.Init(),
		m.historyTab.Init(),
		tickLogRefresh(),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Pass down size updates to active models
		m.chatTab, cmd = m.chatTab.Update(msg)
		cmds = append(cmds, cmd)
		m.dashboardTab, cmd = m.dashboardTab.Update(msg)
		cmds = append(cmds, cmd)
		m.historyTab, cmd = m.historyTab.Update(msg)
		cmds = append(cmds, cmd)

	case telemetry.TelemetryUpdateMsg:
		m.dashboardTab, cmd = m.dashboardTab.Update(msg)
		return m, cmd
		
	case spinner.TickMsg:
		m.chatTab, cmd = m.chatTab.Update(msg)
		return m, cmd
		
	case []data.Session:
		m.historyTab, cmd = m.historyTab.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			engine.ConsolidateMemory(m.port)
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			// PERF-7: sessions were already loaded on Init(); only re-load on
			// explicit 'r' key (below). Removing Init() on every tab switch
			// eliminates O(n) disk reads each time the user presses Tab.
			return m, tea.Batch(cmds...)
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			return m, tea.Batch(cmds...)
		case "r":
			// PERF-7: explicit refresh for the history tab on demand
			if m.activeTab == 2 {
				cmds = append(cmds, m.historyTab.Init())
				return m, tea.Batch(cmds...)
			}
		case "ctrl+o":
			m.activeTab = 0
			m.chatTab, cmd = m.chatTab.Update(NewChatMsg{})
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
	case LoadSessionMsg:
		m.activeTab = 0
		m.chatTab, cmd = m.chatTab.Update(msg)
		return m, cmd

	case logRefreshMsg:
		// PERF-6: update the log cache and schedule the next tick
		if m.logPath != "" {
			if data, err := os.ReadFile(m.logPath); err == nil {
				m.logCache = string(data)
			}
		}
		return m, tickLogRefresh()
	}

	// Route messages to the active tab
	if m.activeTab == 0 {
		m.chatTab, cmd = m.chatTab.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.activeTab == 1 {
		m.dashboardTab, cmd = m.dashboardTab.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.activeTab == 2 {
		m.historyTab, cmd = m.historyTab.Update(msg)
		cmds = append(cmds, cmd)
	}
	// Models and logs tabs are placeholders for now

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	var view string

	// Render Header
	width := m.width - 4
	if width < 62 {
		width = 62
	}
	if m.activeTab == 1 { // Telemetry
		if width > 100 {
			width = 100
		}
	}
	header := styles.RenderTabs(m.activeTab, m.tabs, width)
	view += lipgloss.PlaceHorizontal(m.width, lipgloss.Center, header) + "\n\n"

	// Render Active Tab Content
	var content string
	switch m.activeTab {
	case 0:
		content = m.chatTab.View()
	case 1:
		content = m.dashboardTab.View()
	case 2:
		content = m.historyTab.View()
	case 3:
		content = styles.DimText.Render("Model Manager coming soon...")
		content = lipgloss.Place(m.width, m.height-7, lipgloss.Center, lipgloss.Center, content)
	case 4:
		content = m.renderLogs()
		content = lipgloss.NewStyle().Padding(1, 2).Render(content)
	}

	// Lock the body height to keep footer position constant across tabs
	bodyHeight := m.height - 7
	if m.activeTab == 1 && bodyHeight < 24 { // Telemetry needs at least 24 lines to render fully
		bodyHeight = 24
	}
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	body := lipgloss.NewStyle().Height(bodyHeight).Render(content)
	view += body

	// Global Footer
	footerText := "[ctrl+c: Quit] [Tab: Next Tab] [ctrl+o: New Chat]"
	if m.width < 50 {
		footerText = "ctrl+c: Quit • Tab: Nav • ctrl+o: New"
	}
	footer := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Foreground(styles.DimWhite).
		MarginTop(1).
		Render(footerText)

	return lipgloss.JoinVertical(lipgloss.Left, view, footer)
}

func (m AppModel) renderLogs() string {
	// PERF-6: return cached content — logCache is refreshed every 500ms by
	// the tickLogRefresh command instead of reading the file on every frame.
	if m.logCache == "" {
		return styles.DimText.Render("Waiting for engine logs...")
	}
	lines := strings.Split(m.logCache, "\n")
	maxLines := m.height - 10
	if maxLines < 10 {
		maxLines = 10
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

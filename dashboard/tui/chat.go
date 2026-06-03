package tui

import (
	"bufio"
	"bytes"
	"context"
	"dashboard/data"
	"dashboard/styles"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type ChatModel struct {
	textInput    textinput.Model
	viewport     viewport.Model
	spinner      spinner.Model
	session      data.Session
	isGenerating bool
	width        int
	height       int
	port         int
	engineName      string
	modelName       string
	hardware        string
	version         string
	streamChan      chan streamMsg
	streamCancel    context.CancelFunc
	currentResponse string
	tokenCount      int
	startTime       time.Time
	firstTokenTime  time.Time
	isFirstToken    bool
	streamSessionID string
	// PERF-1: cached renderer — recreated only on window resize, not per token
	renderer      *glamour.TermRenderer
	// PERF-2: pre-rendered strings for completed messages — only updated when a
	// new message is finalised, not on every streaming token
	renderedMsgs  []string
	// PERF-5: set true when content changes; cleared after SetContent to skip
	// redundant viewport updates on spinner ticks
	historyDirty  bool
}

func NewChatModel(version string, port int, engineName, modelName, hardware string) ChatModel {
	ti := textinput.New()
	ti.Placeholder = "Type a command or /help for options..."
	ti.Focus()
	ti.Prompt = "› "
	ti.CharLimit = 4000
	ti.Width = 60

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Padding(0, 2)

	// Create new session by default
	sess := data.Session{
		ID:        fmt.Sprintf("session_%d", time.Now().UnixNano()),
		Title:     "New Chat",
		Messages:  []data.Message{},
		UpdatedAt: time.Now(),
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.BlocBlue)

	// PERF-1: build the Glamour renderer once at construction time.
	// It will be rebuilt on WindowSizeMsg when the terminal width changes.
	r, _ := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(60),
	)

	return ChatModel{
		textInput:  ti,
		viewport:   vp,
		spinner:    s,
		session:    sess,
		port:       port,
		version:    version,
		engineName: engineName,
		modelName:  modelName,
		hardware:   hardware,
		renderer:   r,
	}
}

func (m ChatModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

type streamMsg struct {
	content   string
	isDone    bool
	isError   bool
	err       error
	sessionID string // RACE-2: used to discard tokens from cancelled streams
}

type openaiStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type openaiRequest struct {
	Messages    []map[string]string `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func startLLMStream(ctx context.Context, sessionID string, port int, session data.Session, ch chan<- streamMsg) tea.Cmd {
	return func() tea.Msg {
		go func() {
			var reqMsgs []map[string]string
			for _, m := range session.Messages {
				reqMsgs = append(reqMsgs, map[string]string{"role": m.Role, "content": m.Content})
			}
			reqBody := openaiRequest{
				Messages: reqMsgs,
				Stream:   true,
			}
			// SEC-2: handle json.Marshal error instead of silently discarding
			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				ch <- streamMsg{isError: true, err: fmt.Errorf("failed to encode request: %w", err)}
				return
			}
			url := fmt.Sprintf("http://localhost:%d/v1/chat/completions", port)
			
			// SEC-3: use context with 5-minute timeout to prevent infinite blocking
			streamCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			
			req, err := http.NewRequestWithContext(streamCtx, "POST", url, bytes.NewBuffer(jsonData))
			if err != nil {
				ch <- streamMsg{isError: true, err: err}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "text/event-stream")
			
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				select {
				case ch <- streamMsg{isError: true, err: err}:
				default:
				}
				return
			}
			defer resp.Body.Close()
			
			// SEC-5: check HTTP status before reading body as SSE stream
			if resp.StatusCode != http.StatusOK {
				select {
				case ch <- streamMsg{isError: true, err: fmt.Errorf("engine returned HTTP %d — is the model loaded?", resp.StatusCode)}:
				default:
				}
				return
			}
			
			reader := bufio.NewReader(resp.Body)
			for {
				// Bail out if the context was cancelled (new chat started)
				select {
				case <-streamCtx.Done():
					return
				default:
				}
				line, err := reader.ReadBytes('\n')
				if err != nil {
					break
				}
				lineStr := string(line)
				if strings.HasPrefix(lineStr, "data: ") {
					dataStr := strings.TrimSpace(strings.TrimPrefix(lineStr, "data: "))
					if dataStr == "[DONE]" {
						break
					}
					var oResp openaiStreamResponse
					if err := json.Unmarshal([]byte(dataStr), &oResp); err == nil && len(oResp.Choices) > 0 {
						content := oResp.Choices[0].Delta.Content
						if content != "" {
							select {
							case ch <- streamMsg{content: content, sessionID: sessionID}:
							case <-streamCtx.Done():
								return
							}
						}
					}
				}
			}
			select {
			case ch <- streamMsg{isDone: true, sessionID: sessionID}:
			default:
			}
		}()
		return nil
	}
}

func waitForStream(ch <-chan streamMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return streamMsg{isDone: true}
		}
		return msg
	}
}

func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		m.textInput.Width = m.width - 6
		m.viewport.Width = m.width
		vpHeight := m.height - 15
		if vpHeight < 5 {
			vpHeight = 5
		}
		m.viewport.Height = vpHeight
		
		// PERF-1: rebuild renderer when width changes (word-wrap depends on width)
		if r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(m.width-6),
		); err == nil {
			m.renderer = r
		}
		// PERF-2: invalidate the rendered message cache — layout has changed
		m.renderedMsgs = nil
		m.viewport.SetContent(m.renderHistory())

	case LoadSessionMsg:
		session, err := data.LoadSession(msg.SessionID)
		if err == nil {
			m.session = *session
			// PERF-2: invalidate cache on session change
			m.renderedMsgs = nil
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
		}

	case NewChatMsg:
		m.session = data.Session{
			ID:        fmt.Sprintf("session_%d", time.Now().UnixNano()),
			Title:     "New Chat",
			Messages:  []data.Message{},
			UpdatedAt: time.Now(),
		}
		// PERF-2: clear cache on new chat
		m.renderedMsgs = nil
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil

	case streamMsg:
		// RACE-2: discard tokens that belong to a previous stream (old goroutine
		// still draining its buffer after the user started a new chat).
		if msg.sessionID != "" && msg.sessionID != m.streamSessionID {
			return m, nil
		}
		if msg.isError {
			m.isGenerating = false
			m.session.Messages = append(m.session.Messages, data.Message{Role: "assistant", Content: "Error: " + msg.err.Error(), Timestamp: time.Now()})
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		if msg.isDone {
			m.isGenerating = false
			
			var ttft, tps float64
			// DATA-4: only compute metrics if firstTokenTime was actually set
			if !m.firstTokenTime.IsZero() {
				elapsed := time.Since(m.firstTokenTime).Seconds()
				ttft = m.firstTokenTime.Sub(m.startTime).Seconds()
				if elapsed > 0 {
					tps = float64(m.tokenCount) / elapsed
				}
			}
			
			finished := data.Message{
				Role:      "assistant",
				Content:   m.currentResponse,
				Timestamp: time.Now(),
				Tokens:    m.tokenCount,
				TTFT:      ttft,
				TPS:       tps,
			}
			m.session.Messages = append(m.session.Messages, finished)
			if len(m.session.Messages) > 0 {
				title := m.session.Messages[0].Content
				// DATA-1: truncate long user prompts instead of using up to 4000 chars as title
				if len(title) > 50 {
					title = title[:47] + "..."
				}
				m.session.Title = title
			}
			data.SaveSession(&m.session)
			
			// PERF-2: renderHistory (now a pointer receiver) will automatically
			// notice the new message in m.session.Messages and append it to the cache.
			m.currentResponse = ""
			m.historyDirty = true
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			return m, nil
		}
		
		if m.isFirstToken {
			m.firstTokenTime = time.Now()
			m.isFirstToken = false
		}
		
		m.currentResponse += msg.content
		m.tokenCount++
		// PERF-2: only the active streaming block changes — mark dirty and
		// let renderHistory compose cache + active block cheaply
		m.historyDirty = true
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		
		return m, waitForStream(m.streamChan)

	case spinner.TickMsg:
		var sCmd tea.Cmd
		m.spinner, sCmd = m.spinner.Update(msg)
		if m.isGenerating {
			// PERF-5: only update viewport when content has actually changed;
			// skip the redundant re-render on spinner-only ticks
			if m.historyDirty {
				m.viewport.SetContent(m.renderHistory())
				m.historyDirty = false
			}
			cmds = append(cmds, sCmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if msg.String() == "enter" && m.textInput.Value() != "" {
			input := m.textInput.Value()
			m.textInput.SetValue("")
			
			// Add to session
			m.session.Messages = append(m.session.Messages, data.Message{
				Role:      "user",
				Content:   input,
				Timestamp: time.Now(),
			})
			
			m.isGenerating = true
			m.currentResponse = ""
			m.tokenCount = 0
			m.startTime = time.Now()
			m.isFirstToken = true
			
			// SEC-4: cancel the previous stream goroutine before starting a new one
			if m.streamCancel != nil {
				m.streamCancel()
			}
			streamCtx, cancel := context.WithCancel(context.Background())
			m.streamCancel = cancel
			// RACE-2: stamp this stream with the session ID so stale tokens
			// from the old goroutine can be silently discarded on receipt.
			m.streamSessionID = m.session.ID
			m.streamChan = make(chan streamMsg, 100)
			
			m.viewport.SetContent(m.renderHistory())
			m.viewport.GotoBottom()
			
			return m, tea.Batch(
				startLLMStream(streamCtx, m.session.ID, m.port, m.session, m.streamChan),
				waitForStream(m.streamChan),
			)
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// DATA-5: handle multiple <think> blocks in a single response, and handle unclosed blocks
func extractThinking(content string) (string, string) {
	var thinking strings.Builder
	response := content

	for {
		startIdx := strings.Index(response, "<think>")
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(response[startIdx:], "</think>")
		if endIdx != -1 {
			endIdx += startIdx
			if thinking.Len() > 0 {
				thinking.WriteString("\n\n")
			}
			thinking.WriteString(strings.TrimSpace(response[startIdx+7 : endIdx]))
			response = response[:startIdx] + response[endIdx+8:]
		} else {
			// Unclosed tag (e.g. while streaming)
			if thinking.Len() > 0 {
				thinking.WriteString("\n\n")
			}
			thinking.WriteString(strings.TrimSpace(response[startIdx+7:]))
			response = response[:startIdx]
			break
		}
	}
	return thinking.String(), strings.TrimSpace(response)
}

// renderMessage renders a single completed message to a string using the
// cached renderer. Called once per message when it finishes streaming.
func (m ChatModel) renderMessage(msg data.Message) string {
	if msg.Role == "user" {
		userStr := styles.DimText.Render("› ") + msg.Content
		return lipgloss.NewStyle().Width(m.width - 6).Render(userStr)
	}
	if msg.Role == "assistant" {
		thinking, resp := extractThinking(msg.Content)
		var block string
		if thinking != "" {
			block = styles.DimText.Render("✓ Evaluated") + "\n" + styles.ThinkingBox.Render(thinking) + "\n"
		}
		if m.renderer != nil {
			if out, err := m.renderer.Render(resp); err == nil {
				block += strings.TrimSpace(out)
			} else {
				block += resp
			}
		} else {
			block += resp
		}
		if msg.Tokens > 0 {
			statsText := fmt.Sprintf("%d tokens    %.2fs TTFT    %.2f t/s", msg.Tokens, msg.TTFT, msg.TPS)
			block += "\n\n" + styles.DimText.Render(statsText)
		}
		return block
	}
	return styles.DimText.Render("[system] ") + msg.Content
}

func (m *ChatModel) renderHistory() string {
	if len(m.session.Messages) == 0 {
		return m.renderIdleState()
	}

	// PERF-2: build the cache for any completed messages not yet rendered.
	// On most token events, only the active block changes — this loop is O(0).
	for len(m.renderedMsgs) < len(m.session.Messages) {
		idx := len(m.renderedMsgs)
		m.renderedMsgs = append(m.renderedMsgs, m.renderMessage(m.session.Messages[idx]))
	}

	parts := make([]string, 0, len(m.renderedMsgs)+1)
	parts = append(parts, m.renderedMsgs...)

	if m.isGenerating {
		activeBlock := styles.DimText.Render(m.spinner.View() + " Thinking...")
		if m.currentResponse != "" {
			thinking, resp := extractThinking(m.currentResponse)
			activeBlock = ""
			if thinking != "" {
				thinkLabel := styles.DimText.Render("✓ Evaluating...")
				thinkBlock := styles.ThinkingBox.Render(thinking)
				activeBlock = thinkLabel + "\n" + thinkBlock + "\n"
			}
			
			if resp != "" && m.renderer != nil {
				out, rErr := m.renderer.Render(resp)
				if rErr == nil {
					activeBlock += strings.TrimSpace(out)
				} else {
					activeBlock += resp
				}
			} else if resp != "" {
				activeBlock += resp
			}
			
			// Live performance stats during generation
			if !m.firstTokenTime.IsZero() {
				elapsed := time.Since(m.firstTokenTime).Seconds()
				ttft := m.firstTokenTime.Sub(m.startTime).Seconds()
				tps := 0.0
				if elapsed > 0 {
					tps = float64(m.tokenCount) / elapsed
				}
				statsText := fmt.Sprintf("%d tokens    %.2fs TTFT    %.2f t/s", m.tokenCount, ttft, tps)
				activeBlock += "\n\n" + styles.DimText.Render(statsText)
			}
		}
		parts = append(parts, activeBlock)
	}
	
	return strings.Join(parts, "\n\n")
}

func (m ChatModel) renderIdleState() string {
	width := m.width - 4
	if width < 62 {
		width = 62
	}

	wInner := width - 2
	wLeft := (wInner - 5) * 40 / 100
	wRight := (wInner - 5) - wLeft

	// 1. Compile Left Column Lines (Mascot square shrunk to 6x12)
	mascot := `████████████
████████████
████████████
████████████
████████████
████████████`
	
	mascotLines := strings.Split(mascot, "\n")
	mascotColoredLines := make([]string, len(mascotLines))
	for i, line := range mascotLines {
		mascotColoredLines[i] = lipgloss.NewStyle().Foreground(styles.BlocBlue).Render(line)
	}

	greetingText := "Welcome back, Builder!"
	if len(greetingText) > wLeft {
		greetingText = greetingText[:wLeft]
	}
	greeting := styles.TitleText.Render(greetingText)
	
	var infoRaw string
	if wLeft >= 38 {
		infoRaw = fmt.Sprintf("%s  •  %s  •  ~/Documents/bloc", m.modelName, m.hardware)
	} else if wLeft >= 22 {
		infoRaw = fmt.Sprintf("%s  •  %s", m.modelName, m.hardware)
	} else {
		infoRaw = m.modelName
	}
	if len(infoRaw) > wLeft {
		infoRaw = infoRaw[:wLeft]
	}
	infoLine := styles.DimText.Render(infoRaw)

	var leftLines []string
	leftLines = append(leftLines, centerText(greeting, wLeft))
	
	// Center the mascot as a single unified block to prevent line shearing
	centeredMascot := centerBlock(mascotColoredLines, wLeft)
	leftLines = append(leftLines, centeredMascot...)
	
	leftLines = append(leftLines, centerText(infoLine, wLeft))

	// 2. Compile Right Column Lines
	tipsTitleRaw := "Tips for getting started"
	if len(tipsTitleRaw) > wRight {
		tipsTitleRaw = tipsTitleRaw[:wRight]
	}
	tipsTitle := lipgloss.NewStyle().Foreground(styles.BlocBlue).Bold(true).Render(tipsTitleRaw)
	
	var tip1Raw string
	if wRight >= 38 {
		tip1Raw = "Run /run to start a local model server"
	} else {
		tip1Raw = "Run /run to start server"
	}
	if len(tip1Raw) > wRight {
		tip1Raw = tip1Raw[:wRight]
	}
	tip1 := styles.DimText.Render(tip1Raw)

	tip2Raw := "Run /models to view list"
	if len(tip2Raw) > wRight {
		tip2Raw = tip2Raw[:wRight]
	}
	tip2 := styles.DimText.Render(tip2Raw)
	
	dividerLine := lipgloss.NewStyle().Foreground(styles.DimWhite).Render(strings.Repeat("─", wRight))
	
	activityTitleRaw := "Recent activity"
	if len(activityTitleRaw) > wRight {
		activityTitleRaw = activityTitleRaw[:wRight]
	}
	activityTitle := lipgloss.NewStyle().Foreground(styles.BlocBlue).Bold(true).Render(activityTitleRaw)

	activityTextRaw := "No recent activity"
	if len(activityTextRaw) > wRight {
		activityTextRaw = activityTextRaw[:wRight]
	}
	activityText := styles.DimText.Render(activityTextRaw)

	var rightLines []string
	rightLines = append(rightLines, tipsTitle)
	rightLines = append(rightLines, tip1)
	rightLines = append(rightLines, tip2)
	rightLines = append(rightLines, dividerLine)
	rightLines = append(rightLines, activityTitle)
	rightLines = append(rightLines, activityText)

	// Pad both columns to the same height
	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	borderStyle := lipgloss.NewStyle().Foreground(styles.BlocBlue)
	dividerStyle := lipgloss.NewStyle().Foreground(styles.DimWhite)

	// Embed title in top border
	title := fmt.Sprintf("Bloc Studio v%s", m.version)
	titleStyled := borderStyle.Render("── ") + styles.TitleText.Render(title) + borderStyle.Render(" ──")
	titleWidth := lipgloss.Width(titleStyled)
	remainingWidth := wInner - titleWidth
	
	var topBorder string
	if remainingWidth > 0 {
		topBorder = borderStyle.Render("╭") + titleStyled + borderStyle.Render(strings.Repeat("─", remainingWidth)) + borderStyle.Render("╮")
	} else {
		topBorder = borderStyle.Render("╭" + strings.Repeat("─", wInner) + "╮")
	}
	
	bottomBorder := borderStyle.Render("╰") + borderStyle.Render(strings.Repeat("─", wInner)) + borderStyle.Render("╯")

	var resultLines []string
	resultLines = append(resultLines, topBorder)

	for i := 0; i < maxLines; i++ {
		var leftLine string
		if i < len(leftLines) {
			leftLine = leftLines[i]
		}
		leftLinePad := padRight(leftLine, wLeft)

		var rightLine string
		if i < len(rightLines) {
			rightLine = rightLines[i]
		}
		rightLinePad := padRight(rightLine, wRight)

		middleDivider := dividerStyle.Render("│")
		leftSideBorder := borderStyle.Render("│")
		rightSideBorder := borderStyle.Render("│")

		line := leftSideBorder + " " + leftLinePad + " " + middleDivider + " " + rightLinePad + " " + rightSideBorder
		resultLines = append(resultLines, line)
	}

	resultLines = append(resultLines, bottomBorder)

	return strings.Join(resultLines, "\n")
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func centerText(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	left := (width - w) / 2
	right := width - w - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func centerBlock(lines []string, width int) []string {
	maxWidth := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxWidth {
			maxWidth = w
		}
	}

	leftOffset := (width - maxWidth) / 2
	if leftOffset < 0 {
		leftOffset = 0
	}

	var centered []string
	for _, line := range lines {
		w := lipgloss.Width(line)
		rightOffset := width - w - leftOffset
		if rightOffset < 0 {
			rightOffset = 0
		}
		padded := strings.Repeat(" ", leftOffset) + line + strings.Repeat(" ", rightOffset)
		centered = append(centered, padded)
	}
	return centered
}

func (m ChatModel) View() string {
	// Status Info above chat
	statusText := fmt.Sprintf("● ACTIVE  [%s]  [%s]  :%d\n%s\nAPI Endpoint: http://localhost:%d/v1", m.engineName, m.hardware, m.port, m.modelName, m.port)
	status := styles.DimText.Render(statusText)
	
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.BlocBlue).
		Width(m.width - 4).
		Padding(0, 1).
		Render(m.textInput.View())

	view := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Padding(0, 2).Render(status),
		"\n",
		m.viewport.View(),
		"\n",
		lipgloss.PlaceHorizontal(m.width, lipgloss.Center, inputBox),
	)
	
	return view
}

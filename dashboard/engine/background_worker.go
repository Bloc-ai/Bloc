package engine

import (
	"bytes"
	"dashboard/data"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StartIdleMonitor starts a background goroutine that occasionally checks for idle
// sessions and consolidates them into the workspace memory.
func StartIdleMonitor(port int) {
	go func() {
		// Idle monitor that runs every 5 minutes
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			<-ticker.C
			ConsolidateMemory(port)
		}
	}()
}

type openaiRequest struct {
	Messages    []map[string]string `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float64             `json:"temperature"`
	Stream      bool                `json:"stream"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// ConsolidateMemory performs the background memory consolidation (KAIROS/autoDream).
func ConsolidateMemory(port int) {
	// 1. Fetch current workspace memory
	currentMem, err := data.GetWorkspaceMemory()
	if err != nil {
		fmt.Printf("Error reading memory: %v\n", err)
		return
	}

	// 2. Fetch the latest active session to see if there's new info
	sessions, err := data.ListSessions()
	if err != nil || len(sessions) == 0 {
		return
	}

	latestSession := sessions[0]
	// If the session hasn't been updated in the last 10 minutes, we assume it's "idle" enough
	if time.Since(latestSession.UpdatedAt) < 10*time.Minute {
		return // Session is still active, don't consolidate yet
	}

	// Format session history for the LLM
	var sb strings.Builder
	for _, msg := range latestSession.Messages {
		sb.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}
	sessionText := sb.String()
	if len(sessionText) > 10000 {
		sessionText = sessionText[len(sessionText)-10000:] // rough truncation to avoid blowing context
	}

	reqBody := openaiRequest{
		Messages: []map[string]string{
			{"role": "system", "content": "You are a background AI agent. Extract concise factual summaries from the provided session logs and return them as bullet points."},
			{"role": "user", "content": fmt.Sprintf("Existing Memory:\n%s\n\nRecent Session:\n%s", currentMem, sessionText)},
		},
		MaxTokens:   512,
		Temperature: 0.2,
		Stream:      false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return
	}

	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/v1/chat/completions", port), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return // Backend might be busy or down
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var oResp openaiResponse
	if err := json.Unmarshal(body, &oResp); err != nil || len(oResp.Choices) == 0 {
		return
	}

	newFact := strings.TrimSpace(oResp.Choices[0].Message.Content)
	if newFact == "" {
		return
	}

	newMem := currentMem + "\n" + newFact

	// Ensure the memory doesn't grow unbounded (mocking the Prune & Store phase)
	if len(newMem) > 25000 {
		newMem = newMem[len(newMem)-25000:]
	}

	data.UpdateWorkspaceMemory(newMem)
}

package data

import (
	"os"
	"path/filepath"
)

const memoryFileName = ".bloc-memory.md"

// GetWorkspaceMemory returns the contents of the memory file in the current workspace
func GetWorkspaceMemory() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path := filepath.Join(cwd, memoryFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No memory yet
		}
		return "", err
	}
	return string(data), nil
}

// UpdateWorkspaceMemory writes the consolidated memory back to the workspace file
func UpdateWorkspaceMemory(newMemory string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path := filepath.Join(cwd, memoryFileName)
	// SEC-8: use 0600 (owner-only) to protect workspace memory from other users.
	return os.WriteFile(path, []byte(newMemory), 0600)
}

// PruneContext window removes oldest messages to fit within a rough character limit (simulating token limits).
// Always preserves the system prompt (role == "system")
func PruneContext(messages []Message, maxChars int) []Message {
	if len(messages) == 0 {
		return messages
	}

	var systemMsg *Message
	var otherMsgs []Message

	// RACE-3: use index-based loop so &messages[i] points to the real slice
	// element. "for _, m := range" copies m, so &m always points to the loop
	// variable (overwritten each iteration), causing systemMsg to silently
	// point to the last element in the slice, not the system message.
	for i := range messages {
		if messages[i].Role == "system" {
			systemMsg = &messages[i]
		} else {
			otherMsgs = append(otherMsgs, messages[i])
		}
	}

	// Calculate current size
	currSize := 0
	if systemMsg != nil {
		currSize += len(systemMsg.Content)
	}

	// Calculate size of other messages
	for _, m := range otherMsgs {
		currSize += len(m.Content)
	}

	// While we are over budget, remove the oldest non-system message
	for currSize > maxChars && len(otherMsgs) > 1 { // keep at least the latest user prompt if possible
		currSize -= len(otherMsgs[0].Content)
		otherMsgs = otherMsgs[1:]
	}

	var final []Message
	if systemMsg != nil {
		final = append(final, *systemMsg)
	}
	final = append(final, otherMsgs...)
	return final
}

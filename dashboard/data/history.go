package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Tokens    int       `json:"tokens,omitempty"`
	TTFT      float64   `json:"ttft,omitempty"`
	TPS       float64   `json:"tps,omitempty"`
}

type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	UpdatedAt time.Time `json:"updated_at"`
}

func getHistoryDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "bloc", "history")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func SaveSession(s *Session) error {
	dir, err := getHistoryDir()
	if err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	if s.ID == "" {
		s.ID = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}
	path := filepath.Join(dir, s.ID+".json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	// DATA-3: write to a temporary file first, then atomically rename it.
	// This prevents permanent session data loss if the system crashes mid-write.
	tmpPath := path + ".tmp"
	// SEC-8: use 0600 (owner-only) to prevent other users on shared systems
	// from reading private chat history.
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func DeleteSession(id string) error {
	dir, err := getHistoryDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, id+".json")
	return os.Remove(path)
}

func LoadSession(id string) (*Session, error) {
	dir, err := getHistoryDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func ListSessions() ([]Session, error) {
	dir, err := getHistoryDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sessions []Session
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			s, err := LoadSession(entry.Name()[:len(entry.Name())-5])
			if err == nil && s != nil {
				sessions = append(sessions, *s)
			} else if err != nil {
				// DATA-2: Warn if a session file is corrupted instead of silently ignoring it.
				fmt.Fprintf(os.Stderr, "Warning: failed to load session %s: %v\n", entry.Name(), err)
			}
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

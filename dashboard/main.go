package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"dashboard/engine"
	"dashboard/tui"
)

func main() {
	engine.StartIdleMonitor(8080)

	p := tea.NewProgram(tui.NewApp("dev", "", 8080, "llama.cpp", "test-model", "CPU"), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}

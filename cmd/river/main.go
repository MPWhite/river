package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/mattwhite/river-go/internal/ai"
	"github.com/mattwhite/river-go/internal/editor"
	"github.com/mattwhite/river-go/internal/statsui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "stats":
			p := tea.NewProgram(statsui.InitModel(), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Printf("Error: %v", err)
				os.Exit(1)
			}
			return
		case "think":
			if err := ai.GenerateTodos(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "analyze":
			if err := ai.GenerateInsights(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "todo":
			if err := ai.GenerateSimpleTodos(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "prompts":
			if err := ai.GeneratePrompts(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Default behavior - run the note editor
	p := tea.NewProgram(editor.NewInitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}

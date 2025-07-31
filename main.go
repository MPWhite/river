package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	cursor int
	choice string
}

func initialModel() model {
	return model{}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 2 {
				m.cursor++
			}
		case "enter", " ":
			choices := []string{"Start River", "Settings", "Quit"}
			m.choice = choices[m.cursor]
			if m.cursor == 2 {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "Welcome to River!\n\n"

	choices := []string{"Start River", "Settings", "Quit"}

	for i, choice := range choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\nPress q to quit.\n"

	if m.choice != "" {
		s += fmt.Sprintf("\nYou chose: %s\n", m.choice)
	}

	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
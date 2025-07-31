package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	content    []string
	cursor     position
	viewport   viewport
}

type position struct {
	row int
	col int
}

type viewport struct {
	width  int
	height int
}

func initialModel() model {
	return model{
		content: []string{""},
		cursor:  position{0, 0},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.width = msg.Width
		m.viewport.height = msg.Height - 2 // Leave room for status bar

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor.row > 0 {
				m.cursor.row--
				// Adjust column if new line is shorter
				if m.cursor.col > len(m.content[m.cursor.row]) {
					m.cursor.col = len(m.content[m.cursor.row])
				}
			}

		case tea.KeyDown:
			if m.cursor.row < len(m.content)-1 {
				m.cursor.row++
				// Adjust column if new line is shorter
				if m.cursor.col > len(m.content[m.cursor.row]) {
					m.cursor.col = len(m.content[m.cursor.row])
				}
			}

		case tea.KeyLeft:
			if m.cursor.col > 0 {
				m.cursor.col--
			} else if m.cursor.row > 0 {
				// Move to end of previous line
				m.cursor.row--
				m.cursor.col = len(m.content[m.cursor.row])
			}

		case tea.KeyRight:
			if m.cursor.col < len(m.content[m.cursor.row]) {
				m.cursor.col++
			} else if m.cursor.row < len(m.content)-1 {
				// Move to start of next line
				m.cursor.row++
				m.cursor.col = 0
			}

		case tea.KeyEnter:
			// Split the current line at cursor position
			currentLine := m.content[m.cursor.row]
			beforeCursor := currentLine[:m.cursor.col]
			afterCursor := currentLine[m.cursor.col:]
			
			// Update current line and insert new line
			m.content[m.cursor.row] = beforeCursor
			newContent := make([]string, len(m.content)+1)
			copy(newContent[:m.cursor.row+1], m.content[:m.cursor.row+1])
			newContent[m.cursor.row+1] = afterCursor
			copy(newContent[m.cursor.row+2:], m.content[m.cursor.row+1:])
			m.content = newContent
			
			// Move cursor to start of new line
			m.cursor.row++
			m.cursor.col = 0

		case tea.KeyBackspace:
			if m.cursor.col > 0 {
				// Delete character before cursor
				line := m.content[m.cursor.row]
				m.content[m.cursor.row] = line[:m.cursor.col-1] + line[m.cursor.col:]
				m.cursor.col--
			} else if m.cursor.row > 0 {
				// Join with previous line
				prevLine := m.content[m.cursor.row-1]
				currentLine := m.content[m.cursor.row]
				m.cursor.col = len(prevLine)
				m.content[m.cursor.row-1] = prevLine + currentLine
				
				// Remove current line
				newContent := make([]string, len(m.content)-1)
				copy(newContent[:m.cursor.row], m.content[:m.cursor.row])
				copy(newContent[m.cursor.row:], m.content[m.cursor.row+1:])
				m.content = newContent
				m.cursor.row--
			}

		case tea.KeySpace:
			// Insert space at cursor position
			line := m.content[m.cursor.row]
			m.content[m.cursor.row] = line[:m.cursor.col] + " " + line[m.cursor.col:]
			m.cursor.col++

		case tea.KeyRunes:
			// Insert characters at cursor position
			line := m.content[m.cursor.row]
			m.content[m.cursor.row] = line[:m.cursor.col] + string(msg.Runes) + line[m.cursor.col:]
			m.cursor.col += len(msg.Runes)
		}
	}

	return m, nil
}

func (m model) View() string {
	var s strings.Builder

	// Display content with cursor
	for i, line := range m.content {
		if i == m.cursor.row {
			// Show cursor on current line
			if m.cursor.col < len(line) {
				s.WriteString(line[:m.cursor.col])
				s.WriteString("█")
				s.WriteString(line[m.cursor.col:])
			} else {
				s.WriteString(line)
				s.WriteString("█")
			}
		} else {
			s.WriteString(line)
		}
		s.WriteString("\n")
	}

	// Add empty lines to fill viewport
	linesShown := len(m.content)
	if m.viewport.height > 0 {
		for i := linesShown; i < m.viewport.height; i++ {
			s.WriteString("~\n")
		}
	}

	// Status bar
	s.WriteString(fmt.Sprintf("\n[Line %d, Col %d] Press Ctrl+C to quit", m.cursor.row+1, m.cursor.col+1))

	return s.String()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
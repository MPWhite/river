package history

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NoteEntry struct {
	Date      time.Time
	Filename  string
	WordCount int
}

type Mode int

const (
	ListMode Mode = iota
	ViewMode
)

type Model struct {
	notes        []NoteEntry
	cursor       int
	viewport     viewport.Model
	mode         Mode
	selectedNote string
	width        int
	height       int
	ready        bool
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("62")).
			Padding(0, 0, 1, 0)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true)

	dateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(1, 0, 0, 0)

	contentStyle = lipgloss.NewStyle().
			Padding(1, 2)
)

func InitModel() Model {
	notes := loadNotes()

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1)

	return Model{
		notes:    notes,
		cursor:   0,
		viewport: vp,
		mode:     ListMode,
	}
}

func loadNotes() []NoteEntry {
	homeDir, _ := os.UserHomeDir()
	riverDir := filepath.Join(homeDir, "river", "notes")

	entries, err := os.ReadDir(riverDir)
	if err != nil {
		return []NoteEntry{}
	}

	var notes []NoteEntry
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Parse date from filename (YYYY-MM-DD.md)
		dateStr := strings.TrimSuffix(entry.Name(), ".md")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// Skip today's note
		if date.Format("2006-01-02") == time.Now().Format("2006-01-02") {
			continue
		}

		// Get word count
		fullPath := filepath.Join(riverDir, entry.Name())
		content, _ := os.ReadFile(fullPath)
		wordCount := countWords(string(content))

		notes = append(notes, NoteEntry{
			Date:      date,
			Filename:  fullPath,
			WordCount: wordCount,
		})
	}

	// Sort by date descending (most recent first)
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Date.After(notes[j].Date)
	})

	return notes
}

func countWords(text string) int {
	// Remove HTML comments
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "<!--") {
			cleanLines = append(cleanLines, line)
		}
	}
	cleanText := strings.Join(cleanLines, "\n")

	if cleanText == "" {
		return 0
	}
	return len(strings.Fields(cleanText))
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		if m.mode == ViewMode {
			headerHeight := 6
			footerHeight := 3
			m.viewport.Width = m.width - 4
			m.viewport.Height = m.height - headerHeight - footerHeight
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			if m.mode == ViewMode {
				// Go back to list
				m.mode = ListMode
				return m, nil
			}
			// Quit from list view
			return m, tea.Quit

		case "up", "k":
			if m.mode == ListMode && m.cursor > 0 {
				m.cursor--
			} else if m.mode == ViewMode {
				m.viewport, _ = m.viewport.Update(msg)
			}

		case "down", "j":
			if m.mode == ListMode && m.cursor < len(m.notes)-1 {
				m.cursor++
			} else if m.mode == ViewMode {
				m.viewport, _ = m.viewport.Update(msg)
			}

		case "enter":
			if m.mode == ListMode && len(m.notes) > 0 {
				// Load and view selected note
				m.selectedNote = m.notes[m.cursor].Filename
				content, _ := os.ReadFile(m.selectedNote)

				// Format content nicely
				lines := strings.Split(string(content), "\n")
				var displayLines []string

				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
						// Show comments in a styled way
						text := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!--"), "-->"))
						if text != "" {
							displayLines = append(displayLines, dateStyle.Render("✦ "+text))
						}
					} else if trimmed != "" || len(displayLines) > 0 {
						displayLines = append(displayLines, line)
					}
				}

				m.viewport.SetContent(strings.Join(displayLines, "\n"))
				m.mode = ViewMode
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.mode == ViewMode {
		return m.viewModeView()
	}

	return m.listModeView()
}

func (m Model) listModeView() string {
	s := titleStyle.Render("🌊 River - Note History")
	s += "\n\n"

	if len(m.notes) == 0 {
		s += "No previous notes found.\n"
		s += helpStyle.Render("Press q to quit")
		return s
	}

	// Calculate visible range
	visibleHeight := m.height - 8 // Account for header and footer
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	start := m.cursor - visibleHeight/2
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(m.notes) {
		end = len(m.notes)
		start = end - visibleHeight
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		note := m.notes[i]

		// Format date
		dateStr := note.Date.Format("Mon, Jan 2, 2006")

		// Calculate days ago
		daysAgo := int(time.Since(note.Date).Hours() / 24)
		var agoStr string
		if daysAgo == 1 {
			agoStr = "yesterday"
		} else if daysAgo < 7 {
			agoStr = fmt.Sprintf("%d days ago", daysAgo)
		} else if daysAgo < 30 {
			weeks := daysAgo / 7
			if weeks == 1 {
				agoStr = "1 week ago"
			} else {
				agoStr = fmt.Sprintf("%d weeks ago", weeks)
			}
		} else {
			months := daysAgo / 30
			if months == 1 {
				agoStr = "1 month ago"
			} else {
				agoStr = fmt.Sprintf("%d months ago", months)
			}
		}

		line := fmt.Sprintf("  %s (%s) - %d words", dateStr, agoStr, note.WordCount)

		if i == m.cursor {
			line = selectedStyle.Render("▸ "+dateStr) +
				fmt.Sprintf(" (%s) - %d words", agoStr, note.WordCount)
		}

		s += line + "\n"
	}

	// Show scroll indicator
	if len(m.notes) > visibleHeight {
		total := len(m.notes)
		position := m.cursor + 1
		s += "\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(fmt.Sprintf("  Showing %d of %d notes", position, total))
	}

	s += "\n" + helpStyle.Render("↑/↓ or j/k navigate • enter view • q quit")

	return s
}

func (m Model) viewModeView() string {
	note := m.notes[m.cursor]

	header := titleStyle.Render(fmt.Sprintf("🌊 %s", note.Date.Format("Monday, January 2, 2006")))
	header += "\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(fmt.Sprintf("%d words", note.WordCount))
	header += "\n"

	footer := helpStyle.Render("↑/↓ or j/k scroll • esc back • q quit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		m.viewport.View(),
		footer,
	)
}

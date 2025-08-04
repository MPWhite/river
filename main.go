package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"
)

type model struct {
	content      []string
	cursor       position
	viewport     viewport
	filename     string
	modified     bool
	lastActivity time.Time
	typingTime   time.Duration
	startTime    time.Time
	statsFile    string
}

type position struct {
	row int
	col int
}

type viewport struct {
	width  int
	height int
}

type stats struct {
	TypingSeconds int `toml:"typing_seconds"`
	WordCount     int `toml:"word_count"`
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func loadOrCreateTodayFile() ([]string, string, error) {
	// Get today's date in YYYY-MM-DD format
	today := time.Now().Format("2006-01-02")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, "", err
	}

	riverDir := filepath.Join(homeDir, "river", "notes")
	filename := filepath.Join(riverDir, today+".md")

	// Ensure river/notes directory exists
	if err := os.MkdirAll(riverDir, 0755); err != nil {
		return nil, "", err
	}

	// Read file if it exists, otherwise create empty content
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create with template
			template := createDailyNoteTemplate()
			return strings.Split(template, "\n"), filename, nil
		}
		return nil, "", err
	}

	// Split content into lines
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	return lines, filename, nil
}

// createDailyNoteTemplate creates a template for new daily notes with date and prompt
func createDailyNoteTemplate() string {
	// List of prompts for daily reflection
	prompts := []string{
		"What are three things you're grateful for today?",
		"What's one small win you can achieve today?",
		"How do you want to feel at the end of today?",
		"What's one thing you learned yesterday that you can apply today?",
		"What would make today great?",
		"What's your main focus for today?",
		"How can you step outside your comfort zone today?",
		"What habit are you building, and how will you practice it today?",
		"Who can you help or connect with today?",
		"What's one thing you've been putting off that you can tackle today?",
		"How will you take care of yourself today?",
		"What creative problem can you solve today?",
		"What would your best self do today?",
		"What's one way you can simplify your day?",
		"How can you bring more joy into your routine today?",
	}

	// Get today's date
	today := time.Now()
	dateStr := today.Format("Monday, January 2, 2006")

	// Select prompt based on day of year to rotate through them
	dayOfYear := today.YearDay()
	promptIndex := (dayOfYear - 1) % len(prompts)
	selectedPrompt := prompts[promptIndex]

	// Create template with ghost text styling (using markdown comments for now)
	// The ghost styling will need to be handled by the terminal rendering
	template := fmt.Sprintf("<!-- %s -->\n<!-- %s -->\n\n", dateStr, selectedPrompt)

	return template
}

func loadStats(statsFile string) (time.Duration, error) {
	data, err := os.ReadFile(statsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var s stats
	if err := toml.Unmarshal(data, &s); err != nil {
		return 0, err
	}

	return time.Duration(s.TypingSeconds) * time.Second, nil
}

func saveStats(statsFile string, typingTime time.Duration, content []string) error {
	// Count words
	wordCount := 0
	for _, line := range content {
		words := strings.Fields(line)
		wordCount += len(words)
	}

	s := stats{
		TypingSeconds: int(typingTime.Seconds()),
		WordCount:     wordCount,
	}

	data, err := toml.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(statsFile, data, 0644)
}

func saveFile(filename string, content []string) error {
	data := strings.Join(content, "\n")
	return os.WriteFile(filename, []byte(data), 0644)
}

func initialModel() model {
	content, filename, err := loadOrCreateTodayFile()
	if err != nil {
		// If there's an error, start with empty content
		content = []string{fmt.Sprintf("Error loading file: %v", err)}
		filename = "error.txt"
	}

	// Position cursor at the end of the file
	lastRow := len(content) - 1
	if lastRow < 0 {
		lastRow = 0
	}
	lastCol := len(content[lastRow])

	// If the last line has content, add a new empty line and position cursor there
	if lastRow >= 0 && len(content[lastRow]) > 0 {
		content = append(content, "")
		lastRow++
		lastCol = 0
	}

	// Create stats filename
	today := time.Now().Format("2006-01-02")
	dir := filepath.Dir(filename)
	statsFile := filepath.Join(dir, ".stats-"+today+".toml")

	// Load existing typing time
	existingTime, _ := loadStats(statsFile)

	now := time.Now()
	return model{
		content:      content,
		cursor:       position{lastRow, lastCol},
		filename:     filename,
		modified:     false,
		lastActivity: now,
		typingTime:   existingTime,
		startTime:    now,
		statsFile:    statsFile,
	}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		// Check if we should update typing time
		if time.Since(m.lastActivity) < time.Minute {
			// Still active, add time since last tick
			m.typingTime += time.Second
		}
		return m, tickCmd()

	case tea.WindowSizeMsg:
		m.viewport.width = msg.Width
		m.viewport.height = msg.Height - 2 // Leave room for status bar

	case tea.KeyMsg:
		// Update last activity time for any key press
		m.lastActivity = time.Now()

		switch msg.Type {
		case tea.KeyCtrlC:
			// Save file and stats before quitting
			if m.modified {
				if err := saveFile(m.filename, m.content); err != nil {
					// Could add error handling here
				}
			}
			// Always save stats
			saveStats(m.statsFile, m.typingTime, m.content)
			return m, tea.Quit

		case tea.KeyCtrlS:
			// Save file and stats
			if err := saveFile(m.filename, m.content); err == nil {
				m.modified = false
			}
			saveStats(m.statsFile, m.typingTime, m.content)

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
			m.modified = true
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
			m.modified = true
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
			m.modified = true
			// Insert space at cursor position
			line := m.content[m.cursor.row]
			m.content[m.cursor.row] = line[:m.cursor.col] + " " + line[m.cursor.col:]
			m.cursor.col++

		case tea.KeyRunes:
			m.modified = true
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

	// Calculate viewport bounds
	maxContentHeight := m.viewport.height - 2 // Reserve 2 lines for status bar
	if maxContentHeight < 1 {
		maxContentHeight = 1
	}

	// Calculate scroll offset to keep cursor visible
	var scrollOffset int
	if m.cursor.row >= maxContentHeight {
		scrollOffset = m.cursor.row - maxContentHeight + 1
	}

	// Display visible content lines
	visibleLines := 0
	for i := scrollOffset; i < len(m.content) && visibleLines < maxContentHeight; i++ {
		line := m.content[i]

		if i == m.cursor.row {
			// Show cursor on current line
			if m.cursor.col < len(line) {
				s.WriteString(line[:m.cursor.col])
				// Blinking cursor effect
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Bold(true).Render("│"))
				s.WriteString(line[m.cursor.col:])
			} else {
				s.WriteString(line)
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Bold(true).Render("│"))
			}
		} else {
			// Check if line is a comment (ghost text)
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "<!--") && strings.HasSuffix(trimmedLine, "-->") {
				// Extract content between <!-- and -->
				content := strings.TrimSuffix(strings.TrimPrefix(trimmedLine, "<!--"), "-->")
				content = strings.TrimSpace(content)
				// Apply ghost styling (dimmed gray)
				ghostStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Italic(true)
				s.WriteString(ghostStyle.Render(content))
			} else {
				s.WriteString(line)
			}
		}
		s.WriteString("\n")
		visibleLines++
	}

	// Add empty lines to fill remaining viewport space
	for i := visibleLines; i < maxContentHeight; i++ {
		s.WriteString("~\n")
	}

	// Calculate word count (excluding ghost text)
	wordCount := 0
	for _, line := range m.content {
		trimmedLine := strings.TrimSpace(line)
		// Skip HTML comments (ghost text)
		if strings.HasPrefix(trimmedLine, "<!--") && strings.HasSuffix(trimmedLine, "-->") {
			continue
		}
		words := strings.Fields(line)
		wordCount += len(words)
	}

	// Format typing time (minutes only)
	minutes := int(m.typingTime.Minutes())
	timeStr := fmt.Sprintf("%dm", minutes)

	// Create progress bar
	targetWords := 500
	progress := float64(wordCount) / float64(targetWords)
	if progress > 1.0 {
		progress = 1.0
	}

	// Calculate available width for progress bar
	// Format: "XXX/500    [████████████████████]    Xm"
	leftText := fmt.Sprintf("%d/%d", wordCount, targetWords)
	rightText := timeStr
	padding := "  " // 2 spaces padding (reduced for small windows)

	// Calculate available width more conservatively
	usedWidth := len(leftText) + len(rightText) + len(padding)*2 + 2 // +2 for brackets
	availableWidth := m.viewport.width - usedWidth

	// Handle very small windows gracefully
	if availableWidth < 5 {
		availableWidth = 5 // very minimum bar width
		padding = " "      // reduce padding further
	} else if availableWidth < 10 {
		availableWidth = 10 // minimum comfortable bar width
	}

	filledWidth := int(progress * float64(availableWidth))

	// Create subtle progress bar
	var progressBar strings.Builder
	progressBar.WriteString("[")

	// Use subtle but visible characters and colors
	for i := 0; i < availableWidth; i++ {
		if i < filledWidth {
			// Slightly brighter for filled portion with a pink tint
			progressBar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#AA6688")).Render("━"))
		} else {
			progressBar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render("─"))
		}
	}
	progressBar.WriteString("]")

	// Style the components with slightly more visible colors
	leftStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	rightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))

	// Combine into status bar
	statusBar := leftStyle.Render(leftText) + padding + progressBar.String() + padding + rightStyle.Render(rightText)

	// Calculate visual length (without ANSI codes) for width checking
	visualLength := len(leftText) + len(padding)*2 + availableWidth + 2 + len(rightText) // +2 for brackets

	// Ensure status bar doesn't exceed terminal width based on visual length
	if visualLength > m.viewport.width {
		// Fallback to simple status for very small windows
		statusBar = fmt.Sprintf("%d words %s", wordCount, timeStr)
		if len(statusBar) > m.viewport.width {
			statusBar = fmt.Sprintf("%dw", wordCount) // Ultra minimal
		}
	}

	s.WriteString("\n")
	s.WriteString(statusBar)

	return s.String()
}

func main() {
	// Check for command line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "stats":
			// Run stats view
			p := tea.NewProgram(initStatsModel(), tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				fmt.Printf("Error: %v", err)
				os.Exit(1)
			}
			return
		case "think":
			// Run AI analysis with categorized TODOs
			if err := generateTodos(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "analyze":
			// Run AI analysis for insights
			if err := generateInsights(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		case "todo":
			// Run AI analysis focused on TODOs
			if err := generateSimpleTodos(); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// Default behavior - run the note editor
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}

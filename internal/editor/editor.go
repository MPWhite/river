package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	textarea  textarea.Model
	progress  progress.Model
	filename  string
	prompt    string // Today's prompt
	width     int
	height    int
	ready     bool
	wordCount int
}

func loadTodayFile() (content string, prompt string, filename string) {
	// Get today's filename
	today := time.Now().Format("2006-01-02")
	homeDir, _ := os.UserHomeDir()
	riverDir := filepath.Join(homeDir, "river", "notes")
	filename = filepath.Join(riverDir, today+".md")

	// Ensure directory exists
	os.MkdirAll(riverDir, 0755)

	// Try to read the file
	data, err := os.ReadFile(filename)
	if err != nil {
		// File doesn't exist - create with template
		prompt = getPromptForToday()
		template := fmt.Sprintf("<!-- %s -->\n<!-- %s -->\n\n",
			time.Now().Format("Monday, January 2, 2006"),
			prompt)
		os.WriteFile(filename, []byte(template), 0644)
		return "", prompt, filename
	}

	// File exists - parse it
	lines := strings.Split(string(data), "\n")
	var contentLines []string
	var prompts []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
			// Extract prompt text
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!--"), "-->"))
			if text != "" && !strings.Contains(text, "2025") && !strings.Contains(text, "2024") {
				prompts = append(prompts, text)
			}
		} else {
			contentLines = append(contentLines, line)
		}
	}

	// Join prompt lines
	if len(prompts) > 0 {
		prompt = strings.Join(prompts, " ")
	}

	// Remove leading empty lines
	for len(contentLines) > 0 && strings.TrimSpace(contentLines[0]) == "" {
		contentLines = contentLines[1:]
	}

	return strings.Join(contentLines, "\n"), prompt, filename
}

func getPromptForToday() string {
	// Try to load prompts from the AI-generated prompts file
	homeDir, _ := os.UserHomeDir()
	riverDir := filepath.Join(homeDir, "river", "notes")
	promptsFile := filepath.Join(riverDir, ".prompts")
	
	// Check if prompts file exists and was generated recently (within 7 days)
	fileInfo, err := os.Stat(promptsFile)
	if err == nil {
		// Check if file is less than 7 days old
		if time.Since(fileInfo.ModTime()) < 7*24*time.Hour {
			// Read the prompts file
			data, err := os.ReadFile(promptsFile)
			if err == nil {
				lines := strings.Split(string(data), "\n")
				var prompts []string
				
				for _, line := range lines {
					// Skip header line and empty lines
					if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
						continue
					}
					// Extract prompt text (format: "N. Prompt text")
					if idx := strings.Index(line, ". "); idx > 0 {
						prompt := strings.TrimSpace(line[idx+2:])
						if prompt != "" {
							prompts = append(prompts, prompt)
						}
					}
				}
				
				if len(prompts) > 0 {
					// Use day of year modulo number of prompts to select one
					dayOfYear := time.Now().YearDay()
					return prompts[(dayOfYear-1)%len(prompts)]
				}
			}
		}
	}
	
	// Fallback to default prompts if no AI prompts available
	defaultPrompts := []string{
		"What are three things you're grateful for today?",
		"What's one small win you can achieve today?",
		"How do you want to feel at the end of today?",
		"What would make today great?",
		"What's your main focus for today?",
		"What challenge did you overcome recently?",
		"What's bringing you joy right now?",
		"What lesson have you learned this week?",
		"What are you looking forward to?",
		"How have you grown lately?",
	}

	dayOfYear := time.Now().YearDay()
	return defaultPrompts[(dayOfYear-1)%len(defaultPrompts)]
}

func countWords(text string) int {
	if text == "" {
		return 0
	}
	words := strings.Fields(text)
	return len(words)
}

func saveFile(filename string, content string, prompt string) error {
	// Reconstruct file with prompt at top
	var fullContent strings.Builder

	// Add date and prompt as comments
	fullContent.WriteString(fmt.Sprintf("<!-- %s -->\n", time.Now().Format("Monday, January 2, 2006")))
	if prompt != "" {
		fullContent.WriteString(fmt.Sprintf("<!-- %s -->\n", prompt))
	}
	fullContent.WriteString("\n")
	fullContent.WriteString(content)

	return os.WriteFile(filename, []byte(fullContent.String()), 0644)
}

func NewInitialModel() Model {
	// Load today's file
	content, prompt, filename := loadTodayFile()

	// Create textarea
	ta := textarea.New()
	ta.Placeholder = "Start writing..."
	ta.SetValue(content)
	ta.Focus()

	// Simple styling
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	ta.ShowLineNumbers = false
	ta.CharLimit = 0

	// Create progress bar
	prog := progress.New(progress.WithDefaultGradient())

	// Calculate initial word count
	wordCount := countWords(content)

	return Model{
		textarea:  ta,
		progress:  prog,
		filename:  filename,
		prompt:    prompt,
		wordCount: wordCount,
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Set progress bar width
		m.progress.Width = m.width - 4

		// Calculate textarea size
		promptHeight := 0
		if m.prompt != "" {
			// Calculate actual height of prompt box with wrapping
			// We need to render the prompt to get accurate height
			promptBoxWidth := m.width - 6
			if promptBoxWidth > 0 {
				promptBox := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("62")).
					Foreground(lipgloss.Color("251")).
					Padding(0, 2).
					Width(promptBoxWidth)
				promptDisplay := promptBox.Render("💭 " + m.prompt)
				promptHeight = lipgloss.Height(promptDisplay) + 1 // Add margin
			} else {
				promptHeight = 4 // Default
			}
		}
		progressHeight := 2 // Progress bar only

		textAreaHeight := m.height - promptHeight - progressHeight - 1
		if textAreaHeight < 10 {
			textAreaHeight = 10
		}

		m.textarea.SetWidth(m.width - 4)
		m.textarea.SetHeight(textAreaHeight)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Save and quit
			content := m.textarea.Value()
			saveFile(m.filename, content, m.prompt)
			return m, tea.Quit

		case tea.KeyCtrlS:
			// Save
			content := m.textarea.Value()
			saveFile(m.filename, content, m.prompt)

		default:
			// Pass to textarea
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)

			// Update word count
			m.wordCount = countWords(m.textarea.Value())
		}

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd

	default:
		// Pass to textarea
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Build view parts
	var parts []string

	// Prompt box (if we have one)
	if m.prompt != "" {
		promptBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Foreground(lipgloss.Color("251")).
			Padding(0, 2).
			Width(m.width - 6)

		promptDisplay := promptBox.Render("💭 " + m.prompt)
		// Add margin after rendering to avoid top border cutoff
		promptWithMargin := lipgloss.NewStyle().
			Margin(0, 2, 0, 2).
			Render(promptDisplay)
		parts = append(parts, promptWithMargin)
	}

	// Editor
	editorBox := lipgloss.NewStyle().
		Padding(0, 2).
		Margin(0, 0)

	parts = append(parts, editorBox.Render(m.textarea.View()))

	// Progress bar
	const targetWords = 500
	percent := float64(m.wordCount) / float64(targetWords)
	if percent > 1.0 {
		percent = 1.0
	}

	progressBox := lipgloss.NewStyle().
		Padding(0, 2).
		Margin(0, 0)

	parts = append(parts, progressBox.Render(m.progress.ViewAs(percent)))

	// Minimal help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 2)

	helpText := fmt.Sprintf("%d words • ^S save • ^C quit", m.wordCount)
	parts = append(parts, helpStyle.Render(helpText))

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

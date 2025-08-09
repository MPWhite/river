package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"
)

type Model struct {
	textarea     textarea.Model
	viewport     viewport.Model
	filename     string
	modified     bool
	lastActivity time.Time
	typingTime   time.Duration
	startTime    time.Time
	statsFile    string
	progress     progress.Model
	width        int
	height       int
	wordCount    int
	ready        bool
	ghostText    []string // Store the ghost text separately
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

func loadOrCreateTodayFile() (string, string, error) {
	today := time.Now().Format("2006-01-02")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	riverDir := filepath.Join(homeDir, "river", "notes")
	filename := filepath.Join(riverDir, today+".md")

	if err := os.MkdirAll(riverDir, 0755); err != nil {
		return "", "", err
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			template := createDailyNoteTemplate()
			return template, filename, nil
		}
		return "", "", err
	}

	return string(content), filename, nil
}

func loadPersonalizedPrompts() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	riverDir := filepath.Join(homeDir, "river", "notes")
	promptsFile := filepath.Join(riverDir, ".prompts")

	content, err := os.ReadFile(promptsFile)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	var prompts []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 3 && line[1] == '.' && line[0] >= '1' && line[0] <= '9' {
			prompt := strings.TrimSpace(line[3:])
			if prompt != "" {
				prompts = append(prompts, prompt)
			}
		}
	}

	return prompts
}

func createDailyNoteTemplate() string {
	personalizedPrompts := loadPersonalizedPrompts()

	defaultPrompts := []string{
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

	prompts := defaultPrompts
	if len(personalizedPrompts) > 0 {
		prompts = personalizedPrompts
	}

	today := time.Now()
	dateStr := today.Format("Monday, January 2, 2006")
	dayOfYear := today.YearDay()
	promptIndex := (dayOfYear - 1) % len(prompts)
	selectedPrompt := prompts[promptIndex]

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

func saveStats(statsFile string, typingTime time.Duration, wordCount int) error {
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

func saveFile(filename string, content string, ghostText []string) error {
	// Reconstruct the file with ghost text at the beginning
	var fullContent strings.Builder
	
	// Add ghost text as HTML comments
	for _, ghost := range ghostText {
		fullContent.WriteString(fmt.Sprintf("<!-- %s -->\n", ghost))
	}
	
	// Add actual content
	fullContent.WriteString(content)
	
	return os.WriteFile(filename, []byte(fullContent.String()), 0644)
}

func NewInitialModel() Model {
	content, filename, err := loadOrCreateTodayFile()
	if err != nil {
		content = fmt.Sprintf("Error loading file: %v", err)
		filename = "error.txt"
	}

	// Process content to separate ghost text from actual content
	actualContent, ghostText := processGhostText(content)

	// Create a beautiful textarea with Lipgloss styling
	ta := textarea.New()
	ta.Placeholder = "Start writing your thoughts..."
	ta.SetValue(actualContent)
	ta.Focus()
	
	// Style the textarea
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))
	ta.BlurredStyle.Base = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("239"))
	
	// Prompt styling
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
	ta.BlurredStyle.Placeholder = lipgloss.NewStyle().
		Foreground(lipgloss.Color("239"))
	
	// Set unlimited height for the textarea
	ta.SetHeight(20)
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // No limit

	today := time.Now().Format("2006-01-02")
	dir := filepath.Dir(filename)
	statsFile := filepath.Join(dir, ".stats-"+today+".toml")

	existingTime, _ := loadStats(statsFile)

	// Create a gradient progress bar
	prog := progress.New(progress.WithDefaultGradient())

	now := time.Now()
	return Model{
		textarea:     ta,
		viewport:     viewport.New(80, 20),
		filename:     filename,
		modified:     false,
		lastActivity: now,
		typingTime:   existingTime,
		startTime:    now,
		statsFile:    statsFile,
		progress:     prog,
		wordCount:    countWords(actualContent),
		ghostText:    ghostText,
	}
}

// processGhostText separates ghost text (HTML comments) from actual content
func processGhostText(content string) (actualContent string, ghostText []string) {
	lines := strings.Split(content, "\n")
	var actualLines []string
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
			// Extract ghost text
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!--"), "-->"))
			if text != "" {
				ghostText = append(ghostText, text)
			}
		} else {
			// Keep actual content
			actualLines = append(actualLines, line)
		}
	}
	
	return strings.Join(actualLines, "\n"), ghostText
}

func countWords(content string) int {
	// Skip HTML comment lines (ghost text)
	lines := strings.Split(content, "\n")
	wordCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip HTML comments (ghost text)
		if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
			continue
		}
		words := strings.Fields(line)
		wordCount += len(words)
	}
	return wordCount
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		textarea.Blink,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tickMsg:
		if time.Since(m.lastActivity) < time.Minute {
			m.typingTime += time.Second
		}
		cmds = append(cmds, tickCmd())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Update component sizes
		headerHeight := 3
		footerHeight := 3
		textAreaHeight := msg.Height - headerHeight - footerHeight
		if textAreaHeight < 5 {
			textAreaHeight = 5
		}
		
		m.textarea.SetWidth(msg.Width - 4)
		m.textarea.SetHeight(textAreaHeight)
		
		m.progress.Width = msg.Width - 20
		if m.progress.Width < 20 {
			m.progress.Width = 20
		}
		
		if !m.ready {
			m.viewport = viewport.New(msg.Width, textAreaHeight)
			m.viewport.YPosition = 0
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = textAreaHeight
		}

	case tea.KeyMsg:
		m.lastActivity = time.Now()
		
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			// Save before quitting
			if m.modified {
				content := m.textarea.Value()
				_ = saveFile(m.filename, content, m.ghostText)
				_ = saveStats(m.statsFile, m.typingTime, m.wordCount)
			}
			return m, tea.Quit
			
		case tea.KeyCtrlS:
			// Save file
			content := m.textarea.Value()
			if err := saveFile(m.filename, content, m.ghostText); err == nil {
				m.modified = false
			}
			_ = saveStats(m.statsFile, m.typingTime, m.wordCount)
			
		default:
			// Update textarea
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			cmds = append(cmds, cmd)
			
			// Update word count and modified flag
			newContent := m.textarea.Value()
			m.wordCount = countWords(newContent)
			m.modified = true
		}
		
	default:
		// Pass through to textarea
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	
	// Create header with title
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Background(lipgloss.Color("235")).
		Padding(0, 2).
		Width(m.width).
		Align(lipgloss.Center)
	
	dateStr := time.Now().Format("Monday, January 2, 2006")
	header := headerStyle.Render(fmt.Sprintf("✍️  River - %s", dateStr))
	
	// Check if we have ghost text to display above the editor
	ghostText := m.extractGhostText()
	
	// Main editor area
	editorBox := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width)
	
	// If there's ghost text, show it above the editor
	var mainContent string
	if ghostText != "" {
		// Create a more visible ghost text box
		ghostBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Foreground(lipgloss.Color("245")).  // Brighter gray
			Background(lipgloss.Color("235")).  // Subtle background
			Italic(true).
			Padding(0, 2).
			Margin(1, 2).
			Width(m.width - 6)
		
		ghostDisplay := ghostBox.Render("💭 " + ghostText)
		editor := editorBox.Render(m.textarea.View())
		mainContent = lipgloss.JoinVertical(lipgloss.Left, ghostDisplay, editor)
	} else {
		mainContent = editorBox.Render(m.textarea.View())
	}
	
	// Footer with stats
	footer := m.renderFooter()
	
	// Combine all elements
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		mainContent,
		footer,
	)
}

func (m Model) extractGhostText() string {
	if len(m.ghostText) > 0 {
		return strings.Join(m.ghostText, " • ")
	}
	return ""
}

func (m Model) renderFooter() string {
	// Progress bar for word goal
	goal := 500
	progress := float64(m.wordCount) / float64(goal)
	if progress > 1.0 {
		progress = 1.0
	}
	
	// Create footer box
	footerBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderLeft(false).
		BorderRight(false).
		BorderBottom(false).
		BorderForeground(lipgloss.Color("239")).
		Padding(0, 2).
		Width(m.width)
	
	// Stats line
	statsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	valueStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	
	minutes := int(m.typingTime.Minutes())
	timeStr := fmt.Sprintf("%dm", minutes)
	
	statsLine := lipgloss.JoinHorizontal(lipgloss.Top,
		statsStyle.Render("Words: "),
		valueStyle.Render(fmt.Sprintf("%d/%d", m.wordCount, goal)),
		statsStyle.Render(" • Time: "),
		valueStyle.Render(timeStr),
		statsStyle.Render(" • "),
		valueStyle.Render(fmt.Sprintf("%.0f%%", progress*100)),
	)
	
	// Progress bar
	progressBar := m.progress.ViewAs(progress)
	
	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Italic(true)
	help := helpStyle.Render("ctrl+s: save • ctrl+c: quit")
	
	footerContent := lipgloss.JoinVertical(lipgloss.Left,
		progressBar,
		lipgloss.JoinHorizontal(lipgloss.Top, statsLine, "  ", help),
	)
	
	return footerBox.Render(footerContent)
}
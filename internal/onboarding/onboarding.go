package onboarding

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()
	helpStyle    = blurredStyle.Copy()

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginBottom(2)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))
)

type Model struct {
	textInput textinput.Model
	err       error
	saved     bool
	width     int
	height    int
}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "sk-ant-api03-..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'

	return Model{
		textInput: ti,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			apiKey := strings.TrimSpace(m.textInput.Value())
			if apiKey != "" {
				if err := saveAPIKey(apiKey); err != nil {
					m.err = err
				} else {
					m.saved = true
					return m, tea.Quit
				}
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.saved {
		return successStyle.Render("\n✅ API key saved successfully!\n\nYou can now use AI features:\n  • river prompts - Generate personalized journal prompts\n  • river think - Generate categorized TODOs\n  • river analyze - Get insights from your notes\n\n")
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("🌊 Welcome to River"))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("Let's set up AI-powered features"))
	b.WriteString("\n\n")

	b.WriteString("River can use AI to:\n")
	b.WriteString("  • Generate personalized journal prompts based on your writing\n")
	b.WriteString("  • Extract actionable TODOs from your notes\n")
	b.WriteString("  • Provide insights and patterns from your journaling\n\n")

	b.WriteString("Enter your Anthropic API key:\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(warningStyle.Render(fmt.Sprintf("Error: %v\n", m.err)))
	}

	b.WriteString(helpStyle.Render("(Press Enter to save, Esc to skip)"))

	return b.String()
}

func saveAPIKey(apiKey string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	riverDir := filepath.Join(homeDir, "river")
	if err := os.MkdirAll(riverDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(riverDir, ".config")

	// Read existing config if it exists
	existingConfig := make(map[string]string)
	if data, err := os.ReadFile(configPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				existingConfig[parts[0]] = parts[1]
			}
		}
	}

	// Update with new API key
	existingConfig["ANTHROPIC_API_KEY"] = apiKey

	// Write back config
	var configContent strings.Builder
	for key, value := range existingConfig {
		configContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}

	return os.WriteFile(configPath, []byte(configContent.String()), 0600)
}

func LoadAPIKey() string {
	// First check environment variable
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		return apiKey
	}

	// Then check config file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	configPath := filepath.Join(homeDir, "river", ".config")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == "ANTHROPIC_API_KEY" {
			return strings.TrimSpace(parts[1])
		}
	}

	return ""
}

func NeedsOnboarding() bool {
	return LoadAPIKey() == ""
}

func RunOnboarding() error {
	p := tea.NewProgram(NewModel())
	_, err := p.Run()
	return err
}

// Simple CLI version for non-interactive environments
func RunCLIOnboarding() error {
	fmt.Println("🌊 Welcome to River")
	fmt.Println("\n If you'd like to use AI features, please provide an Anthropic API key")
	fmt.Print("\nEnter your Anthropic API key (or press Enter to skip): ")

	reader := bufio.NewReader(os.Stdin)
	apiKey, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		fmt.Println("\nSkipping API key setup. You can set it later by:")
		fmt.Println("  • Running 'river onboard'")
		fmt.Println("  • Setting the ANTHROPIC_API_KEY environment variable")
		return nil
	}

	if err := saveAPIKey(apiKey); err != nil {
		return fmt.Errorf("failed to save API key: %v", err)
	}

	fmt.Println("\n✅ API key saved successfully!")
	fmt.Println("\nYou can now use AI features:")
	fmt.Println("  • river prompts - Generate personalized journal prompts")
	fmt.Println("  • river think - Generate categorized TODOs")
	fmt.Println("  • river analyze - Get insights from your notes")

	return nil
}

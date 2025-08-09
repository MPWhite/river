package statsui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type statsModel struct {
	stats   aggregatedStats
	loading bool
	error   error
	width   int
	height  int
}

type aggregatedStats struct {
	totalWords    int
	totalDays     int
	currentStreak int
}

func InitModel() statsModel {
	return statsModel{
		loading: true,
	}
}

func (m statsModel) Init() tea.Cmd {
	return loadStatsCmd()
}

func loadStatsCmd() tea.Cmd {
	return func() tea.Msg {
		stats, err := collectAllStats()
		if err != nil {
			return statsErrorMsg{err}
		}
		return statsLoadedMsg{stats}
	}
}

type statsLoadedMsg struct{ stats aggregatedStats }
type statsErrorMsg struct{ err error }

func (m statsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case statsLoadedMsg:
		m.stats = msg.stats
		m.loading = false
	case statsErrorMsg:
		m.error = msg.err
		m.loading = false
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" || msg.String() == "esc" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m statsModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Loading...")
	}
	if m.error != nil {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(fmt.Sprintf("Error: %v", m.error))
	}
	
	content := lipgloss.NewStyle().
		Padding(2).
		Render(fmt.Sprintf("Words: %d\nDays: %d\nStreak: %d\n\nPress q to quit", 
			m.stats.totalWords,
			m.stats.totalDays,
			m.stats.currentStreak))
	
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(content)
}

func collectAllStats() (aggregatedStats, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return aggregatedStats{}, err
	}
	
	riverDir := filepath.Join(homeDir, "river", "notes")
	files, err := filepath.Glob(filepath.Join(riverDir, "*.md"))
	if err != nil {
		return aggregatedStats{}, err
	}
	
	var stats aggregatedStats
	dateMap := make(map[string]bool)
	
	for _, file := range files {
		base := filepath.Base(file)
		dateStr := strings.TrimSuffix(base, ".md")
		if _, err := time.Parse("2006-01-02", dateStr); err != nil {
			continue
		}
		
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		words := len(strings.Fields(string(content)))
		stats.totalWords += words
		dateMap[dateStr] = true
	}
	
	stats.totalDays = len(dateMap)
	stats.currentStreak = calculateStreak(dateMap)
	
	return stats, nil
}

func calculateStreak(dateMap map[string]bool) int {
	streak := 0
	date := time.Now()
	
	// Check if we have today or yesterday
	todayKey := date.Format("2006-01-02")
	yesterdayKey := date.AddDate(0, 0, -1).Format("2006-01-02")
	
	if !dateMap[todayKey] && !dateMap[yesterdayKey] {
		return 0
	}
	
	// Start from today if we have it, otherwise yesterday
	if !dateMap[todayKey] {
		date = date.AddDate(0, 0, -1)
	}
	
	// Count backwards
	for {
		key := date.Format("2006-01-02")
		if !dateMap[key] {
			break
		}
		streak++
		date = date.AddDate(0, 0, -1)
	}
	
	return streak
}
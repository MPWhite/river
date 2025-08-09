package statsui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/charmbracelet/lipgloss/table"
)

type statsModel struct {
	stats      aggregatedStats
	loading    bool
	error      error
	width      int
	height     int
	spinner    spinner.Model
	progress   progress.Model
	viewport   viewport.Model
	ready      bool
}

type aggregatedStats struct {
	totalWords    int
	totalDays     int
	currentStreak int
	todayWords    int
	recentNotes   []noteInfo
	weeklyTotals  []weekInfo
}

type noteInfo struct {
	date  string
	words int
}

type weekInfo struct {
	week  string
	words int
	days  int
}

func InitModel() statsModel {
	// Create a nice spinner for loading
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	
	// Create a gradient progress bar
	p := progress.New(progress.WithDefaultGradient())
	
	return statsModel{
		loading:  true,
		spinner:  s,
		progress: p,
		viewport: viewport.New(80, 20),
	}
}

func (m statsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadStatsCmd(),
	)
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
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		if !m.ready {
			// Initialize viewport with proper dimensions
			m.viewport = viewport.New(msg.Width-4, msg.Height-10)
			m.viewport.YPosition = 0
			m.ready = true
		} else {
			m.viewport.Width = msg.Width - 4
			m.viewport.Height = msg.Height - 10
		}
		
		// Update progress bar width
		m.progress.Width = msg.Width - 20
		if m.progress.Width < 20 {
			m.progress.Width = 20
		}
		
	case statsLoadedMsg:
		m.stats = msg.stats
		m.loading = false
		m.viewport.SetContent(m.renderContent())
		
	case statsErrorMsg:
		m.error = msg.err
		m.loading = false
		
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "j", "down":
			m.viewport.LineDown(1)
		case "k", "up":
			m.viewport.LineUp(1)
		case "d":
			m.viewport.HalfViewDown()
		case "u":
			m.viewport.HalfViewUp()
		}
		
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	
	// Update viewport
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

func (m statsModel) View() string {
	if m.loading {
		// Create a nice loading box
		loadingBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(2, 4).
			Width(40).
			Align(lipgloss.Center)
		
		content := lipgloss.JoinVertical(lipgloss.Center,
			m.spinner.View(),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Loading your writing stats..."),
		)
		
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			loadingBox.Render(content),
		)
	}
	
	if m.error != nil {
		errorBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("9")).
			Padding(1, 3).
			Width(50)
		
		errorContent := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true).
			Render(fmt.Sprintf("✗ Error: %v", m.error))
		
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			errorBox.Render(errorContent),
		)
	}
	
	// Create a beautiful header with gradient
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Background(lipgloss.Color("235")).
		Padding(0, 2).
		Width(m.width).
		Align(lipgloss.Center)
	
	header := headerStyle.Render("✨ River Writing Stats ✨")
	
	// Progress section with nice styling
	progressSection := m.renderProgressSection()
	
	// Main content in viewport
	mainContent := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("239")).
		Padding(1).
		Render(m.viewport.View())
	
	// Footer with help
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(m.width).
		Align(lipgloss.Center)
	
	footer := footerStyle.Render("j/k: scroll • d/u: page • q: quit")
	
	// Combine everything with proper spacing
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		progressSection,
		"",
		mainContent,
		footer,
	)
}

func (m statsModel) renderProgressSection() string {
	// Create a nice box for today's progress
	progressBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(m.width - 4)
	
	goal := 500
	progress := float64(m.stats.todayWords) / float64(goal)
	if progress > 1.0 {
		progress = 1.0
	}
	
	// Style the progress info
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	valueStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	
	progressInfo := lipgloss.JoinHorizontal(lipgloss.Top,
		labelStyle.Render("Today's Progress: "),
		valueStyle.Render(fmt.Sprintf("%d/%d words", m.stats.todayWords, goal)),
		labelStyle.Render(fmt.Sprintf(" (%.0f%%)", progress*100)),
	)
	
	progressContent := lipgloss.JoinVertical(lipgloss.Left,
		progressInfo,
		m.progress.ViewAs(progress),
	)
	
	return progressBox.Render(progressContent)
}

func (m statsModel) renderContent() string {
	var sections []string
	
	// Overall stats section with a nice table
	sections = append(sections, m.renderStatsTable())
	
	// Weekly stats table
	if len(m.stats.weeklyTotals) > 0 {
		sections = append(sections, "", m.renderWeeklyTable())
	}
	
	// Recent notes as a styled list
	if len(m.stats.recentNotes) > 0 {
		sections = append(sections, "", m.renderRecentNotesList())
	}
	
	return strings.Join(sections, "\n")
}

func (m statsModel) renderStatsTable() string {
	// Create a beautiful stats table using lipgloss/table
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("239"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().
					Foreground(lipgloss.Color("212")).
					Bold(true).
					Align(lipgloss.Center)
			}
			if col == 0 {
				return lipgloss.NewStyle().
					Foreground(lipgloss.Color("241"))
			}
			return lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Bold(true).
				Align(lipgloss.Right)
		}).
		Headers("Metric", "Value").
		Row("Total Words", fmt.Sprintf("%d", m.stats.totalWords)).
		Row("Days Written", fmt.Sprintf("%d", m.stats.totalDays)).
		Row("Current Streak", fmt.Sprintf("%d days", m.stats.currentStreak)).
		Row("Average Words/Day", fmt.Sprintf("%.0f", float64(m.stats.totalWords)/float64(max(m.stats.totalDays, 1))))
	
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("📊 Overall Statistics"),
		t.Render(),
	)
}

func (m statsModel) renderWeeklyTable() string {
	// Create weekly stats table
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("239"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == 0 {
				return lipgloss.NewStyle().
					Foreground(lipgloss.Color("212")).
					Bold(true).
					Align(lipgloss.Center)
			}
			return lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Align(lipgloss.Right)
		}).
		Headers("Week", "Words", "Days")
	
	for _, week := range m.stats.weeklyTotals {
		t.Row(week.week, fmt.Sprintf("%d", week.words), fmt.Sprintf("%d", week.days))
	}
	
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("📅 Weekly Summary"),
		t.Render(),
	)
}

func (m statsModel) renderRecentNotesList() string {
	// Create a styled list of recent notes
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)
	
	// Create list items with custom styling
	items := make([]any, len(m.stats.recentNotes))
	for i, note := range m.stats.recentNotes {
		// Create a mini bar visualization
		bar := m.renderMiniBar(note.words, 1000, 15)
		item := fmt.Sprintf("%-10s %s %5d words", note.date, bar, note.words)
		items[i] = item
	}
	
	// Use lipgloss/list for beautiful list rendering
	l := list.New(items...).
		Enumerator(list.Bullet).
		EnumeratorStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205"))).
		ItemStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("252")))
	
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("📝 Recent Notes"),
		l.String(),
	)
}

func (m statsModel) renderMiniBar(current, max, width int) string {
	progress := float64(current) / float64(max)
	if progress > 1.0 {
		progress = 1.0
	}
	filled := int(progress * float64(width))
	
	// Use gradient colors for the bar
	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			// Gradient from pink to purple
			color := 205 - (i * 2)
			if color < 165 {
				color = 165
			}
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("%d", color))).Render("█"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("237")).Render("░"))
		}
	}
	return bar.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
	weekMap := make(map[string]*weekInfo)
	var notes []noteInfo
	today := time.Now().Format("2006-01-02")
	
	for _, file := range files {
		base := filepath.Base(file)
		dateStr := strings.TrimSuffix(base, ".md")
		
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		
		words := len(strings.Fields(string(content)))
		stats.totalWords += words
		dateMap[dateStr] = true
		
		// Track today's words
		if dateStr == today {
			stats.todayWords = words
		}
		
		// Track weekly totals
		weekStart := date
		for weekStart.Weekday() != time.Sunday {
			weekStart = weekStart.AddDate(0, 0, -1)
		}
		weekKey := weekStart.Format("Jan 2")
		
		if w, exists := weekMap[weekKey]; exists {
			w.words += words
			w.days++
		} else {
			weekMap[weekKey] = &weekInfo{
				week:  weekKey,
				words: words,
				days:  1,
			}
		}
		
		// Keep track of recent notes (last 7 days)
		if time.Since(date) <= 7*24*time.Hour {
			notes = append(notes, noteInfo{
				date:  date.Format("Jan 2"),
				words: words,
			})
		}
	}
	
	// Convert week map to slice and limit to last 4 weeks
	for _, week := range weekMap {
		stats.weeklyTotals = append(stats.weeklyTotals, *week)
	}
	
	// Sort weekly totals by date (simple bubble sort)
	for i := 0; i < len(stats.weeklyTotals)-1; i++ {
		for j := i + 1; j < len(stats.weeklyTotals); j++ {
			if stats.weeklyTotals[i].week > stats.weeklyTotals[j].week {
				stats.weeklyTotals[i], stats.weeklyTotals[j] = stats.weeklyTotals[j], stats.weeklyTotals[i]
			}
		}
	}
	
	// Keep only last 4 weeks
	if len(stats.weeklyTotals) > 4 {
		stats.weeklyTotals = stats.weeklyTotals[len(stats.weeklyTotals)-4:]
	}
	
	// Sort recent notes by date
	for i := 0; i < len(notes)-1; i++ {
		for j := i + 1; j < len(notes); j++ {
			if notes[i].date > notes[j].date {
				notes[i], notes[j] = notes[j], notes[i]
			}
		}
	}
	
	stats.recentNotes = notes
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
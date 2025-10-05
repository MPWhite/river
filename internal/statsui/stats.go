package statsui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattwhite/river-go/internal/ai"
)

const (
	dailyGoal = 500
)

// Color palette - calm, reflective, motivating
var (
	colorSubtle     = lipgloss.AdaptiveColor{Light: "#999999", Dark: "#666666"}
	colorText       = lipgloss.AdaptiveColor{Light: "#262626", Dark: "#E0E0E0"}
	colorAccent     = lipgloss.AdaptiveColor{Light: "#5B8DBE", Dark: "#7BA4DB"}
	colorSuccess    = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#66BB6A"}
	colorWarning    = lipgloss.AdaptiveColor{Light: "#F57C00", Dark: "#FFB74D"}
	colorMissing    = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#EF5350"}
	colorBackground = lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#1A1A1A"}
)

type zoomLevel int

const (
	zoom3Days zoomLevel = iota
	zoom7Days
	zoom30Days
	zoom90Days
	zoomAll
)

func (z zoomLevel) String() string {
	switch z {
	case zoom3Days:
		return "3 days"
	case zoom7Days:
		return "7 days"
	case zoom30Days:
		return "30 days"
	case zoom90Days:
		return "90 days"
	case zoomAll:
		return "all time"
	default:
		return ""
	}
}

func (z zoomLevel) Days() int {
	switch z {
	case zoom3Days:
		return 3
	case zoom7Days:
		return 7
	case zoom30Days:
		return 30
	case zoom90Days:
		return 90
	case zoomAll:
		return 0 // 0 means all
	default:
		return 3
	}
}

type keyMap struct {
	Quit     key.Binding
	ZoomIn   key.Binding
	ZoomOut  key.Binding
	Refresh  key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
		key.WithHelp("q", "quit"),
	),
	ZoomIn: key.NewBinding(
		key.WithKeys("-", "left"),
		key.WithHelp("-/←", "zoom in"),
	),
	ZoomOut: key.NewBinding(
		key.WithKeys("=", "right"),
		key.WithHelp("=/→", "zoom out"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh insight"),
	),
}

type Model struct {
	width          int
	height         int
	loading        bool
	error          error
	stats          *stats
	zoomLevel      zoomLevel
	loadingInsight bool
}

type stats struct {
	entries       []entry
	totalEntries  int
	totalWords    int
	firstDate     time.Time
	lastDate      time.Time
	currentStreak int
	avgWords      int
	last7Days     []dayData
	insight       string
}

type entry struct {
	date  time.Time
	words int
}

type dayData struct {
	date    time.Time
	words   int
	missing bool
}

func InitModel() Model {
	return Model{
		loading:   true,
		zoomLevel: zoom3Days,
	}
}

func (m Model) Init() tea.Cmd {
	return loadStats
}

type statsMsg struct {
	stats *stats
	err   error
}

func loadStats() tea.Msg {
	stats, err := collectStats()
	return statsMsg{stats: stats, err: err}
}

type insightMsg struct {
	insight string
	err     error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case statsMsg:
		m.loading = false
		if msg.err != nil {
			m.error = msg.err
		} else {
			m.stats = msg.stats
		}

	case insightMsg:
		m.loadingInsight = false
		if msg.err == nil && m.stats != nil {
			m.stats.insight = msg.insight
		}

	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, keys.ZoomIn) {
			if m.zoomLevel > zoom3Days {
				m.zoomLevel--
				m.loadingInsight = true
				return m, refreshInsightCmd(m.stats, m.zoomLevel)
			}
		}
		if key.Matches(msg, keys.ZoomOut) {
			if m.zoomLevel < zoomAll {
				m.zoomLevel++
				m.loadingInsight = true
				return m, refreshInsightCmd(m.stats, m.zoomLevel)
			}
		}
		if key.Matches(msg, keys.Refresh) {
			m.loadingInsight = true
			return m, refreshInsightCmd(m.stats, m.zoomLevel)
		}
	}

	return m, nil
}

func refreshInsightCmd(s *stats, zoom zoomLevel) tea.Cmd {
	return func() tea.Msg {
		if s == nil {
			return insightMsg{err: fmt.Errorf("no stats")}
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return insightMsg{err: err}
		}

		riverDir := filepath.Join(homeDir, "river", "notes")
		cacheFile := filepath.Join(riverDir, fmt.Sprintf(".insight-cache-%s", zoom.String()))

		// Delete old cache to force refresh
		os.Remove(cacheFile)

		// Generate new insight
		days := zoom.Days()
		if days == 0 {
			days = 365 // Cap at 1 year for "all time"
		}

		recentNotes, err := getRecentNotesForDays(days)
		if err != nil || recentNotes == "" {
			return insightMsg{err: err}
		}

		insight, err := ai.GetStatsInsight(s.totalWords, s.totalEntries, s.currentStreak, s.avgWords, recentNotes)
		if err != nil {
			return insightMsg{err: err}
		}

		// Cache the insight
		_ = os.WriteFile(cacheFile, []byte(insight), 0644)

		return insightMsg{insight: insight}
	}
}

func getRecentNotesForDays(days int) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	riverDir := filepath.Join(homeDir, "river", "notes")

	var allContent strings.Builder

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")
		filename := filepath.Join(riverDir, dateStr+".md")

		content, err := os.ReadFile(filename)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}

		lines := strings.Split(string(content), "\n")
		var filteredLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "<!--") && strings.HasSuffix(trimmed, "-->") {
				continue
			}
			if strings.TrimSpace(line) != "" {
				filteredLines = append(filteredLines, line)
			}
		}

		if len(filteredLines) > 0 {
			allContent.WriteString(fmt.Sprintf("\n=== %s ===\n", date.Format("Monday, January 2, 2006")))
			allContent.WriteString(strings.Join(filteredLines, "\n"))
			allContent.WriteString("\n")
		}
	}

	return allContent.String(), nil
}

func (m Model) View() string {
	if m.loading {
		return m.renderLoading()
	}

	if m.error != nil {
		return m.renderError()
	}

	return m.renderStats()
}

func (m Model) renderLoading() string {
	content := lipgloss.NewStyle().
		Foreground(colorSubtle).
		Render("Loading your journey...")

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m Model) renderError() string {
	content := lipgloss.NewStyle().
		Foreground(colorWarning).
		Bold(true).
		Render(fmt.Sprintf("✗ %v", m.error))

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

func (m Model) renderStats() string {
	if m.stats == nil || m.stats.totalEntries == 0 {
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(colorSubtle).Render("No entries yet. Start your journey today!"),
		)
	}

	sections := []string{}

	// Hero: Current streak
	sections = append(sections, m.renderStreakHero())
	sections = append(sections, "")

	// Key stats
	sections = append(sections, m.renderKeyStats())
	sections = append(sections, "")
	sections = append(sections, "")

	// Last 7 days
	sections = append(sections, m.renderLast7Days())

	// AI Insight
	sections = append(sections, "")
	sections = append(sections, "")
	sections = append(sections, m.renderInsight())

	// Footer
	footer := lipgloss.NewStyle().
		Foreground(colorSubtle).
		Width(m.width).
		Align(lipgloss.Center).
		Padding(2, 0, 0, 0).
		Render("-/= zoom • r refresh • q quit")

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Left, content, footer),
	)
}

func (m Model) renderStreakHero() string {
	streak := m.stats.currentStreak

	var flames string
	var streakColor lipgloss.AdaptiveColor

	switch {
	case streak == 0:
		flames = "○"
		streakColor = colorSubtle
	case streak < 7:
		flames = "🔥"
		streakColor = colorWarning
	case streak < 30:
		flames = "🔥"
		streakColor = colorSuccess
	default:
		flames = "🔥"
		streakColor = colorSuccess
	}

	streakNum := lipgloss.NewStyle().
		Foreground(streakColor).
		Bold(true).
		Render(fmt.Sprintf("%d", streak))

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		flames,
		streakNum+" day streak",
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(content)
}

func (m Model) renderKeyStats() string {
	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText)

	labelStyle := lipgloss.NewStyle().
		Foreground(colorSubtle)

	stats := [][]string{
		{
			valueStyle.Render(formatNumber(m.stats.totalWords)),
			labelStyle.Render("words"),
		},
		{
			valueStyle.Render(fmt.Sprintf("%d", m.stats.totalEntries)),
			labelStyle.Render("entries"),
		},
		{
			valueStyle.Render(fmt.Sprintf("%d", m.stats.avgWords)),
			labelStyle.Render("avg"),
		},
	}

	var rendered []string
	for _, stat := range stats {
		rendered = append(rendered,
			lipgloss.NewStyle().
				Width(15).
				Align(lipgloss.Center).
				Render(lipgloss.JoinVertical(lipgloss.Center, stat...)))
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(content)
}

func (m Model) renderLast7Days() string {
	days := m.stats.last7Days
	if len(days) == 0 {
		return ""
	}

	var cells []string
	for _, day := range days {
		var symbol string
		var color lipgloss.AdaptiveColor

		switch {
		case day.missing || day.words == 0:
			symbol = "○"
			color = colorSubtle
		case day.words < dailyGoal:
			symbol = "◐"
			color = colorWarning
		default:
			symbol = "●"
			color = colorSuccess
		}

		dayLabel := lipgloss.NewStyle().
			Foreground(colorSubtle).
			Width(5).
			Align(lipgloss.Center).
			Render(day.date.Format("Mon"))

		daySymbol := lipgloss.NewStyle().
			Foreground(color).
			Width(5).
			Align(lipgloss.Center).
			Render(symbol)

		cells = append(cells, lipgloss.JoinVertical(lipgloss.Center, dayLabel, daySymbol))
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, cells...)

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(content)
}

func (m Model) renderInsight() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSubtle).
		Padding(1, 2).
		Width(60).
		Align(lipgloss.Left)

	// Zoom level indicator
	zoomIndicator := lipgloss.NewStyle().
		Foreground(colorSubtle).
		Render(fmt.Sprintf("viewing %s", m.zoomLevel.String()))

	var content string
	if m.loadingInsight {
		content = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Render("generating insight...")
	} else if m.stats != nil && m.stats.insight != "" {
		content = lipgloss.NewStyle().
			Foreground(colorText).
			Render(m.stats.insight)
	} else {
		content = lipgloss.NewStyle().
			Foreground(colorSubtle).
			Render("no insights available")
	}

	boxContent := lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		"",
		zoomIndicator,
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(boxStyle.Render(boxContent))
}

// ═══════════════════════════════════════════════════════════════════════════
// STATS COLLECTION
// ═══════════════════════════════════════════════════════════════════════════

func collectStats() (*stats, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	riverDir := filepath.Join(homeDir, "river", "notes")
	files, err := filepath.Glob(filepath.Join(riverDir, "*.md"))
	if err != nil {
		return nil, err
	}

	var entries []entry

	for _, file := range files {
		base := filepath.Base(file)
		if strings.HasPrefix(base, ".") {
			continue
		}

		dateStr := strings.TrimSuffix(base, ".md")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Count words (excluding HTML comments)
		text := removeHTMLComments(string(content))
		words := len(strings.Fields(text))

		entries = append(entries, entry{
			date:  date,
			words: words,
		})
	}

	// Sort by date
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].date.Before(entries[j].date)
	})

	// Calculate statistics
	s := &stats{
		entries:      entries,
		totalEntries: len(entries),
	}

	if len(entries) == 0 {
		return s, nil
	}

	s.firstDate = entries[0].date
	s.lastDate = entries[len(entries)-1].date

	// Calculate totals and patterns
	totalWords := 0
	entryMap := make(map[string]int)

	for _, e := range entries {
		totalWords += e.words
		entryMap[e.date.Format("2006-01-02")] = e.words
	}

	s.totalWords = totalWords
	if s.totalEntries > 0 {
		s.avgWords = totalWords / s.totalEntries
	}

	// Streaks
	s.currentStreak = calculateCurrentStreak(entryMap)

	// Last 7 days
	s.last7Days = getLast7Days(entryMap, s.firstDate)

	// AI Insight (with caching) - default to 3 days
	s.insight = loadOrGenerateInsight(s, zoom3Days)

	return s, nil
}

func loadOrGenerateInsight(s *stats, zoom zoomLevel) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	riverDir := filepath.Join(homeDir, "river", "notes")
	cacheFile := filepath.Join(riverDir, fmt.Sprintf(".insight-cache-%s", zoom.String()))

	// Check if cache exists and is recent (less than 6 hours old)
	if fileInfo, err := os.Stat(cacheFile); err == nil {
		if time.Since(fileInfo.ModTime()) < 6*time.Hour {
			if cached, err := os.ReadFile(cacheFile); err == nil {
				return string(cached)
			}
		}
	}

	// Generate new insight
	days := zoom.Days()
	if days == 0 {
		days = 365 // Cap at 1 year for "all time"
	}

	recentNotes, err := getRecentNotesForDays(days)
	if err != nil || recentNotes == "" {
		return ""
	}

	insight, err := ai.GetStatsInsight(s.totalWords, s.totalEntries, s.currentStreak, s.avgWords, recentNotes)
	if err != nil {
		return ""
	}

	// Cache the insight
	_ = os.WriteFile(cacheFile, []byte(insight), 0644)

	return insight
}

func getLast7Days(entryMap map[string]int, firstDate time.Time) []dayData {
	var days []dayData
	end := time.Now()
	start := end.AddDate(0, 0, -6)

	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		words := entryMap[dateKey]

		missing := false
		if words == 0 && d.Before(end) && d.After(firstDate.AddDate(0, 0, -1)) {
			missing = true
		}

		days = append(days, dayData{
			date:    d,
			words:   words,
			missing: missing,
		})
	}

	return days
}

func calculateCurrentStreak(entryMap map[string]int) int {
	streak := 0
	date := time.Now()

	// Check if we have today or yesterday
	todayKey := date.Format("2006-01-02")
	yesterdayKey := date.AddDate(0, 0, -1).Format("2006-01-02")

	if _, todayExists := entryMap[todayKey]; !todayExists {
		if _, yesterdayExists := entryMap[yesterdayKey]; !yesterdayExists {
			return 0
		}
		date = date.AddDate(0, 0, -1)
	}

	// Count backwards
	for {
		key := date.Format("2006-01-02")
		if _, exists := entryMap[key]; !exists {
			break
		}
		streak++
		date = date.AddDate(0, 0, -1)
	}

	return streak
}

func removeHTMLComments(text string) string {
	for {
		start := strings.Index(text, "<!--")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], "-->")
		if end == -1 {
			break
		}
		text = text[:start] + text[start+end+3:]
	}
	return text
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 10000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%dk", n/1000)
}

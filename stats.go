package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"
)

// statsModel represents the model for the stats view
type statsModel struct {
	stats       aggregatedStats
	viewport    viewport
	selectedTab int
	tabs        []string
	loading     bool
	error       error
}

// aggregatedStats holds all the statistics data
type aggregatedStats struct {
	totalWords          int
	totalTypingTime     time.Duration
	totalDays           int
	currentStreak       int
	longestStreak       int
	averageWords        int
	mostProductiveDay   string
	mostProductiveWords int
	dailyStats          []dailyStat
	weeklyStats         []weeklyStat
	monthlyTotals       map[string]monthStat
}

type dailyStat struct {
	date       time.Time
	words      int
	typingTime time.Duration
}

type weeklyStat struct {
	weekStart  time.Time
	totalWords int
	totalTime  time.Duration
	daysActive int
}

type monthStat struct {
	totalWords int
	totalTime  time.Duration
	daysActive int
}

func initStatsModel() statsModel {
	return statsModel{
		tabs:        []string{"Overview", "Daily", "Weekly", "Trends"},
		selectedTab: 1, // Start with Daily tab selected
		loading:     true,
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

type statsLoadedMsg struct {
	stats aggregatedStats
}

type statsErrorMsg struct {
	err error
}

func (m statsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewport.width = msg.Width
		m.viewport.height = msg.Height

	case statsLoadedMsg:
		m.stats = msg.stats
		m.loading = false

	case statsErrorMsg:
		m.error = msg.err
		m.loading = false

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "right":
			m.selectedTab = (m.selectedTab + 1) % len(m.tabs)
		case "shift+tab", "left":
			m.selectedTab = (m.selectedTab - 1 + len(m.tabs)) % len(m.tabs)
		}
	}

	return m, nil
}

func (m statsModel) View() string {
	if m.loading {
		return lipgloss.NewStyle().
			Width(m.viewport.width).
			Height(m.viewport.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render("Loading stats...")
	}

	if m.error != nil {
		return lipgloss.NewStyle().
			Width(m.viewport.width).
			Height(m.viewport.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(lipgloss.Color("#FF0000")).
			Render(fmt.Sprintf("Error loading stats: %v", m.error))
	}

	// Header with tabs
	header := m.renderTabs()

	// Content based on selected tab
	var content string
	switch m.selectedTab {
	case 0:
		content = m.renderOverview()
	case 1:
		content = m.renderDaily()
	case 2:
		content = m.renderWeekly()
	case 3:
		content = m.renderTrends()
	}

	// Footer with help
	footer := m.renderFooter()

	// Combine all parts
	return lipgloss.JoinVertical(
		lipgloss.Top,
		header,
		content,
		footer,
	)
}

func (m statsModel) renderTabs() string {
	var tabs []string

	tabStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Margin(0, 1)

	activeTabStyle := tabStyle.Copy().
		Foreground(lipgloss.Color("#FF1493")).
		Bold(true).
		Underline(true)

	for i, tab := range m.tabs {
		if i == m.selectedTab {
			tabs = append(tabs, activeTabStyle.Render(tab))
		} else {
			tabs = append(tabs, tabStyle.Render(tab))
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	return lipgloss.NewStyle().
		Width(m.viewport.width).
		Padding(1, 0).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#444")).
		Render(tabBar)
}

func (m statsModel) renderOverview() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF1493")).
		MarginBottom(1)

	statStyle := lipgloss.NewStyle().
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888"))

	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFF"))

	// Calculate additional stats
	avgWordsPerDay := 0
	if m.stats.totalDays > 0 {
		avgWordsPerDay = m.stats.totalWords / m.stats.totalDays
	}

	avgTimePerDay := time.Duration(0)
	if m.stats.totalDays > 0 {
		avgTimePerDay = m.stats.totalTypingTime / time.Duration(m.stats.totalDays)
	}

	// Build overview content
	var content []string

	content = append(content, titleStyle.Render("📊 River Statistics Overview"))
	content = append(content, "")

	// Main stats grid
	statsGrid := [][]string{
		{
			statStyle.Render(labelStyle.Render("Total Words Written: ") + valueStyle.Render(fmt.Sprintf("%d", m.stats.totalWords))),
			statStyle.Render(labelStyle.Render("Total Time: ") + valueStyle.Render(formatDuration(m.stats.totalTypingTime))),
		},
		{
			statStyle.Render(labelStyle.Render("Days Active: ") + valueStyle.Render(fmt.Sprintf("%d", m.stats.totalDays))),
			statStyle.Render(labelStyle.Render("Average Words/Day: ") + valueStyle.Render(fmt.Sprintf("%d", avgWordsPerDay))),
		},
		{
			statStyle.Render(labelStyle.Render("Current Streak: ") + valueStyle.Render(fmt.Sprintf("%d days", m.stats.currentStreak))),
			statStyle.Render(labelStyle.Render("Longest Streak: ") + valueStyle.Render(fmt.Sprintf("%d days", m.stats.longestStreak))),
		},
		{
			statStyle.Render(labelStyle.Render("Average Time/Day: ") + valueStyle.Render(formatDuration(avgTimePerDay))),
			statStyle.Render(labelStyle.Render("Most Productive Day: ") + valueStyle.Render(fmt.Sprintf("%s (%d words)", m.stats.mostProductiveDay, m.stats.mostProductiveWords))),
		},
	}

	for _, row := range statsGrid {
		content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}

	// Add a visual progress bar for today's goal
	if len(m.stats.dailyStats) > 0 {
		today := m.stats.dailyStats[len(m.stats.dailyStats)-1]
		if today.date.Format("2006-01-02") == time.Now().Format("2006-01-02") {
			content = append(content, "")
			content = append(content, titleStyle.Render("Today's Progress"))
			content = append(content, m.renderProgressBar(today.words, 500, 40))
		}
	}

	return lipgloss.NewStyle().
		Padding(2).
		Render(strings.Join(content, "\n"))
}

func (m statsModel) renderDaily() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF1493")).
		MarginBottom(1)

	var content []string
	content = append(content, titleStyle.Render("📅 Daily Statistics"))
	content = append(content, "")

	// Show last 14 days including missing days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -13) // 14 days total including today

	// Create a map of existing stats for quick lookup
	statsMap := make(map[string]dailyStat)
	for _, stat := range m.stats.dailyStats {
		statsMap[stat.date.Format("2006-01-02")] = stat
	}

	// Iterate through each day in the range
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		dateStr := d.Format("Mon, Jan 2")

		if dateKey == time.Now().Format("2006-01-02") {
			dateStr += " (Today)"
		}

		// Check if we have data for this day
		if stat, exists := statsMap[dateKey]; exists {
			// Day with data
			bar := m.renderMiniBar(stat.words, 1000, 30)
			timeStr := formatDuration(stat.typingTime)
			line := fmt.Sprintf("%-20s %s %5d words %8s", dateStr, bar, stat.words, timeStr)

			if dateKey == time.Now().Format("2006-01-02") {
				line = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Render(line)
			}

			content = append(content, line)
		} else {
			// Day without data - show as empty/missed
			emptyBar := m.renderEmptyBar(30)
			line := fmt.Sprintf("%-20s %s %5s %8s", dateStr, emptyBar, "-", "-")

			// Style missed days differently (dimmed)
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(line)

			content = append(content, line)
		}
	}

	return lipgloss.NewStyle().
		Padding(2).
		Render(strings.Join(content, "\n"))
}

func (m statsModel) renderWeekly() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF1493")).
		MarginBottom(1)

	var content []string
	content = append(content, titleStyle.Render("📈 Weekly Statistics"))
	content = append(content, "")

	// Show last 8 weeks
	startIdx := len(m.stats.weeklyStats) - 8
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(m.stats.weeklyStats); i++ {
		stat := m.stats.weeklyStats[i]

		weekStr := fmt.Sprintf("Week of %s", stat.weekStart.Format("Jan 2"))
		avgWords := 0
		if stat.daysActive > 0 {
			avgWords = stat.totalWords / stat.daysActive
		}

		bar := m.renderMiniBar(stat.totalWords, 5000, 25)

		line := fmt.Sprintf("%-20s %s %5d words (%d days, avg %d/day)",
			weekStr, bar, stat.totalWords, stat.daysActive, avgWords)

		content = append(content, line)
	}

	return lipgloss.NewStyle().
		Padding(2).
		Render(strings.Join(content, "\n"))
}

func (m statsModel) renderTrends() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF1493")).
		MarginBottom(1)

	var content []string
	content = append(content, titleStyle.Render("📈 Writing Trends"))
	content = append(content, "")

	// Monthly overview
	content = append(content, lipgloss.NewStyle().Bold(true).Render("Monthly Totals:"))
	content = append(content, "")

	// Sort months
	var months []string
	for month := range m.stats.monthlyTotals {
		months = append(months, month)
	}
	sort.Strings(months)

	// Show last 6 months
	startIdx := len(months) - 6
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(months); i++ {
		month := months[i]
		stat := m.stats.monthlyTotals[month]

		monthTime, _ := time.Parse("2006-01", month)
		monthStr := monthTime.Format("January 2006")

		bar := m.renderMiniBar(stat.totalWords, 20000, 25)

		line := fmt.Sprintf("%-20s %s %6d words (%d days active)",
			monthStr, bar, stat.totalWords, stat.daysActive)

		content = append(content, line)
	}

	// Add insights
	content = append(content, "")
	content = append(content, lipgloss.NewStyle().Bold(true).Render("Insights:"))
	content = append(content, "")

	// Best writing time (simplified for now)
	content = append(content, fmt.Sprintf("• Most productive day: %s", m.stats.mostProductiveDay))
	content = append(content, fmt.Sprintf("• Average session: %s", formatDuration(m.stats.totalTypingTime/time.Duration(m.stats.totalDays))))
	content = append(content, fmt.Sprintf("• Total writing time: %s", formatDuration(m.stats.totalTypingTime)))

	return lipgloss.NewStyle().
		Padding(2).
		Render(strings.Join(content, "\n"))
}

func (m statsModel) renderFooter() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666")).
		Padding(1, 2)

	help := "Tab/→: Next tab • Shift+Tab/←: Previous tab • q/Esc: Quit"

	return lipgloss.NewStyle().
		Width(m.viewport.width).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#444")).
		Render(helpStyle.Render(help))
}

func (m statsModel) renderProgressBar(current, target, width int) string {
	progress := float64(current) / float64(target)
	if progress > 1.0 {
		progress = 1.0
	}

	filled := int(progress * float64(width))

	var bar strings.Builder
	bar.WriteString("[")

	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Render("█"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#444")).Render("░"))
		}
	}

	bar.WriteString("]")

	percentage := fmt.Sprintf(" %d%% (%d/%d words)", int(progress*100), current, target)

	return bar.String() + percentage
}

func (m statsModel) renderMiniBar(current, max, width int) string {
	progress := float64(current) / float64(max)
	if progress > 1.0 {
		progress = 1.0
	}

	filled := int(progress * float64(width))

	var bar strings.Builder

	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#AA6688")).Render("▓"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#444")).Render("░"))
		}
	}

	return bar.String()
}

func (m statsModel) renderEmptyBar(width int) string {
	var bar strings.Builder

	for i := 0; i < width; i++ {
		// Use a different character to indicate no data
		bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#333")).Render("─"))
	}

	return bar.String()
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func collectAllStats() (aggregatedStats, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return aggregatedStats{}, err
	}

	riverDir := filepath.Join(homeDir, "river", "notes")

	// Get all markdown files
	files, err := filepath.Glob(filepath.Join(riverDir, "*.md"))
	if err != nil {
		return aggregatedStats{}, err
	}

	var allStats aggregatedStats
	allStats.monthlyTotals = make(map[string]monthStat)

	// Collect daily stats
	for _, file := range files {
		base := filepath.Base(file)
		dateStr := strings.TrimSuffix(base, ".md")

		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue // Skip files that don't match date format
		}

		// Read the file to count words
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		words := countWords(string(content))

		// Read stats file
		statsFile := filepath.Join(riverDir, ".stats-"+dateStr+".toml")
		typingTime := time.Duration(0)

		if data, err := os.ReadFile(statsFile); err == nil {
			var s stats
			if err := toml.Unmarshal(data, &s); err == nil {
				typingTime = time.Duration(s.TypingSeconds) * time.Second
			}
		}

		dailyStat := dailyStat{
			date:       date,
			words:      words,
			typingTime: typingTime,
		}

		allStats.dailyStats = append(allStats.dailyStats, dailyStat)
		allStats.totalWords += words
		allStats.totalTypingTime += typingTime

		// Update most productive day
		if words > allStats.mostProductiveWords {
			allStats.mostProductiveWords = words
			allStats.mostProductiveDay = date.Format("Jan 2, 2006")
		}

		// Update monthly stats
		monthKey := date.Format("2006-01")
		month := allStats.monthlyTotals[monthKey]
		month.totalWords += words
		month.totalTime += typingTime
		month.daysActive++
		allStats.monthlyTotals[monthKey] = month
	}

	// Sort daily stats by date
	sort.Slice(allStats.dailyStats, func(i, j int) bool {
		return allStats.dailyStats[i].date.Before(allStats.dailyStats[j].date)
	})

	allStats.totalDays = len(allStats.dailyStats)

	// Calculate streaks
	allStats.currentStreak = calculateCurrentStreak(allStats.dailyStats)
	allStats.longestStreak = calculateLongestStreak(allStats.dailyStats)

	// Calculate weekly stats
	allStats.weeklyStats = calculateWeeklyStats(allStats.dailyStats)

	return allStats, nil
}

func countWords(content string) int {
	words := strings.Fields(content)
	return len(words)
}

func calculateCurrentStreak(dailyStats []dailyStat) int {
	if len(dailyStats) == 0 {
		return 0
	}

	// Create a map for quick date lookup
	dateMap := make(map[string]bool)
	for _, stat := range dailyStats {
		dateMap[stat.date.Format("2006-01-02")] = true
	}

	streak := 0
	today := time.Now()

	// Check if today has an entry
	todayKey := today.Format("2006-01-02")
	if !dateMap[todayKey] {
		// If no entry today, streak is already broken
		return 0
	}

	// Count backwards from today
	for d := today; ; d = d.AddDate(0, 0, -1) {
		dateKey := d.Format("2006-01-02")
		if dateMap[dateKey] {
			streak++
		} else {
			// Gap found, streak ends
			break
		}
	}

	return streak
}

func calculateLongestStreak(dailyStats []dailyStat) int {
	if len(dailyStats) == 0 {
		return 0
	}

	longestStreak := 1
	currentStreak := 1

	for i := 1; i < len(dailyStats); i++ {
		prevDate := dailyStats[i-1].date
		currDate := dailyStats[i].date

		// Check if consecutive days
		if currDate.Sub(prevDate).Hours() == 24 {
			currentStreak++
			if currentStreak > longestStreak {
				longestStreak = currentStreak
			}
		} else {
			currentStreak = 1
		}
	}

	return longestStreak
}

func calculateWeeklyStats(dailyStats []dailyStat) []weeklyStat {
	if len(dailyStats) == 0 {
		return nil
	}

	var weeklyStats []weeklyStat

	// Group by week
	weekMap := make(map[string]*weeklyStat)

	for _, daily := range dailyStats {
		// Get start of week (Sunday)
		weekStart := daily.date
		for weekStart.Weekday() != time.Sunday {
			weekStart = weekStart.AddDate(0, 0, -1)
		}

		weekKey := weekStart.Format("2006-01-02")

		if week, exists := weekMap[weekKey]; exists {
			week.totalWords += daily.words
			week.totalTime += daily.typingTime
			week.daysActive++
		} else {
			weekMap[weekKey] = &weeklyStat{
				weekStart:  weekStart,
				totalWords: daily.words,
				totalTime:  daily.typingTime,
				daysActive: 1,
			}
		}
	}

	// Convert map to slice and sort
	for _, week := range weekMap {
		weeklyStats = append(weeklyStats, *week)
	}

	sort.Slice(weeklyStats, func(i, j int) bool {
		return weeklyStats[i].weekStart.Before(weeklyStats[j].weekStart)
	})

	return weeklyStats
}

package statsui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	dailyGoal = 500
)

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	warning   = lipgloss.AdaptiveColor{Light: "#FF5F87", Dark: "#FF6F91"}
	
	tabBorderColor = lipgloss.Color("240")
	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}
	tabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}
)

type tab int

const (
	tabOverview tab = iota
	tabDaily
	tabWeekly
	tabTrends
)

var tabNames = []string{"Overview", "Daily", "Weekly", "Trends"}

type keyMap struct {
	Tab    key.Binding
	Left   key.Binding
	Right  key.Binding
	Up     key.Binding
	Down   key.Binding
	Quit   key.Binding
}

var keys = keyMap{
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next tab"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev tab"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next tab"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "scroll up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "scroll down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c", "esc"),
		key.WithHelp("q", "quit"),
	),
}

type Model struct {
	width      int
	height     int
	activeTab  tab
	loading    bool
	error      error
	stats      *stats
	spinner    spinner.Model
	progress   progress.Model
	scrollY    int
	maxScrollY int
}

type stats struct {
	notes         []noteData
	totalWords    int
	totalDays     int
	currentStreak int
	longestStreak int
	avgWords      float64
	todayWords    int
	weeklyData    []weekData
	monthlyData   []monthData
}

type noteData struct {
	date  time.Time
	words int
}

type weekData struct {
	startDate time.Time
	words     int
	days      int
	avg       float64
}

type monthData struct {
	month time.Month
	year  int
	words int
	days  int
	avg   float64
}

func InitModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(highlight)
	
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)
	
	return Model{
		loading:   true,
		spinner:   s,
		progress:  p,
		activeTab: tabOverview,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadStats,
	)
}

type statsMsg struct {
	stats *stats
	err   error
}

func loadStats() tea.Msg {
	stats, err := collectStats()
	return statsMsg{stats: stats, err: err}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = min(msg.Width-20, 60)
		
	case statsMsg:
		m.loading = false
		if msg.err != nil {
			m.error = msg.err
		} else {
			m.stats = msg.stats
		}
		
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Right):
			m.activeTab = (m.activeTab + 1) % 4
			m.scrollY = 0
		case key.Matches(msg, keys.Left):
			m.activeTab = (m.activeTab + 3) % 4
			m.scrollY = 0
		case key.Matches(msg, keys.Down):
			if m.scrollY < m.maxScrollY {
				m.scrollY++
			}
		case key.Matches(msg, keys.Up):
			if m.scrollY > 0 {
				m.scrollY--
			}
		}
		
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}
	
	return m, tea.Batch(cmds...)
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
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		m.spinner.View(),
		"",
		lipgloss.NewStyle().
			Foreground(subtle).
			Render("Analyzing your writing journey..."),
	)
	
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(2, 4).
			Render(content),
	)
}

func (m Model) renderError() string {
	content := lipgloss.NewStyle().
		Foreground(warning).
		Bold(true).
		Render(fmt.Sprintf("✗ %v", m.error))
	
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warning).
			Padding(1, 3).
			Render(content),
	)
}

func (m Model) renderStats() string {
	header := m.renderHeader()
	tabs := m.renderTabs()
	content := m.renderContent()
	footer := m.renderFooter()
	
	availHeight := m.height - lipgloss.Height(header) - lipgloss.Height(tabs) - lipgloss.Height(footer) - 2
	
	contentBox := lipgloss.NewStyle().
		Height(availHeight).
		Width(m.width).
		Padding(0, 2).
		Render(content)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabs,
		contentBox,
		footer,
	)
}

func (m Model) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render("River Statistics")
	
	subtitle := lipgloss.NewStyle().
		Foreground(subtle).
		Render(fmt.Sprintf("%d words • %d days • %d day streak",
			m.stats.totalWords, m.stats.totalDays, m.stats.currentStreak))
	
	header := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		subtitle,
	)
	
	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Padding(1, 0).
		Render(header)
}

func (m Model) renderTabs() string {
	var tabs []string
	
	for i, name := range tabNames {
		if tab(i) == m.activeTab {
			tabs = append(tabs, activeTab(name))
		} else {
			tabs = append(tabs, inactiveTab(name))
		}
	}
	
	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		tabs...,
	)
	
	gap := lipgloss.NewStyle().
		Width(m.width - lipgloss.Width(row)).
		Render("")
	
	return lipgloss.NewStyle().
		Width(m.width).
		Render(lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap))
}

func activeTab(name string) string {
	return lipgloss.NewStyle().
		Border(activeTabBorder).
		BorderForeground(highlight).
		Foreground(highlight).
		Padding(0, 2).
		Render(name)
}

func inactiveTab(name string) string {
	return lipgloss.NewStyle().
		Border(tabBorder).
		BorderForeground(tabBorderColor).
		Foreground(subtle).
		Padding(0, 2).
		Render(name)
}

func (m Model) renderContent() string {
	switch m.activeTab {
	case tabOverview:
		return m.renderOverview()
	case tabDaily:
		return m.renderDaily()
	case tabWeekly:
		return m.renderWeekly()
	case tabTrends:
		return m.renderTrends()
	default:
		return ""
	}
}

func (m Model) renderOverview() string {
	sections := []string{}
	
	// Today's progress
	todayProgress := m.renderTodayProgress()
	sections = append(sections, todayProgress)
	
	// Quick stats cards
	cards := m.renderStatsCards()
	sections = append(sections, "", cards)
	
	// Recent activity
	recent := m.renderRecentActivity()
	sections = append(sections, "", recent)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderTodayProgress() string {
	progress := float64(m.stats.todayWords) / float64(dailyGoal)
	percentage := int(progress * 100)
	if percentage > 100 {
		percentage = 100
	}
	
	progressBar := m.progress.ViewAs(progress)
	
	label := lipgloss.NewStyle().
		Foreground(subtle).
		Render("Today's Progress")
	
	stats := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		Render(fmt.Sprintf("%d / %d words", m.stats.todayWords, dailyGoal))
	
	emoji := "📝"
	if progress >= 1.0 {
		emoji = "🎉"
	} else if progress >= 0.5 {
		emoji = "💪"
	}
	
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		emoji,
		" ",
		label,
		" • ",
		stats,
		fmt.Sprintf(" (%d%%)", percentage),
	)
	
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(subtle).
		Padding(1, 2).
		Width(min(m.width-4, 80)).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			progressBar,
		))
}

func (m Model) renderStatsCards() string {
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(subtle).
		Padding(1).
		Width(20).
		Align(lipgloss.Center)
	
	labelStyle := lipgloss.NewStyle().
		Foreground(subtle)
	
	valueStyle := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true)
	
	cards := []string{}
	
	// Streak card
	streakCard := cardStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			valueStyle.Render(fmt.Sprintf("%d", m.stats.currentStreak)),
			labelStyle.Render("day streak"),
		),
	)
	cards = append(cards, streakCard)
	
	// Average words card
	avgCard := cardStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			valueStyle.Render(fmt.Sprintf("%.0f", m.stats.avgWords)),
			labelStyle.Render("avg words/day"),
		),
	)
	cards = append(cards, avgCard)
	
	// Total days card
	daysCard := cardStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			valueStyle.Render(fmt.Sprintf("%d", m.stats.totalDays)),
			labelStyle.Render("days written"),
		),
	)
	cards = append(cards, daysCard)
	
	// Total words card  
	wordsCard := cardStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			valueStyle.Render(formatNumber(m.stats.totalWords)),
			labelStyle.Render("total words"),
		),
	)
	cards = append(cards, wordsCard)
	
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		cards...,
	)
}

func (m Model) renderRecentActivity() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		MarginBottom(1)
	
	title := titleStyle.Render("Recent Activity")
	
	// Get last 7 days
	days := []string{}
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		words := m.getWordsForDate(date)
		
		dayStyle := lipgloss.NewStyle().Width(10)
		if words > 0 {
			dayStyle = dayStyle.Foreground(special)
		} else {
			dayStyle = dayStyle.Foreground(subtle)
		}
		
		bar := m.renderSparkBar(words, dailyGoal, 8)
		dayLine := fmt.Sprintf("%-3s %s %4d", 
			date.Format("Mon"),
			bar,
			words,
		)
		days = append(days, dayStyle.Render(dayLine))
	}
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(days, "\n"),
	)
	
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(subtle).
		Padding(1, 2).
		Width(min(m.width-4, 40)).
		Render(content)
}

func (m Model) renderDaily() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		MarginBottom(1)
	
	title := titleStyle.Render("Daily Writing Log")
	
	// Create a scrollable list of all days
	days := []string{}
	for _, note := range m.stats.notes {
		dateStr := note.date.Format("Mon, Jan 2")
		bar := m.renderSparkBar(note.words, dailyGoal, 20)
		
		lineStyle := lipgloss.NewStyle()
		if note.words >= dailyGoal {
			lineStyle = lineStyle.Foreground(special)
		} else if note.words > 0 {
			lineStyle = lineStyle.Foreground(lipgloss.Color("252"))
		} else {
			lineStyle = lineStyle.Foreground(subtle)
		}
		
		line := fmt.Sprintf("%-15s %s %5d words", dateStr, bar, note.words)
		days = append(days, lineStyle.Render(line))
	}
	
	// Apply scrolling
	visibleHeight := m.height - 10
	startIdx := m.scrollY
	endIdx := min(startIdx+visibleHeight, len(days))
	
	if startIdx < len(days) {
		days = days[startIdx:endIdx]
	}
	
	m.maxScrollY = max(0, len(m.stats.notes)-visibleHeight)
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(days, "\n"),
	)
	
	return content
}

func (m Model) renderWeekly() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		MarginBottom(1)
	
	title := titleStyle.Render("Weekly Summary")
	
	weeks := []string{}
	for _, week := range m.stats.weeklyData {
		weekStr := week.startDate.Format("Week of Jan 2")
		
		bar := m.renderSparkBar(int(week.avg), dailyGoal, 15)
		
		lineStyle := lipgloss.NewStyle()
		if week.avg >= float64(dailyGoal) {
			lineStyle = lineStyle.Foreground(special)
		} else if week.words > 0 {
			lineStyle = lineStyle.Foreground(lipgloss.Color("252"))
		} else {
			lineStyle = lineStyle.Foreground(subtle)
		}
		
		line := fmt.Sprintf("%-18s %s %6d words • %d days • %.0f avg",
			weekStr, bar, week.words, week.days, week.avg)
		weeks = append(weeks, lineStyle.Render(line))
	}
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(weeks, "\n"),
	)
	
	return content
}

func (m Model) renderTrends() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		MarginBottom(1)
	
	sections := []string{}
	
	// Monthly trends
	monthTitle := titleStyle.Render("Monthly Trends")
	months := []string{}
	
	for _, month := range m.stats.monthlyData {
		monthStr := fmt.Sprintf("%s %d", month.month.String()[:3], month.year)
		
		bar := m.renderSparkBar(month.words, month.days*dailyGoal, 20)
		
		lineStyle := lipgloss.NewStyle()
		if month.avg >= float64(dailyGoal) {
			lineStyle = lineStyle.Foreground(special)
		} else {
			lineStyle = lineStyle.Foreground(lipgloss.Color("252"))
		}
		
		line := fmt.Sprintf("%-10s %s %6d words • %.0f/day",
			monthStr, bar, month.words, month.avg)
		months = append(months, lineStyle.Render(line))
	}
	
	sections = append(sections,
		lipgloss.JoinVertical(lipgloss.Left, monthTitle, strings.Join(months, "\n")))
	
	// Writing patterns
	patternTitle := titleStyle.Render("Writing Patterns")
	patterns := m.analyzePatterns()
	
	patternStyle := lipgloss.NewStyle().
		Foreground(subtle).
		MarginLeft(2)
	
	sections = append(sections, "",
		lipgloss.JoinVertical(lipgloss.Left,
			patternTitle,
			patternStyle.Render(patterns),
		))
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m Model) renderSparkBar(value, max, width int) string {
	if max == 0 {
		max = 1
	}
	
	filled := int(float64(value) / float64(max) * float64(width))
	if filled > width {
		filled = width
	}
	
	bar := strings.Builder{}
	for i := 0; i < width; i++ {
		if i < filled {
			if value >= max {
				bar.WriteString(lipgloss.NewStyle().Foreground(special).Render("█"))
			} else {
				intensity := 236 + (i * 2)
				if intensity > 250 {
					intensity = 250
				}
				bar.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color(fmt.Sprintf("%d", intensity))).
					Render("█"))
			}
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("235")).Render("░"))
		}
	}
	
	return bar.String()
}

func (m Model) renderFooter() string {
	help := []string{}
	
	if m.activeTab == tabDaily || m.activeTab == tabTrends {
		help = append(help, "↑↓/jk: scroll")
	}
	
	help = append(help,
		"←→/hl: switch tabs",
		"tab: next",
		"q: quit",
	)
	
	return lipgloss.NewStyle().
		Foreground(subtle).
		Width(m.width).
		Align(lipgloss.Center).
		Padding(1, 0).
		Render(strings.Join(help, " • "))
}

func (m Model) getWordsForDate(date time.Time) int {
	dateStr := date.Format("2006-01-02")
	for _, note := range m.stats.notes {
		if note.date.Format("2006-01-02") == dateStr {
			return note.words
		}
	}
	return 0
}

func (m Model) analyzePatterns() string {
	if len(m.stats.notes) == 0 {
		return "No data yet"
	}
	
	// Find best day of week
	dayCount := make(map[time.Weekday]int)
	dayWords := make(map[time.Weekday]int)
	
	for _, note := range m.stats.notes {
		day := note.date.Weekday()
		dayCount[day]++
		dayWords[day] += note.words
	}
	
	var bestDay time.Weekday
	bestAvg := 0.0
	for day := time.Sunday; day <= time.Saturday; day++ {
		if count := dayCount[day]; count > 0 {
			avg := float64(dayWords[day]) / float64(count)
			if avg > bestAvg {
				bestAvg = avg
				bestDay = day
			}
		}
	}
	
	// Find best time (simplified - just morning/afternoon/evening based on file creation)
	consistency := float64(m.stats.currentStreak) / float64(m.stats.totalDays) * 100
	
	patterns := []string{
		fmt.Sprintf("📅 Best day: %s (%.0f words avg)", bestDay, bestAvg),
		fmt.Sprintf("🔥 Longest streak: %d days", m.stats.longestStreak),
		fmt.Sprintf("📊 Consistency: %.1f%%", consistency),
	}
	
	return strings.Join(patterns, "\n")
}

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
	
	notes := []noteData{}
	dateMap := make(map[string]int)
	
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
		
		// Count words (excluding HTML comments for ghost text)
		text := string(content)
		text = removeHTMLComments(text)
		words := len(strings.Fields(text))
		
		notes = append(notes, noteData{
			date:  date,
			words: words,
		})
		
		dateMap[dateStr] = words
	}
	
	// Sort notes by date
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].date.Before(notes[j].date)
	})
	
	// Calculate statistics
	stats := &stats{
		notes:      notes,
		totalDays:  len(notes),
	}
	
	if len(notes) > 0 {
		// Calculate totals
		for _, note := range notes {
			stats.totalWords += note.words
		}
		
		// Today's words
		today := time.Now().Format("2006-01-02")
		if words, exists := dateMap[today]; exists {
			stats.todayWords = words
		}
		
		// Average
		stats.avgWords = float64(stats.totalWords) / float64(stats.totalDays)
		
		// Streaks
		stats.currentStreak = calculateCurrentStreak(dateMap)
		stats.longestStreak = calculateLongestStreak(notes)
		
		// Weekly data
		stats.weeklyData = calculateWeeklyData(notes)
		
		// Monthly data
		stats.monthlyData = calculateMonthlyData(notes)
	}
	
	return stats, nil
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

func calculateCurrentStreak(dateMap map[string]int) int {
	streak := 0
	date := time.Now()
	
	// Check if we have today or yesterday
	todayKey := date.Format("2006-01-02")
	yesterdayKey := date.AddDate(0, 0, -1).Format("2006-01-02")
	
	if _, todayExists := dateMap[todayKey]; !todayExists {
		if _, yesterdayExists := dateMap[yesterdayKey]; !yesterdayExists {
			return 0
		}
		date = date.AddDate(0, 0, -1)
	}
	
	// Count backwards
	for {
		key := date.Format("2006-01-02")
		if _, exists := dateMap[key]; !exists {
			break
		}
		streak++
		date = date.AddDate(0, 0, -1)
	}
	
	return streak
}

func calculateLongestStreak(notes []noteData) int {
	if len(notes) == 0 {
		return 0
	}
	
	longest := 1
	current := 1
	
	for i := 1; i < len(notes); i++ {
		diff := notes[i].date.Sub(notes[i-1].date).Hours() / 24
		if diff == 1 {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 1
		}
	}
	
	return longest
}

func calculateWeeklyData(notes []noteData) []weekData {
	weekMap := make(map[string]*weekData)
	
	for _, note := range notes {
		// Find week start (Sunday)
		weekStart := note.date
		for weekStart.Weekday() != time.Sunday {
			weekStart = weekStart.AddDate(0, 0, -1)
		}
		
		key := weekStart.Format("2006-01-02")
		
		if w, exists := weekMap[key]; exists {
			w.words += note.words
			w.days++
		} else {
			weekMap[key] = &weekData{
				startDate: weekStart,
				words:     note.words,
				days:      1,
			}
		}
	}
	
	// Convert to slice and calculate averages
	weeks := []weekData{}
	for _, w := range weekMap {
		w.avg = float64(w.words) / float64(w.days)
		weeks = append(weeks, *w)
	}
	
	// Sort by date
	sort.Slice(weeks, func(i, j int) bool {
		return weeks[i].startDate.Before(weeks[j].startDate)
	})
	
	// Keep last 8 weeks
	if len(weeks) > 8 {
		weeks = weeks[len(weeks)-8:]
	}
	
	return weeks
}

func calculateMonthlyData(notes []noteData) []monthData {
	monthMap := make(map[string]*monthData)
	
	for _, note := range notes {
		key := note.date.Format("2006-01")
		
		if m, exists := monthMap[key]; exists {
			m.words += note.words
			m.days++
		} else {
			monthMap[key] = &monthData{
				month: note.date.Month(),
				year:  note.date.Year(),
				words: note.words,
				days:  1,
			}
		}
	}
	
	// Convert to slice and calculate averages
	months := []monthData{}
	for _, m := range monthMap {
		m.avg = float64(m.words) / float64(m.days)
		months = append(months, *m)
	}
	
	// Sort by date
	sort.Slice(months, func(i, j int) bool {
		if months[i].year != months[j].year {
			return months[i].year < months[j].year
		}
		return months[i].month < months[j].month
	})
	
	// Keep last 6 months
	if len(months) > 6 {
		months = months[len(months)-6:]
	}
	
	return months
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
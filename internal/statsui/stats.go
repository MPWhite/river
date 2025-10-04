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

	tabBorderColor  = lipgloss.Color("240")
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
	tabPrompts
)

var tabNames = []string{"Overview", "Daily", "Weekly", "Prompts"}

type keyMap struct {
	Tab   key.Binding
	Left  key.Binding
	Right key.Binding
	Up    key.Binding
	Down  key.Binding
	Quit  key.Binding
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
	// Extended dashboard fields
	startDate       time.Time
	endDate         time.Time
	wordsSeries     []int
	sentimentSeries []float64
	anxietySeries   []float64
	morningCount    int
	afternoonCount  int
	eveningCount    int
	lateNightCount  int
	avgReadability  float64
	emotionTotals   map[string]int
	topTopics       []topicScore
}

type noteData struct {
	date        time.Time
	words       int
	hour        int
	text        string
	sentiment   float64
	anxiety     float64
	readability float64
	emotions    map[string]int
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

type topicScore struct {
	term  string
	count int
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
			m.activeTab = tab((int(m.activeTab) + 1) % len(tabNames))
			m.scrollY = 0
		case key.Matches(msg, keys.Left):
			m.activeTab = tab((int(m.activeTab) + len(tabNames) - 1) % len(tabNames))
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
	streak := ""
	if m.stats.currentStreak > 0 {
		streak = fmt.Sprintf(" • %d day streak 🔥", m.stats.currentStreak)
	}

	header := lipgloss.NewStyle().
		Foreground(subtle).
		Render(fmt.Sprintf("%s words • %d days%s",
			formatNumber(m.stats.totalWords), m.stats.totalDays, streak))

	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Padding(1, 0, 0, 0).
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
	case tabPrompts:
		return m.renderPrompts()
	default:
		return ""
	}
}

func (m Model) renderOverview() string {
	if m.stats == nil {
		return ""
	}

	header := m.renderDashboardHeader()

	heatmap := m.renderEntriesHeatmap(16)
	wordSpark := m.renderWordCountSpark()

	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width/2-2).Render(heatmap),
		lipgloss.NewStyle().Width(m.width/2-2).Render(wordSpark),
	)

	sent := m.renderMiniTimeseries("Sentiment", m.stats.sentimentSeries, 32, "Δ")
	anx := m.renderMiniTimeseries("Anxiety", m.stats.anxietySeries, 32, "avg")
	read := m.renderReadabilityCard()

	middleRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width/3-2).Render(sent),
		lipgloss.NewStyle().Width(m.width/3-2).Render(anx),
		lipgloss.NewStyle().Width(m.width/3-2).Render(read),
	)

	emotions := m.renderEmotionBars()
	writingTime := m.renderWritingTimeBars()
	topics := m.renderTopicBars()

	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width/3-2).Render(emotions),
		lipgloss.NewStyle().Width(m.width/3-2).Render(writingTime),
		lipgloss.NewStyle().Width(m.width/3-2).Render(topics),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		topRow,
		"",
		middleRow,
		"",
		bottomRow,
	)
}

func (m Model) renderTodayProgress() string {
	progress := float64(m.stats.todayWords) / float64(dailyGoal)

	progressBar := m.progress.ViewAs(progress)

	status := ""
	if progress >= 1.0 {
		status = " ✓"
	}

	header := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		Render(fmt.Sprintf("Today: %d / %d words%s", m.stats.todayWords, dailyGoal, status))

	return lipgloss.NewStyle().
		Padding(0, 1).
		Width(min(m.width-4, 60)).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			progressBar,
		))
}

// --- New dashboard rendering helpers ---

func (m Model) renderDashboardHeader() string {
	if m.stats == nil {
		return ""
	}
	start := m.stats.startDate
	if start.IsZero() && len(m.stats.notes) > 0 {
		start = m.stats.notes[0].date
	}
	end := m.stats.endDate
	if end.IsZero() {
		end = time.Now()
	}
	rangeText := fmt.Sprintf("Range: %s to %s  Entries: %d",
		start.Format("2006-01-02"), end.Format("2006-01-02"), len(m.stats.notes))
	return lipgloss.NewStyle().Foreground(subtle).Render(rangeText)
}

func (m Model) renderEntriesHeatmap(weeks int) string {
	if weeks <= 0 {
		weeks = 16
	}
	cols := weeks
	rows := 4
	// build grid last cols*rows weeks (approx days)
	totalDays := cols * 7
	start := time.Now().AddDate(0, 0, -totalDays+1)
	// map
	noteMap := map[string]int{}
	for _, n := range m.stats.notes {
		noteMap[n.date.Format("2006-01-02")] = n.words
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("Entries by Day") + "\n")
	// Render rows by week rows stacked like GitHub calendar simplified
	for r := 0; r < rows; r++ {
		// Month label progresses by months to resemble the mock
		month := start.AddDate(0, r, 0).Format("Jan")
		b.WriteString(fmt.Sprintf("%s ", month))
		for c := 0; c < cols; c++ {
			d := start.AddDate(0, 0, c*7+r)
			key := d.Format("2006-01-02")
			words := noteMap[key]
			cell := "▁"
			color := lipgloss.Color("238")
			switch {
			case words == 0:
				color = lipgloss.Color("238")
				cell = "·"
			case words < 200:
				color = lipgloss.Color("24")
				cell = "▂"
			case words < 500:
				color = lipgloss.Color("31")
				cell = "▃"
			case words < 800:
				color = lipgloss.Color("37")
				cell = "▅"
			default:
				color = lipgloss.Color("43")
				cell = "█"
			}
			b.WriteString(lipgloss.NewStyle().Foreground(color).Render(cell))
			b.WriteString(" ")
		}
		b.WriteString("\n")
	}
	legend := lipgloss.NewStyle().Foreground(subtle).Render("none  low  med  high")
	return lipgloss.JoinVertical(lipgloss.Left, b.String(), legend)
}

func (m Model) renderWordCountSpark() string {
	title := lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("Word Count")
	series := m.stats.wordsSeries
	if len(series) == 0 {
		return title
	}
	spark := renderSpark(series, 40)
	// compute 30 and 50 day averages when available
	avgN := func(n int) int {
		if len(series) == 0 {
			return 0
		}
		if n > len(series) {
			n = len(series)
		}
		sum := 0
		for i := len(series) - n; i < len(series); i++ {
			if i >= 0 {
				sum += series[i]
			}
		}
		if n == 0 {
			return 0
		}
		return sum / n
	}
	avg30 := avgN(30)
	avg50 := avgN(50)
	meta := lipgloss.NewStyle().Foreground(subtle).Render(fmt.Sprintf("%d→%d-day avg", avg30, avg50))
	return lipgloss.JoinVertical(lipgloss.Left, title, spark+"  "+meta)
}

func (m Model) renderMiniTimeseries(label string, series []float64, width int, suffix string) string {
	title := lipgloss.NewStyle().Foreground(highlight).Bold(true).Render(label)
	if len(series) == 0 {
		return title
	}
	scaled := make([]int, len(series))
	// normalize -1..1 → 0..100
	for i, v := range series {
		if v < -1 {
			v = -1
		}
		if v > 1 {
			v = 1
		}
		scaled[i] = int((v + 1) * 50)
	}
	spark := renderSpark(scaled, width)
	metaText := ""
	if suffix == "Δ" {
		delta := 0.0
		if len(series) > 0 {
			delta = series[len(series)-1] - series[0]
		}
		metaText = fmt.Sprintf("Δ: %.2f", delta)
	} else {
		avg := 0.0
		for _, v := range series {
			avg += v
		}
		avg /= float64(len(series))
		metaText = fmt.Sprintf("avg: %.2f", avg)
	}
	meta := lipgloss.NewStyle().Foreground(subtle).Render(metaText)
	return lipgloss.JoinVertical(lipgloss.Left, title, spark+"  "+meta)
}

func (m Model) renderReadabilityCard() string {
	title := lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("Readability")
	grade := m.stats.avgReadability
	target := 8
	text := lipgloss.NewStyle().Foreground(subtle).Render(fmt.Sprintf("grade %.1f (target:%d)", grade, target))
	return lipgloss.JoinVertical(lipgloss.Left, title, text)
}

func (m Model) renderEmotionBars() string {
	title := lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("Emotion")
	if m.stats.emotionTotals == nil {
		return title
	}
	order := []string{"joy", "sadness", "anger", "fear", "surprise"}
	lines := []string{title}
	sumV := 0
	for _, k := range order {
		sumV += m.stats.emotionTotals[k]
	}
	if sumV == 0 {
		sumV = 1
	}
	for _, k := range order {
		v := m.stats.emotionTotals[k]
		bar := m.renderSparkBar(v, sumV, 18)
		lines = append(lines, fmt.Sprintf("%-8s %s %d%%", k, bar, int(float64(v)/float64(sumV)*100)))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderWritingTimeBars() string {
	title := lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("Writing Time")
	vals := []struct {
		label string
		v     int
	}{
		{"Morning", m.stats.morningCount},
		{"Afternoon", m.stats.afternoonCount},
		{"Evening", m.stats.eveningCount},
		{"Late Night", m.stats.lateNightCount},
	}
	maxV := 1
	for _, it := range vals {
		if it.v > maxV {
			maxV = it.v
		}
	}
	var lines []string
	lines = append(lines, title)
	for _, it := range vals {
		bar := m.renderSparkBar(it.v, maxV, 18)
		lines = append(lines, fmt.Sprintf("%-10s %s", it.label, bar))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderTopicBars() string {
	title := lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("Topics")
	if len(m.stats.topTopics) == 0 {
		return title
	}
	maxV := 1
	for _, t := range m.stats.topTopics {
		if t.count > maxV {
			maxV = t.count
		}
	}
	var lines []string
	lines = append(lines, title)
	for _, t := range m.stats.topTopics {
		bar := m.renderSparkBar(t.count, maxV, 18)
		lines = append(lines, fmt.Sprintf("%-10s %s", t.term, bar))
	}
	return strings.Join(lines, "\n")
}

// simple unicode sparkline using blocks
func renderSpark(series []int, width int) string {
	if width <= 0 {
		width = 40
	}
	if len(series) == 0 {
		return ""
	}
	// downsample if needed
	step := max(1, len(series)/width)
	minV, maxV := series[0], series[0]
	for _, v := range series {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if maxV == minV {
		maxV = minV + 1
	}
	var b strings.Builder
	for i := 0; i < len(series); i += step {
		v := series[i]
		level := int(float64(v-minV) / float64(maxV-minV) * 7)
		if level < 0 {
			level = 0
		}
		if level > 7 {
			level = 7
		}
		chars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
		b.WriteString(chars[level])
	}
	return b.String()
}

func (m Model) renderQuickStats() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(subtle)

	valueStyle := lipgloss.NewStyle().
		Foreground(highlight)

	stats := []string{
		fmt.Sprintf("%s %s",
			valueStyle.Render(fmt.Sprintf("%.0f", m.stats.avgWords)),
			labelStyle.Render("avg words/day")),
		fmt.Sprintf("%s %s",
			valueStyle.Render(fmt.Sprintf("%d", m.stats.longestStreak)),
			labelStyle.Render("longest streak")),
	}

	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(strings.Join(stats, "  •  "))
}

func (m Model) renderRecentActivity() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true)

	title := titleStyle.Render("Last 7 Days")

	// Get last 7 days
	days := []string{}
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		words := m.getWordsForDate(date)

		dayStyle := lipgloss.NewStyle()
		bar := ""
		wordStr := ""

		if words == -1 {
			// Missing day
			dayStyle = dayStyle.Foreground(warning)
			bar = strings.Repeat("─", 12)
			wordStr = "miss"
		} else if words >= dailyGoal {
			dayStyle = dayStyle.Foreground(special)
			bar = m.renderSparkBar(words, dailyGoal, 12)
			wordStr = fmt.Sprintf("%4d", words)
		} else if words > 0 {
			dayStyle = dayStyle.Foreground(lipgloss.Color("252"))
			bar = m.renderSparkBar(words, dailyGoal, 12)
			wordStr = fmt.Sprintf("%4d", words)
		} else {
			dayStyle = dayStyle.Foreground(subtle)
			bar = m.renderSparkBar(words, dailyGoal, 12)
			wordStr = fmt.Sprintf("%4d", words)
		}

		dayLine := fmt.Sprintf("%-3s %s %s",
			date.Format("Mon"),
			bar,
			wordStr,
		)
		days = append(days, dayStyle.Render(dayLine))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(days, "\n"),
	)

	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(content)
}

func (m Model) renderDaily() string {
	// Create a scrollable list of all days including missing ones
	days := []string{}

	// Get date range
	var startDate, endDate time.Time
	if len(m.stats.notes) > 0 {
		startDate = m.stats.notes[0].date
		endDate = time.Now()
	} else {
		// No notes, show last 30 days
		endDate = time.Now()
		startDate = endDate.AddDate(0, 0, -30)
	}

	// Create map for quick lookup
	noteMap := make(map[string]int)
	for _, note := range m.stats.notes {
		noteMap[note.date.Format("2006-01-02")] = note.words
	}

	// Iterate from most recent to oldest
	for date := endDate; !date.Before(startDate); date = date.AddDate(0, 0, -1) {
		dateKey := date.Format("2006-01-02")
		dateStr := date.Format("Jan 2")
		if date.Weekday() == time.Monday {
			dateStr = date.Format("Jan 2 (Mon)")
		}

		lineStyle := lipgloss.NewStyle()
		bar := ""
		wordStr := ""

		if words, exists := noteMap[dateKey]; exists {
			// Day with note
			if words >= dailyGoal {
				lineStyle = lineStyle.Foreground(special)
			} else if words > 0 {
				lineStyle = lineStyle.Foreground(lipgloss.Color("252"))
			} else {
				lineStyle = lineStyle.Foreground(subtle)
			}
			bar = m.renderSparkBar(words, dailyGoal, 15)
			wordStr = fmt.Sprintf("%4d", words)
		} else {
			// Missing day
			lineStyle = lineStyle.Foreground(warning)
			bar = strings.Repeat("─", 15)
			wordStr = "miss"
		}

		line := fmt.Sprintf("%-12s %s %s", dateStr, bar, wordStr)
		days = append(days, lineStyle.Render(line))
	}

	// Apply scrolling
	visibleHeight := m.height - 8
	startIdx := m.scrollY
	endIdx := min(startIdx+visibleHeight, len(days))

	if startIdx < len(days) {
		days = days[startIdx:endIdx]
	}

	m.maxScrollY = max(0, len(days)-visibleHeight)

	return strings.Join(days, "\n")
}

func (m Model) renderWeekly() string {
	weeks := []string{}
	for i := len(m.stats.weeklyData) - 1; i >= 0; i-- {
		week := m.stats.weeklyData[i]
		weekStr := week.startDate.Format("Jan 2")

		// Calculate expected days (7 unless it's the current week)
		expectedDays := 7
		if week.startDate.AddDate(0, 0, 7).After(time.Now()) {
			// Current week - count days from start to today
			expectedDays = int(time.Since(week.startDate).Hours()/24) + 1
			if expectedDays > 7 {
				expectedDays = 7
			}
		}

		missingDays := expectedDays - week.days
		bar := m.renderSparkBar(week.words, expectedDays*dailyGoal, 15)

		lineStyle := lipgloss.NewStyle()
		if missingDays > 0 && missingDays < expectedDays {
			// Some missing days
			lineStyle = lineStyle.Foreground(lipgloss.Color("214")) // Orange
		} else if week.avg >= float64(dailyGoal) {
			lineStyle = lineStyle.Foreground(special)
		} else if week.words > 0 {
			lineStyle = lineStyle.Foreground(lipgloss.Color("252"))
		} else {
			lineStyle = lineStyle.Foreground(subtle)
		}

		dayInfo := fmt.Sprintf("%d/%d days", week.days, expectedDays)
		line := fmt.Sprintf("Week %-7s %s %5d total • %s",
			weekStr, bar, week.words, dayInfo)
		weeks = append(weeks, lineStyle.Render(line))
	}

	return strings.Join(weeks, "\n")
}

func (m Model) renderTrendsOld() string {
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

	if m.activeTab == tabDaily {
		help = append(help, "↑↓: scroll")
	}

	help = append(help,
		"←→: tabs",
		"q: quit",
	)

	return lipgloss.NewStyle().
		Foreground(subtle).
		Width(m.width).
		Align(lipgloss.Center).
		Padding(1, 0, 0, 0).
		Render(strings.Join(help, " • "))
}

func (m Model) getWordsForDate(date time.Time) int {
	dateStr := date.Format("2006-01-02")

	// Check if this date is in the past and should have a note
	if date.Before(time.Now().AddDate(0, 0, 1)) {
		for _, note := range m.stats.notes {
			if note.date.Format("2006-01-02") == dateStr {
				return note.words
			}
		}
		// Date is in the past but has no note - it's missing
		if len(m.stats.notes) > 0 && !date.Before(m.stats.notes[0].date) {
			return -1 // Missing day indicator
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

func (m Model) renderPrompts() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(highlight).
		Bold(true).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		MarginLeft(2).
		MarginBottom(1)

	sections := []string{}

	// Check for existing prompts file
	homeDir, _ := os.UserHomeDir()
	riverDir := filepath.Join(homeDir, "river", "notes")
	promptsFile := filepath.Join(riverDir, ".prompts")

	fileInfo, err := os.Stat(promptsFile)
	if err != nil {
		// No prompts file exists
		sections = append(sections,
			titleStyle.Render("✨ AI-Generated Prompts"),
			sectionStyle.Render("No personalized prompts found.\n\nRun 'river prompts' to generate prompts based on your recent entries!"))
	} else {
		// Load and display prompts
		data, err := os.ReadFile(promptsFile)
		if err != nil {
			sections = append(sections,
				titleStyle.Render("⚠️ Error"),
				sectionStyle.Render("Could not read prompts file."))
		} else {
			lines := strings.Split(string(data), "\n")
			var prompts []string
			var generatedDate string

			for _, line := range lines {
				// Extract generation date
				if strings.HasPrefix(line, "# Generated on") {
					generatedDate = strings.TrimPrefix(line, "# Generated on ")
					continue
				}
				// Skip empty lines
				if strings.TrimSpace(line) == "" {
					continue
				}
				// Extract prompt text (format: "N. Prompt text")
				if idx := strings.Index(line, ". "); idx > 0 {
					prompt := strings.TrimSpace(line[idx+2:])
					if prompt != "" {
						prompts = append(prompts, line)
					}
				}
			}

			// Check if prompts are stale (older than 7 days)
			daysSinceGeneration := int(time.Since(fileInfo.ModTime()).Hours() / 24)
			freshnessIndicator := "🟢"
			freshnessText := fmt.Sprintf("Generated %d days ago", daysSinceGeneration)

			if daysSinceGeneration >= 5 {
				freshnessIndicator = "🟡"
				freshnessText += " (consider regenerating soon)"
			}
			if daysSinceGeneration >= 7 {
				freshnessIndicator = "🔴"
				freshnessText = "Prompts are stale! Run 'river prompts' to refresh"
			}

			sections = append(sections,
				titleStyle.Render("✨ Upcoming Journal Prompts"))

			if generatedDate != "" {
				metaStyle := lipgloss.NewStyle().
					Foreground(subtle).
					MarginLeft(2).
					MarginBottom(1)
				sections = append(sections,
					metaStyle.Render(fmt.Sprintf("%s %s", freshnessIndicator, freshnessText)))
			}

			// Show which prompt is for today
			dayOfYear := time.Now().YearDay()
			todayIndex := (dayOfYear - 1) % len(prompts)

			promptStyle := lipgloss.NewStyle().
				MarginLeft(2).
				MarginBottom(1)

			todayStyle := lipgloss.NewStyle().
				Foreground(special).
				Bold(true).
				MarginLeft(2).
				MarginBottom(1)

			for i, prompt := range prompts {
				if i == todayIndex {
					sections = append(sections,
						todayStyle.Render(fmt.Sprintf("📌 TODAY: %s", prompt)))
				} else {
					dayOffset := i - todayIndex
					if dayOffset < 0 {
						dayOffset += len(prompts)
					}
					futureDate := time.Now().AddDate(0, 0, dayOffset)
					datePrefix := futureDate.Format("Mon, Jan 2")
					sections = append(sections,
						promptStyle.Render(fmt.Sprintf("   %s: %s", datePrefix, prompt)))
				}
			}

			// Add tip
			tipStyle := lipgloss.NewStyle().
				Foreground(subtle).
				MarginTop(2).
				MarginLeft(2)
			sections = append(sections,
				tipStyle.Render("\n💡 Tip: Run 'river prompts' weekly for fresh, personalized prompts!"))
		}
	}

	return strings.Join(sections, "\n")
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

		// Basic analytics
		sent, anx, em := analyzeTextSentiment(text)
		readGrade := computeReadability(text)

		// Writing time (approx by file mtime)
		fi, _ := os.Stat(file)
		hour := 12
		if fi != nil {
			hour = fi.ModTime().Hour()
		}

		notes = append(notes, noteData{
			date:        date,
			words:       words,
			hour:        hour,
			text:        text,
			sentiment:   sent,
			anxiety:     anx,
			readability: readGrade,
			emotions:    em,
		})

		dateMap[dateStr] = words
	}

	// Sort notes by date
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].date.Before(notes[j].date)
	})

	// Calculate statistics
	stats := &stats{
		notes:     notes,
		totalDays: len(notes),
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

		// Extended analytics population
		stats.startDate = notes[0].date
		stats.endDate = time.Now()
		// Readability avg
		var totalRead float64
		stats.emotionTotals = make(map[string]int)
		for _, n := range notes {
			totalRead += n.readability
			for k, v := range n.emotions {
				stats.emotionTotals[k] += v
			}
			// writing time buckets
			switch {
			case n.hour >= 5 && n.hour < 12:
				stats.morningCount++
			case n.hour >= 12 && n.hour < 17:
				stats.afternoonCount++
			case n.hour >= 17 && n.hour < 22:
				stats.eveningCount++
			default:
				stats.lateNightCount++
			}
		}
		if len(notes) > 0 {
			stats.avgReadability = totalRead / float64(len(notes))
		}
		// Series
		buildSeries(stats)
		// Topics
		stats.topTopics = extractTopTopics(notes)
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

// --- Lightweight analytics helpers (local only, heuristic; no AI calls) ---

// computeReadability returns an approximate US grade level using
// a simplified Flesch–Kincaid style heuristic. Bounded to 3..12 range.
func computeReadability(text string) float64 {
	if text == "" {
		return 0
	}
	words := float64(len(strings.Fields(text)))
	if words == 0 {
		return 0
	}
	sentences := float64(strings.Count(text, ".") + strings.Count(text, "!") + strings.Count(text, "?"))
	if sentences == 0 {
		sentences = 1
	}
	// crude syllable estimate
	vowels := 0
	for _, ch := range strings.ToLower(text) {
		switch ch {
		case 'a', 'e', 'i', 'o', 'u', 'y':
			vowels++
		}
	}
	syllables := float64(vowels) * 0.3 // heuristic scaling
	// FKGL approximation
	grade := 0.39*(words/sentences) + 11.8*(syllables/words) - 15.59
	if grade < 3 {
		grade = 3
	}
	if grade > 12 {
		grade = 12
	}
	return grade
}

// analyzeTextSentiment returns simple sentiment/anxiety signals and an emotion map
// using keyword heuristics to avoid network calls.
func analyzeTextSentiment(text string) (sentiment float64, anxiety float64, emotions map[string]int) {
	emotions = map[string]int{"joy": 0, "sadness": 0, "anger": 0, "fear": 0, "surprise": 0}
	lower := strings.ToLower(text)
	posWords := []string{"good", "great", "love", "happy", "joy", "grateful", "proud", "excited"}
	negWords := []string{"bad", "sad", "angry", "upset", "tired", "frustrated", "anxious", "worried"}
	joyWords := []string{"joy", "happy", "smile", "grateful", "thanks", "love"}
	sadWords := []string{"sad", "down", "blue", "depressed", "cry"}
	angerWords := []string{"angry", "mad", "furious", "annoyed", "irritated"}
	fearWords := []string{"afraid", "scared", "worried", "anxious", "fear"}
	surpriseWords := []string{"surprised", "wow", "unexpected", "shocked"}

	score := 0
	for _, w := range posWords {
		if strings.Contains(lower, w) {
			score++
		}
	}
	for _, w := range negWords {
		if strings.Contains(lower, w) {
			score--
		}
	}
	// normalize to roughly -1..1
	sentiment = float64(score) / 10.0

	anxCount := 0
	for _, w := range []string{"anxious", "worried", "nervous", "stress", "panic"} {
		if strings.Contains(lower, w) {
			anxCount++
		}
	}
	anxiety = float64(anxCount) / 5.0

	countContains := func(words []string) int {
		c := 0
		for _, w := range words {
			if strings.Contains(lower, w) {
				c++
			}
		}
		return c
	}
	emotions["joy"] = countContains(joyWords)
	emotions["sadness"] = countContains(sadWords)
	emotions["anger"] = countContains(angerWords)
	emotions["fear"] = countContains(fearWords)
	emotions["surprise"] = countContains(surpriseWords)
	return
}

// buildSeries prepares 60-day series for words, sentiment, anxiety
func buildSeries(s *stats) {
	if s == nil || len(s.notes) == 0 {
		return
	}
	// last 60 days
	end := time.Now()
	start := end.AddDate(0, 0, -60)
	s.startDate, s.endDate = start, end
	dayCount := int(end.Sub(start).Hours()/24) + 1
	s.wordsSeries = make([]int, dayCount)
	s.sentimentSeries = make([]float64, dayCount)
	s.anxietySeries = make([]float64, dayCount)

	// map by date
	dateToNote := map[string]noteData{}
	for _, n := range s.notes {
		key := n.date.Format("2006-01-02")
		dateToNote[key] = n
	}
	for i := 0; i < dayCount; i++ {
		d := start.AddDate(0, 0, i)
		key := d.Format("2006-01-02")
		if n, ok := dateToNote[key]; ok {
			s.wordsSeries[i] = n.words
			s.sentimentSeries[i] = n.sentiment
			s.anxietySeries[i] = n.anxiety
		} else {
			s.wordsSeries[i] = 0
			s.sentimentSeries[i] = 0
			s.anxietySeries[i] = 0
		}
	}
}

func extractTopTopics(notes []noteData) []topicScore {
	freq := map[string]int{}
	stop := map[string]struct{}{"the": {}, "and": {}, "to": {}, "a": {}, "of": {}, "in": {}, "for": {}, "it": {}, "is": {}, "on": {}}
	for _, n := range notes {
		for _, raw := range strings.Fields(strings.ToLower(n.text)) {
			w := strings.Trim(raw, ".,!?:;()[]\"'")
			if len(w) < 4 {
				continue
			}
			if _, ok := stop[w]; ok {
				continue
			}
			if strings.HasPrefix(w, "http") {
				continue
			}
			freq[w]++
		}
	}
	// convert and sort
	var topics []topicScore
	for k, v := range freq {
		topics = append(topics, topicScore{term: k, count: v})
	}
	sort.Slice(topics, func(i, j int) bool { return topics[i].count > topics[j].count })
	if len(topics) > 6 {
		topics = topics[:6]
	}
	return topics
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

	// Count backwards - streak breaks on missing days
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

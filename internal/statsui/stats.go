package statsui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	progress "github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"

	aiclient "github.com/mattwhite/river-go/internal/ai"
)

type statsModel struct {
    stats       aggregatedStats
    viewport    viewport
    selectedTab int
    tabs        []string
    loading     bool
    error       error
    aiInsights  string
    aiLoading   bool
    aiError     error
    loader      spinner.Model
    aiLoader    spinner.Model
    goalBar     progress.Model
}

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

type viewport struct {
    width  int
    height int
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

func InitModel() statsModel {
    // Spinners for initial load and AI generation
    ld := spinner.New()
    ld.Spinner = spinner.MiniDot
    ld.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493"))

    ai := spinner.New()
    ai.Spinner = spinner.Dot
    ai.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Bold(true)

    // Progress bar used for "Today's Progress"
    gb := progress.New(
        progress.WithDefaultGradient(),
    )

    return statsModel{
        tabs:        []string{"Overview", "Daily", "Weekly", "Trends", "AI Insights"},
        selectedTab: 1,
        loading:     true,
        loader:      ld,
        aiLoader:    ai,
        goalBar:     gb,
    }
}

func (m statsModel) Init() tea.Cmd { return tea.Batch(loadStatsCmd(), m.loader.Tick) }

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
type aiInsightsMsg struct{ insights string }
type aiInsightsErrorMsg struct{ err error }

func (m statsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.viewport.width = msg.Width
        m.viewport.height = msg.Height
        // Keep a comfortable width for progress bar
        w := m.viewport.width - 30
        if w < 20 {
            w = 20
        }
        m.goalBar.Width = w
    case statsLoadedMsg:
        m.stats = msg.stats
        m.loading = false
    case statsErrorMsg:
        m.error = msg.err
        m.loading = false
    case aiInsightsMsg:
        m.aiInsights = msg.insights
        m.aiLoading = false
    case aiInsightsErrorMsg:
        m.aiError = msg.err
        m.aiLoading = false
    case spinner.TickMsg:
        var cmds []tea.Cmd
        var cmd tea.Cmd
        m.loader, cmd = m.loader.Update(msg)
        cmds = append(cmds, cmd)
        if m.aiLoading {
            m.aiLoader, cmd = m.aiLoader.Update(msg)
            cmds = append(cmds, cmd)
        }
        return m, tea.Batch(cmds...)
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c", "esc":
            return m, tea.Quit
        case "tab", "right":
            m.selectedTab = (m.selectedTab + 1) % len(m.tabs)
        case "shift+tab", "left":
            m.selectedTab = (m.selectedTab - 1 + len(m.tabs)) % len(m.tabs)
        case "g":
            if m.selectedTab == 4 && !m.aiLoading && m.aiInsights == "" {
                m.aiLoading = true
                return m, tea.Batch(m.aiLoader.Tick, generateAIInsightsCmd(m.stats))
            }
        }
    }
    return m, nil
}

func (m statsModel) View() string {
    if m.loading {
        title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF1493")).Render("River • Stats")
        sub := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render("Preparing your beautiful dashboard…")
        load := m.loader.View()
        box := lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("#444")).
            Padding(1, 2).
            Width(m.viewport.width/2 + 10).
            Align(lipgloss.Center, lipgloss.Center).
            Render(fmt.Sprintf("%s\n\n%s\n\n%s", title, load, sub))
        return lipgloss.NewStyle().Width(m.viewport.width).Height(m.viewport.height).Align(lipgloss.Center, lipgloss.Center).Render(box)
    }
    if m.error != nil {
        return lipgloss.NewStyle().Width(m.viewport.width).Height(m.viewport.height).Align(lipgloss.Center, lipgloss.Center).Foreground(lipgloss.Color("#FF0000")).Render(fmt.Sprintf("Error loading stats: %v", m.error))
    }
    header := m.renderTabs()
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
    case 4:
        content = m.renderAIInsights()
    }
    footer := m.renderFooter()
    return lipgloss.JoinVertical(lipgloss.Top, header, content, footer)
}

func (m statsModel) renderTabs() string {
    var tabs []string
    tabStyle := lipgloss.NewStyle().Padding(0, 2).Margin(0, 1).Foreground(lipgloss.Color("#999"))
    activeTabStyle := tabStyle.Copy().Foreground(lipgloss.Color("#FF1493")).Bold(true).Underline(true)
    for i, tab := range m.tabs {
        if i == m.selectedTab {
            tabs = append(tabs, activeTabStyle.Render(tab))
        } else {
            tabs = append(tabs, tabStyle.Render(tab))
        }
    }
    tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
    return lipgloss.NewStyle().Width(m.viewport.width).Padding(1, 0).BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#444")).Render(tabBar)
}

func (m statsModel) renderOverview() string {
    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF1493")).MarginBottom(1)
    statStyle := lipgloss.NewStyle().Padding(0, 1)
    labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
    valueStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF"))
    card := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("#444")).
        Padding(1, 2).
        Margin(0, 2, 1, 0)
    avgWordsPerDay := 0
    if m.stats.totalDays > 0 { avgWordsPerDay = m.stats.totalWords / m.stats.totalDays }
    avgTimePerDay := time.Duration(0)
    if m.stats.totalDays > 0 { avgTimePerDay = m.stats.totalTypingTime / time.Duration(m.stats.totalDays) }
    var content []string
    content = append(content, titleStyle.Render("📊 River Statistics Overview"))
    content = append(content, "")

    // Metrics cards
    left := []string{
        statStyle.Render(labelStyle.Render("Total Words") + "  " + valueStyle.Render(fmt.Sprintf("%d", m.stats.totalWords))),
        statStyle.Render(labelStyle.Render("Total Time") + "   " + valueStyle.Render(formatDuration(m.stats.totalTypingTime))),
        statStyle.Render(labelStyle.Render("Days Active") + " " + valueStyle.Render(fmt.Sprintf("%d", m.stats.totalDays))),
    }
    right := []string{
        statStyle.Render(labelStyle.Render("Avg Words/Day") + " " + valueStyle.Render(fmt.Sprintf("%d", avgWordsPerDay))),
        statStyle.Render(labelStyle.Render("Avg Time/Day") + "  " + valueStyle.Render(formatDuration(avgTimePerDay))),
        statStyle.Render(labelStyle.Render("Current Streak") + " " + valueStyle.Render(fmt.Sprintf("%d days", m.stats.currentStreak))),
        statStyle.Render(labelStyle.Render("Longest Streak") + " " + valueStyle.Render(fmt.Sprintf("%d days", m.stats.longestStreak))),
    }

    cards := lipgloss.JoinHorizontal(
        lipgloss.Top,
        card.Render(strings.Join(left, "\n")),
        card.Render(strings.Join(right, "\n")),
    )
    content = append(content, cards)

    // Today goal progress and sparkline
    if len(m.stats.dailyStats) > 0 {
        today := m.stats.dailyStats[len(m.stats.dailyStats)-1]
        if today.date.Format("2006-01-02") == time.Now().Format("2006-01-02") {
            content = append(content, "")
            content = append(content, titleStyle.Render("Today's Progress"))
            goal := 500
            pct := float64(today.words) / float64(goal)
            if pct > 1 {
                pct = 1
            }
            bar := m.goalBar.ViewAs(pct)
            meta := lipgloss.NewStyle().Foreground(lipgloss.Color("#999")).Render(fmt.Sprintf("%d/%d words", today.words, goal))
            content = append(content, lipgloss.JoinHorizontal(lipgloss.Top, bar, " ", meta))
        }
    }

    // Recent 14-day sparkline
    if len(m.stats.dailyStats) > 0 {
        content = append(content, "")
        content = append(content, titleStyle.Render("Last 14 Days"))
        content = append(content, m.renderSparkline(14))
    }

    // Most productive day callout
    if m.stats.mostProductiveDay != "" {
        callout := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Bold(true).Render("★ ") +
            lipgloss.NewStyle().Bold(true).Render("Most productive: ") +
            lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF")).Render(fmt.Sprintf("%s (%d words)", m.stats.mostProductiveDay, m.stats.mostProductiveWords))
        content = append(content, "")
        content = append(content, callout)
    }

    return lipgloss.NewStyle().Padding(2).Render(strings.Join(content, "\n"))
}

func (m statsModel) renderDaily() string {
    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF1493")).MarginBottom(1)
    var content []string
    content = append(content, titleStyle.Render("📅 Daily Statistics"))
    content = append(content, "")
    endDate := time.Now()
    startDate := endDate.AddDate(0, 0, -13)
    statsMap := make(map[string]dailyStat)
    for _, stat := range m.stats.dailyStats { statsMap[stat.date.Format("2006-01-02")] = stat }
    for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
        dateKey := d.Format("2006-01-02")
        dateStr := d.Format("Mon, Jan 2")
        if dateKey == time.Now().Format("2006-01-02") { dateStr += " (Today)" }
        if stat, exists := statsMap[dateKey]; exists {
            bar := m.renderMiniBar(stat.words, 1000, 30)
            timeStr := formatDuration(stat.typingTime)
            line := fmt.Sprintf("%-20s %s %5d words %8s", dateStr, bar, stat.words, timeStr)
            if dateKey == time.Now().Format("2006-01-02") {
                line = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Render(line)
            }
            content = append(content, line)
        } else {
            emptyBar := m.renderEmptyBar(30)
            line := fmt.Sprintf("%-20s %s %5s %8s", dateStr, emptyBar, "-", "-")
            line = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(line)
            content = append(content, line)
        }
    }
    return lipgloss.NewStyle().Padding(2).Render(strings.Join(content, "\n"))
}

func (m statsModel) renderWeekly() string {
    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF1493")).MarginBottom(1)
    var content []string
    content = append(content, titleStyle.Render("📈 Weekly Statistics"))
    content = append(content, "")
    startIdx := len(m.stats.weeklyStats) - 8
    if startIdx < 0 { startIdx = 0 }
    for i := startIdx; i < len(m.stats.weeklyStats); i++ {
        stat := m.stats.weeklyStats[i]
        weekStr := fmt.Sprintf("Week of %s", stat.weekStart.Format("Jan 2"))
        avgWords := 0
        if stat.daysActive > 0 { avgWords = stat.totalWords / stat.daysActive }
        bar := m.renderMiniBar(stat.totalWords, 5000, 25)
        line := fmt.Sprintf("%-20s %s %5d words (%d days, avg %d/day)", weekStr, bar, stat.totalWords, stat.daysActive, avgWords)
        content = append(content, line)
    }
    return lipgloss.NewStyle().Padding(2).Render(strings.Join(content, "\n"))
}

func (m statsModel) renderTrends() string {
    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF1493")).MarginBottom(1)
    var content []string
    content = append(content, titleStyle.Render("📈 Writing Trends"))
    content = append(content, "")
    content = append(content, lipgloss.NewStyle().Bold(true).Render("Monthly Totals:"))
    content = append(content, "")
    var months []string
    for month := range m.stats.monthlyTotals { months = append(months, month) }
    sort.Strings(months)
    startIdx := len(months) - 6
    if startIdx < 0 { startIdx = 0 }
    for i := startIdx; i < len(months); i++ {
        month := months[i]
        stat := m.stats.monthlyTotals[month]
        monthTime, _ := time.Parse("2006-01", month)
        monthStr := monthTime.Format("January 2006")
        bar := m.renderMiniBar(stat.totalWords, 20000, 25)
        line := fmt.Sprintf("%-20s %s %6d words (%d days active)", monthStr, bar, stat.totalWords, stat.daysActive)
        content = append(content, line)
    }
    content = append(content, "")
    content = append(content, lipgloss.NewStyle().Bold(true).Render("Insights:"))
    content = append(content, "")
    content = append(content, fmt.Sprintf("• Most productive day: %s", m.stats.mostProductiveDay))
    content = append(content, fmt.Sprintf("• Average session: %s", formatDuration(m.stats.totalTypingTime/time.Duration(m.stats.totalDays))))
    content = append(content, fmt.Sprintf("• Total writing time: %s", formatDuration(m.stats.totalTypingTime)))
    return lipgloss.NewStyle().Padding(2).Render(strings.Join(content, "\n"))
}

func (m statsModel) renderAIInsights() string {
    titleStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FF1493")).
        MarginBottom(1)

    var content []string
    content = append(content, titleStyle.Render("🤖 AI-Powered Writing Insights"))
    content = append(content, "")

    if m.aiLoading {
        loading := lipgloss.JoinVertical(
            lipgloss.Left,
            lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Render("🔮 Analyzing your writing patterns…"),
            "",
            m.aiLoader.View(),
            lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render("This may take a few moments as AI reviews your recent notes."),
        )
        return lipgloss.NewStyle().Padding(2, 0).Render(loading)
    }

    if m.aiError != nil {
        errorStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FF6B6B")).
            Padding(2, 0)
        errorMsg := fmt.Sprintf("❌ Error generating insights: %v", m.aiError)
        if strings.Contains(m.aiError.Error(), "ANTHROPIC_API_KEY") {
            errorMsg += "\n\n💡 Tip: Set your API key with:\n   export ANTHROPIC_API_KEY=your_key_here"
        }
        return errorStyle.Render(errorMsg)
    }

    if m.aiInsights == "" {
        buttonStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color("#FF1493")).
            Bold(true).
            Padding(1, 0)
        helpStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color("#888")).
            Padding(1, 0)
        content = append(content, buttonStyle.Render("Press 'g' to generate AI insights from your recent writing"))
        content = append(content, "")
        content = append(content, helpStyle.Render("AI will analyze:"))
        content = append(content, helpStyle.Render("• Your writing patterns and themes"))
        content = append(content, helpStyle.Render("• Productivity trends and habits"))
        content = append(content, helpStyle.Render("• Emotional patterns in your entries"))
        content = append(content, helpStyle.Render("• Personalized recommendations"))
        content = append(content, "")
        content = append(content, helpStyle.Render("Note: This feature requires an Anthropic API key"))
    } else {
        content = append(content, m.aiInsights)
    }

    return lipgloss.NewStyle().Padding(2).Render(strings.Join(content, "\n"))
}

func (m statsModel) renderFooter() string {
    helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666")).Padding(1, 2)
    help := "Tab/→: Next tab • Shift+Tab/←: Previous tab • g: Generate AI insights • q/Esc: Quit"
    return lipgloss.NewStyle().Width(m.viewport.width).BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#444")).Render(helpStyle.Render(help))
}

func (m statsModel) renderProgressBar(current, target, width int) string {
    progress := float64(current) / float64(target)
    if progress > 1.0 { progress = 1.0 }
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
    if progress > 1.0 { progress = 1.0 }
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
    for i := 0; i < width; i++ { bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#333")).Render("─")) }
    return bar.String()
}

// renderSparkline shows the last n days as a unicode sparkline.
func (m statsModel) renderSparkline(days int) string {
    if len(m.stats.dailyStats) == 0 {
        return ""
    }
    blocks := []rune("▁▂▃▄▅▆▇█")
    start := len(m.stats.dailyStats) - days
    if start < 0 {
        start = 0
    }
    slice := m.stats.dailyStats[start:]
    max := 0
    for _, d := range slice {
        if d.words > max {
            max = d.words
        }
    }
    if max == 0 {
        return strings.Repeat(string(blocks[0])+" ", len(slice))
    }
    var b strings.Builder
    for _, d := range slice {
        idx := int(float64(d.words) / float64(max) * float64(len(blocks)-1))
        if idx < 0 {
            idx = 0
        }
        if idx > len(blocks)-1 {
            idx = len(blocks) - 1
        }
        b.WriteRune(blocks[idx])
        b.WriteRune(' ')
    }
    label := lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render("(words per day)")
    return lipgloss.JoinHorizontal(lipgloss.Top, b.String(), label)
}

func formatDuration(d time.Duration) string {
    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60
    if hours > 0 { return fmt.Sprintf("%dh %dm", hours, minutes) }
    return fmt.Sprintf("%dm", minutes)
}

func collectAllStats() (aggregatedStats, error) {
    homeDir, err := os.UserHomeDir()
    if err != nil { return aggregatedStats{}, err }
    riverDir := filepath.Join(homeDir, "river", "notes")
    files, err := filepath.Glob(filepath.Join(riverDir, "*.md"))
    if err != nil { return aggregatedStats{}, err }
    var allStats aggregatedStats
    allStats.monthlyTotals = make(map[string]monthStat)
    for _, file := range files {
        base := filepath.Base(file)
        dateStr := strings.TrimSuffix(base, ".md")
        date, err := time.Parse("2006-01-02", dateStr)
        if err != nil { continue }
        content, err := os.ReadFile(file)
        if err != nil { continue }
        words := countWords(string(content))
        statsFile := filepath.Join(riverDir, ".stats-"+dateStr+".toml")
        typingTime := time.Duration(0)
        if data, err := os.ReadFile(statsFile); err == nil {
            var s struct{ TypingSeconds int `toml:"typing_seconds"` }
            if err := toml.Unmarshal(data, &s); err == nil { typingTime = time.Duration(s.TypingSeconds) * time.Second }
        }
        daily := dailyStat{date: date, words: words, typingTime: typingTime}
        allStats.dailyStats = append(allStats.dailyStats, daily)
        allStats.totalWords += words
        allStats.totalTypingTime += typingTime
        if words > allStats.mostProductiveWords {
            allStats.mostProductiveWords = words
            allStats.mostProductiveDay = date.Format("Jan 2, 2006")
        }
        monthKey := date.Format("2006-01")
        month := allStats.monthlyTotals[monthKey]
        month.totalWords += words
        month.totalTime += typingTime
        month.daysActive++
        allStats.monthlyTotals[monthKey] = month
    }
    sort.Slice(allStats.dailyStats, func(i, j int) bool { return allStats.dailyStats[i].date.Before(allStats.dailyStats[j].date) })
    allStats.totalDays = len(allStats.dailyStats)
    allStats.currentStreak = calculateCurrentStreak(allStats.dailyStats)
    allStats.longestStreak = calculateLongestStreak(allStats.dailyStats)
    allStats.weeklyStats = calculateWeeklyStats(allStats.dailyStats)
    return allStats, nil
}

func countWords(content string) int { return len(strings.Fields(content)) }

func calculateCurrentStreak(dailyStats []dailyStat) int {
    if len(dailyStats) == 0 { return 0 }
    dateMap := make(map[string]bool)
    for _, stat := range dailyStats { dateMap[stat.date.Format("2006-01-02")] = true }
    streak := 0
    today := time.Now()
    todayKey := today.Format("2006-01-02")
    if !dateMap[todayKey] { return 0 }
    for d := today; ; d = d.AddDate(0, 0, -1) {
        dateKey := d.Format("2006-01-02")
        if dateMap[dateKey] { streak++ } else { break }
    }
    return streak
}

func calculateLongestStreak(dailyStats []dailyStat) int {
    if len(dailyStats) == 0 { return 0 }
    longestStreak := 1
    currentStreak := 1
    for i := 1; i < len(dailyStats); i++ {
        prevDate := dailyStats[i-1].date
        currDate := dailyStats[i].date
        if currDate.Sub(prevDate).Hours() == 24 {
            currentStreak++
            if currentStreak > longestStreak { longestStreak = currentStreak }
        } else {
            currentStreak = 1
        }
    }
    return longestStreak
}

func calculateWeeklyStats(dailyStats []dailyStat) []weeklyStat {
    if len(dailyStats) == 0 { return nil }
    var weeklyStats []weeklyStat
    weekMap := make(map[string]*weeklyStat)
    for _, daily := range dailyStats {
        weekStart := daily.date
        for weekStart.Weekday() != time.Sunday { weekStart = weekStart.AddDate(0, 0, -1) }
        weekKey := weekStart.Format("2006-01-02")
        if week, exists := weekMap[weekKey]; exists {
            week.totalWords += daily.words
            week.totalTime += daily.typingTime
            week.daysActive++
        } else {
            weekMap[weekKey] = &weeklyStat{weekStart: weekStart, totalWords: daily.words, totalTime: daily.typingTime, daysActive: 1}
        }
    }
    for _, week := range weekMap { weeklyStats = append(weeklyStats, *week) }
    sort.Slice(weeklyStats, func(i, j int) bool { return weeklyStats[i].weekStart.Before(weeklyStats[j].weekStart) })
    return weeklyStats
}

func generateAIInsightsCmd(stats aggregatedStats) tea.Cmd {
    return func() tea.Msg {
        notes, err := aiclient.GenerateNotesForInsights()
        if err != nil {
            return aiInsightsErrorMsg{err: fmt.Errorf("error reading notes: %v", err)}
        }
        avgWords := 0
        if stats.totalDays > 0 { avgWords = stats.totalWords / stats.totalDays }
        converted := aiclient.AggregatedStats{
            TotalWords:          stats.totalWords,
            TotalTypingTime:     stats.totalTypingTime,
            TotalDays:           stats.totalDays,
            CurrentStreak:       stats.currentStreak,
            LongestStreak:       stats.longestStreak,
            AverageWords:        avgWords,
            MostProductiveDay:   stats.mostProductiveDay,
            MostProductiveWords: stats.mostProductiveWords,
        }
        for _, d := range stats.dailyStats {
            converted.DailyStats = append(converted.DailyStats, aiclient.DailyStat{Date: d.date, Words: d.words, TypingTime: d.typingTime})
        }
        insights, err := aiclient.CallAnthropicForStatsInsights(converted, notes)
        if err != nil { return aiInsightsErrorMsg{err: err} }
        return aiInsightsMsg{insights: insights}
    }
}



package statsui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	help "github.com/charmbracelet/bubbles/help"
	key "github.com/charmbracelet/bubbles/key"
	paginator "github.com/charmbracelet/bubbles/paginator"
	progress "github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	btable "github.com/charmbracelet/bubbles/table"
	bviewport "github.com/charmbracelet/bubbles/viewport"
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
    contentVP   bviewport.Model
    help        help.Model
    keys        keymap
    dailyPaginator paginator.Model
    dailyPageSize  int
    weeklyTable btable.Model
    trendsTable btable.Model
    sidebarWidth int
    showHelp     bool
    // animated counters for a premium feel
    animate      bool
    totalWordsAnim int
    totalTimeAnim  time.Duration
    daysActiveAnim int
    streakAnim     int
    longestStreakAnim int
    lastAnimAt   time.Time
}

func (m statsModel) mainWidth() int { return m.viewport.width - m.sidebarWidth }

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

    m := statsModel{
        tabs:        []string{"Overview", "Daily", "Weekly", "Trends", "AI Insights"},
        selectedTab: 1,
        loading:     true,
        loader:      ld,
        aiLoader:    ai,
        goalBar:     gb,
        help:        help.New(),
        keys:        newKeymap(),
        contentVP:   bviewport.New(0, 0),
        dailyPaginator: paginator.New(),
        dailyPageSize:  14,
        sidebarWidth:  28,
        showHelp:      false,
        animate:       true,
    }
    m.dailyPaginator.Type = paginator.Dots
    m.dailyPaginator.PerPage = m.dailyPageSize
    return m
}

type animTickMsg time.Time

func animTickCmd() tea.Cmd {
    return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg { return animTickMsg(t) })
}

func (m statsModel) Init() tea.Cmd { return tea.Batch(loadStatsCmd(), m.loader.Tick, animTickCmd()) }

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
        w := m.mainWidth() - 6
        if w < 20 {
            w = 20
        }
        m.goalBar.Width = w
        // Resize viewport and page size
        contentHeight := m.viewport.height - 3
        if contentHeight < 5 {
            contentHeight = 5
        }
        m.contentVP.Width = m.mainWidth()
        m.contentVP.Height = contentHeight
        m.dailyPageSize = contentHeight - 6
        if m.dailyPageSize < 7 {
            m.dailyPageSize = 7
        }
        m.dailyPaginator.PerPage = m.dailyPageSize
        m.rebuildTables()
    case statsLoadedMsg:
        m.stats = msg.stats
        m.loading = false
        // reset counters for animation
        m.totalWordsAnim = 0
        m.totalTimeAnim = 0
        m.daysActiveAnim = 0
        m.streakAnim = 0
        m.longestStreakAnim = 0
        m.lastAnimAt = time.Now()
        m.rebuildTables()
        m.updateDailyPaginatorTotal()
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
    case animTickMsg:
        if !m.loading && m.animate {
            // animate towards targets
            done := true
            stepInt := func(curr, target int) (int, bool) {
                if curr >= target { return target, true }
                diff := target - curr
                inc := diff / 7
                if inc < 1 { inc = 1 }
                v := curr + inc
                if v > target { v = target }
                return v, v == target
            }
            stepDur := func(curr, target time.Duration) (time.Duration, bool) {
                if curr >= target { return target, true }
                diff := target - curr
                inc := diff / 7
                if inc < time.Second { inc = time.Second }
                v := curr + inc
                if v > target { v = target }
                return v, v == target
            }
            var ok bool
            m.totalWordsAnim, ok = stepInt(m.totalWordsAnim, m.stats.totalWords)
            if !ok { done = false }
            m.totalTimeAnim, ok = stepDur(m.totalTimeAnim, m.stats.totalTypingTime)
            if !ok { done = false }
            m.daysActiveAnim, ok = stepInt(m.daysActiveAnim, m.stats.totalDays)
            if !ok { done = false }
            m.streakAnim, ok = stepInt(m.streakAnim, m.stats.currentStreak)
            if !ok { done = false }
            m.longestStreakAnim, ok = stepInt(m.longestStreakAnim, m.stats.longestStreak)
            if !ok { done = false }
            if done {
                m.animate = false
            }
            return m, animTickCmd()
        }
        return m, nil
    case tea.KeyMsg:
        switch msg.String() {
        case "q", "ctrl+c", "esc":
            return m, tea.Quit
        case "tab", "right":
            m.selectedTab = (m.selectedTab + 1) % len(m.tabs)
        case "shift+tab", "left":
            m.selectedTab = (m.selectedTab - 1 + len(m.tabs)) % len(m.tabs)
        case "?", "h":
            m.showHelp = !m.showHelp
        case "j", "down":
            m.contentVP.LineDown(1)
        case "k", "up":
            m.contentVP.LineUp(1)
        case "pgdown", "f":
            m.contentVP.HalfViewDown()
        case "pgup", "b":
            m.contentVP.HalfViewUp()
        case "home", "g" + "g":
            m.contentVP.GotoTop()
        case "end", "G":
            m.contentVP.GotoBottom()
        case "g":
            if m.selectedTab == 4 && !m.aiLoading && m.aiInsights == "" {
                m.aiLoading = true
                return m, tea.Batch(m.aiLoader.Tick, generateAIInsightsCmd(m.stats))
            }
        case "[":
            if m.selectedTab == 1 {
                m.dailyPaginator.PrevPage()
            }
        case "]":
            if m.selectedTab == 1 {
                m.dailyPaginator.NextPage()
            }
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
            Render("Loading...")
    }
    if m.error != nil {
        return lipgloss.NewStyle().
            Width(m.viewport.width).
            Height(m.viewport.height).
            Align(lipgloss.Center, lipgloss.Center).
            Render(fmt.Sprintf("Error: %v", m.error))
    }
    
    // Just a minimal page with basic info
    content := lipgloss.NewStyle().
        Padding(2).
        Render(fmt.Sprintf("Words: %d\nDays: %d\nStreak: %d\n\nPress q to quit", 
            m.stats.totalWords,
            m.stats.totalDays,
            m.stats.currentStreak))
    
    return lipgloss.NewStyle().
        Width(m.viewport.width).
        Height(m.viewport.height).
        Render(content)
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

func (m statsModel) renderSidebar() string {
    // Sidebar with tabs and quick stats
    width := m.sidebarWidth
    title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF1493")).Render("River")
    subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAA")).Render("Dashboard")
    // tabs list
    items := []string{
        m.renderSidebarTab(0, "Overview", "🏠"),
        m.renderSidebarTab(1, "Daily", "📅"),
        m.renderSidebarTab(2, "Weekly", "📈"),
        m.renderSidebarTab(3, "Trends", "📊"),
        m.renderSidebarTab(4, "AI Insights", "🤖"),
    }
    quick := []string{
        lipgloss.NewStyle().Foreground(lipgloss.Color("#888")).Render("Quick Stats"),
        fmt.Sprintf("Words: %d", m.totalWordsAnim),
        fmt.Sprintf("Time: %s", formatDuration(m.totalTimeAnim)),
        fmt.Sprintf("Streak: %d", m.streakAnim),
    }
    box := lipgloss.NewStyle().
        Width(width).
        Height(m.viewport.height). // stretch full height for symmetry
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("#444")).
        Padding(1, 2)
    content := lipgloss.JoinVertical(lipgloss.Left,
        title,
        subtitle,
        "",
        strings.Join(items, "\n"),
        "",
        strings.Join(quick, "\n"),
        "",
        lipgloss.NewStyle().Foreground(lipgloss.Color("#777")).Render("? for help"),
    )
    return box.Render(content)
}

func (m statsModel) renderSidebarTab(index int, label, icon string) string {
    style := lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("#AAA"))
    active := style.Copy().Foreground(lipgloss.Color("#FF1493")).Bold(true)
    s := fmt.Sprintf("%s %s", icon, label)
    if index == m.selectedTab {
        return active.Render(s)
    }
    return style.Render(s)
}

func (m statsModel) renderHeaderBanner() string {
    // Big banner with animated counters
    title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB6C1")).Render("Writing Stats")
    subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("#DDD")).Render("Beautiful insights into your writing practice")
    
    // Simple counter display without boxes
    counterStyle := lipgloss.NewStyle().Padding(1, 3)
    labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999"))
    valueStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF"))
    
    counters := lipgloss.JoinHorizontal(lipgloss.Top,
        counterStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
            labelStyle.Render("Words"),
            valueStyle.Render(fmt.Sprintf("%d", m.totalWordsAnim)))),
        counterStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
            labelStyle.Render("Time"),
            valueStyle.Render(formatDuration(m.totalTimeAnim)))),
        counterStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
            labelStyle.Render("Days"),
            valueStyle.Render(fmt.Sprintf("%d", m.daysActiveAnim)))),
        counterStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
            labelStyle.Render("Streak"),
            valueStyle.Render(fmt.Sprintf("%d", m.streakAnim)))),
        counterStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
            labelStyle.Render("Longest"),
            valueStyle.Render(fmt.Sprintf("%d", m.longestStreakAnim)))),
    )
    
    banner := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("#444")).
        Background(lipgloss.Color("#2A0F25")).
        Padding(1, 2).
        Width(m.mainWidth() - 4).
        Align(lipgloss.Center)
    
    return banner.Render(lipgloss.JoinVertical(lipgloss.Center, title, subtitle, counters))
}

func (m statsModel) counterCard(label, value string) string {
    l := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAA")).Render(label)
    v := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF")).Render(value)
    card := lipgloss.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("#444")).
        Padding(1, 2).MarginRight(2)
    return card.Render(lipgloss.JoinVertical(lipgloss.Left, l, v))
}

func (m statsModel) counterCardFixed(label, value string, width int) string {
    l := lipgloss.NewStyle().Foreground(lipgloss.Color("#AAA")).Align(lipgloss.Center).Width(width-6).Render(label)
    v := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF")).Align(lipgloss.Center).Width(width-6).Render(value)
    card := lipgloss.NewStyle().
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("#444")).
        Padding(0, 1).
        Width(width).
        MarginRight(2)
    return card.Render(lipgloss.JoinVertical(lipgloss.Center, l, v))
}

func (m statsModel) renderHelpOverlay() string {
    helpContent := m.help.View(m.keys)
    box := lipgloss.NewStyle().
        Width(m.viewport.width - 8).
        Height(m.viewport.height - 6).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("#FF1493")).
        Padding(1, 2)
    overlay := box.Render(helpContent + "\n\nPress 'h' or '?' to close")
    return lipgloss.NewStyle().Width(m.viewport.width).Height(m.viewport.height).Align(lipgloss.Center, lipgloss.Center).Render(overlay)
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

    // Determine window based on paginator page
    today := time.Now()
    pages := m.dailyPaginator.TotalPages
    if pages < 1 {
        pages = 1
    }
    pageFromEnd := (pages - 1) - m.dailyPaginator.Page
    endDate := today.AddDate(0, 0, -pageFromEnd*m.dailyPageSize)
    startDate := endDate.AddDate(0, 0, -(m.dailyPageSize - 1))

    statsMap := make(map[string]dailyStat)
    for _, stat := range m.stats.dailyStats {
        statsMap[stat.date.Format("2006-01-02")] = stat
    }
    
    for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
        dateKey := d.Format("2006-01-02")
        dateStr := d.Format("Mon, Jan 2")
        if dateKey == today.Format("2006-01-02") {
            dateStr += " (Today)"
        }
        
        if stat, exists := statsMap[dateKey]; exists {
            bar := m.renderMiniBar(stat.words, 1000, 30)
            timeStr := formatDuration(stat.typingTime)
            line := fmt.Sprintf("%-20s %s %6d words  %7s", dateStr, bar, stat.words, timeStr)
            if dateKey == today.Format("2006-01-02") {
                line = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Render(line)
            }
            content = append(content, line)
        } else {
            emptyBar := m.renderEmptyBar(30)
            line := fmt.Sprintf("%-20s %s      -         -", dateStr, emptyBar)
            line = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render(line)
            content = append(content, line)
        }
    }
    // Paginator control
    content = append(content, "")
    content = append(content, m.dailyPaginator.View())
    return lipgloss.NewStyle().Padding(1, 2).Render(strings.Join(content, "\n"))
}

func (m statsModel) renderWeekly() string {
    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF1493")).MarginBottom(1)
    header := titleStyle.Render("📈 Weekly Statistics") + "\n\n"
    return lipgloss.NewStyle().Padding(2).Render(header + m.weeklyTable.View())
}

func (m statsModel) renderTrends() string {
    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF1493")).MarginBottom(1)
    header := titleStyle.Render("📈 Writing Trends") + "\n\n" + lipgloss.NewStyle().Bold(true).Render("Monthly Totals:") + "\n\n"
    insights := []string{
        fmt.Sprintf("• Most productive day: %s", m.stats.mostProductiveDay),
    }
    if m.stats.totalDays > 0 {
        insights = append(insights, fmt.Sprintf("• Average session: %s", formatDuration(m.stats.totalTypingTime/time.Duration(m.stats.totalDays))))
    }
    insights = append(insights, fmt.Sprintf("• Total writing time: %s", formatDuration(m.stats.totalTypingTime)))
    return lipgloss.NewStyle().Padding(2).Render(header + m.trendsTable.View() + "\n\n" + strings.Join(insights, "\n"))
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
    helpView := m.help.View(m.keys)
    // Footer aligned with main content area
    return lipgloss.NewStyle().
        Width(m.mainWidth()).
        BorderTop(true).
        BorderStyle(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("#444")).
        Padding(0, 2).
        Render(helpView)
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

// keymap and helpers
type keymap struct {
    NextTab       key.Binding
    PrevTab       key.Binding
    Quit          key.Binding
    GenerateAI    key.Binding
    ScrollDown    key.Binding
    ScrollUp      key.Binding
    PageDown      key.Binding
    PageUp        key.Binding
    Top           key.Binding
    Bottom        key.Binding
    DailyPrevPage key.Binding
    DailyNextPage key.Binding
}

func newKeymap() keymap {
    return keymap{
        NextTab: key.NewBinding(key.WithKeys("tab", "right"), key.WithHelp("tab/→", "next tab")),
        PrevTab: key.NewBinding(key.WithKeys("shift+tab", "left"), key.WithHelp("⇧tab/←", "prev tab")),
        Quit: key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q/esc", "quit")),
        GenerateAI: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "AI insights (Insights tab)")),
        ScrollDown: key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "scroll down")),
        ScrollUp: key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "scroll up")),
        PageDown: key.NewBinding(key.WithKeys("pgdown", "f"), key.WithHelp("pgdn/f", "half page ↓")),
        PageUp: key.NewBinding(key.WithKeys("pgup", "b"), key.WithHelp("pgup/b", "half page ↑")),
        Top: key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
        Bottom: key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
        DailyPrevPage: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev page (Daily)")),
        DailyNextPage: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next page (Daily)")),
    }
}

func (k keymap) ShortHelp() []key.Binding { return []key.Binding{k.PrevTab, k.NextTab, k.Quit} }

func (k keymap) FullHelp() [][]key.Binding {
    return [][]key.Binding{
        {k.PrevTab, k.NextTab, k.DailyPrevPage, k.DailyNextPage},
        {k.ScrollUp, k.ScrollDown, k.PageUp, k.PageDown, k.Top, k.Bottom},
        {k.GenerateAI, k.Quit},
    }
}

// build tables and paginator totals when stats/size change
func (m *statsModel) rebuildTables() {
    // Weekly table (last 8 weeks)
    columns := []btable.Column{
        {Title: "Week", Width: 14},
        {Title: "Words", Width: 10},
        {Title: "Days", Width: 6},
        {Title: "Avg/Day", Width: 10},
    }
    var rows []btable.Row
    startIdx := len(m.stats.weeklyStats) - 8
    if startIdx < 0 {
        startIdx = 0
    }
    for i := startIdx; i < len(m.stats.weeklyStats); i++ {
        stat := m.stats.weeklyStats[i]
        avg := 0
        if stat.daysActive > 0 {
            avg = stat.totalWords / stat.daysActive
        }
        rows = append(rows, btable.Row{
            stat.weekStart.Format("Jan 2"),
            fmt.Sprintf("%d", stat.totalWords),
            fmt.Sprintf("%d", stat.daysActive),
            fmt.Sprintf("%d", avg),
        })
    }
    t := btable.New(btable.WithColumns(columns), btable.WithRows(rows), btable.WithFocused(true))
    t.SetStyles(tableStyles())
    width := m.mainWidth() - 6
    if width < 40 {
        width = m.mainWidth() - 2
    }
    t.SetWidth(width)
    m.weeklyTable = t

    // Trends monthly table (last 6 months)
    var months []string
    for month := range m.stats.monthlyTotals {
        months = append(months, month)
    }
    sort.Strings(months)
    startM := len(months) - 6
    if startM < 0 {
        startM = 0
    }
    mcols := []btable.Column{{Title: "Month", Width: 18}, {Title: "Words", Width: 10}, {Title: "Days", Width: 6}, {Title: "Time", Width: 10}}
    var mrows []btable.Row
    for i := startM; i < len(months); i++ {
        key := months[i]
        st := m.stats.monthlyTotals[key]
        mt, _ := time.Parse("2006-01", key)
        mrows = append(mrows, btable.Row{
            mt.Format("Jan 2006"),
            fmt.Sprintf("%d", st.totalWords),
            fmt.Sprintf("%d", st.daysActive),
            formatDuration(st.totalTime),
        })
    }
    tt := btable.New(btable.WithColumns(mcols), btable.WithRows(mrows), btable.WithFocused(true))
    tt.SetStyles(tableStyles())
    tt.SetWidth(width)
    m.trendsTable = tt
}

func (m *statsModel) updateDailyPaginatorTotal() {
    totalDays := len(m.stats.dailyStats)
    if totalDays == 0 {
        m.dailyPaginator.SetTotalPages(1)
        return
    }
    // Consider last 30 days for pagination window
    window := 30
    if totalDays < window {
        window = totalDays
    }
    pages := window / m.dailyPageSize
    if window%m.dailyPageSize != 0 {
        pages++
    }
    if pages == 0 {
        pages = 1
    }
    m.dailyPaginator.SetTotalPages(pages)
    // Move to last page by default (most recent)
    for m.dailyPaginator.Page < m.dailyPaginator.TotalPages-1 {
        m.dailyPaginator.NextPage()
    }
}

func tableStyles() btable.Styles {
    s := btable.DefaultStyles()
    s.Header = s.Header.
        BorderStyle(lipgloss.NormalBorder()).
        BorderBottom(true).
        Bold(true).
        BorderForeground(lipgloss.Color("#444")).
        Foreground(lipgloss.Color("#FF1493"))
    s.Selected = s.Selected.
        Foreground(lipgloss.Color("#FFF")).
        Background(lipgloss.Color("#4A1242"))
    s.Cell = s.Cell.Foreground(lipgloss.Color("#DDD"))
    return s
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
    
    // Check if we have an entry for today or yesterday
    yesterdayKey := today.AddDate(0, 0, -1).Format("2006-01-02")
    if !dateMap[todayKey] && !dateMap[yesterdayKey] { 
        return 0 
    }
    
    // Start from today if we have it, otherwise from yesterday
    startDate := today
    if !dateMap[todayKey] {
        startDate = today.AddDate(0, 0, -1)
    }
    
    for d := startDate; ; d = d.AddDate(0, 0, -1) {
        dateKey := d.Format("2006-01-02")
        if dateMap[dateKey] { 
            streak++ 
        } else { 
            break 
        }
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



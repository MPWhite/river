package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pelletier/go-toml/v2"
)

type Model struct {
    content      []string
    cursor       position
    viewport     viewport
    filename     string
    modified     bool
    lastActivity time.Time
    typingTime   time.Duration
    startTime    time.Time
    statsFile    string
}

type position struct {
    row int
    col int
}

type viewport struct {
    width  int
    height int
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

func loadOrCreateTodayFile() ([]string, string, error) {
    today := time.Now().Format("2006-01-02")
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return nil, "", err
    }

    riverDir := filepath.Join(homeDir, "river", "notes")
    filename := filepath.Join(riverDir, today+".md")

    if err := os.MkdirAll(riverDir, 0755); err != nil {
        return nil, "", err
    }

    content, err := os.ReadFile(filename)
    if err != nil {
        if os.IsNotExist(err) {
            template := createDailyNoteTemplate()
            return strings.Split(template, "\n"), filename, nil
        }
        return nil, "", err
    }

    lines := strings.Split(string(content), "\n")
    if len(lines) == 0 {
        lines = []string{""}
    }

    return lines, filename, nil
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

func saveStats(statsFile string, typingTime time.Duration, content []string) error {
    wordCount := 0
    for _, line := range content {
        words := strings.Fields(line)
        wordCount += len(words)
    }

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

func saveFile(filename string, content []string) error {
    data := strings.Join(content, "\n")
    return os.WriteFile(filename, []byte(data), 0644)
}

func NewInitialModel() Model {
    content, filename, err := loadOrCreateTodayFile()
    if err != nil {
        content = []string{fmt.Sprintf("Error loading file: %v", err)}
        filename = "error.txt"
    }

    lastRow := len(content) - 1
    if lastRow < 0 {
        lastRow = 0
    }
    lastCol := len(content[lastRow])

    if lastRow >= 0 && len(content[lastRow]) > 0 {
        content = append(content, "")
        lastRow++
        lastCol = 0
    }

    today := time.Now().Format("2006-01-02")
    dir := filepath.Dir(filename)
    statsFile := filepath.Join(dir, ".stats-"+today+".toml")

    existingTime, _ := loadStats(statsFile)

    now := time.Now()
    return Model{
        content:      content,
        cursor:       position{lastRow, lastCol},
        filename:     filename,
        modified:     false,
        lastActivity: now,
        typingTime:   existingTime,
        startTime:    now,
        statsFile:    statsFile,
    }
}

func (m Model) Init() tea.Cmd {
    return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tickMsg:
        if time.Since(m.lastActivity) < time.Minute {
            m.typingTime += time.Second
        }
        return m, tickCmd()

    case tea.WindowSizeMsg:
        m.viewport.width = msg.Width
        m.viewport.height = msg.Height - 2

    case tea.KeyMsg:
        m.lastActivity = time.Now()
        switch msg.Type {
        case tea.KeyCtrlC:
            if m.modified {
                _ = saveFile(m.filename, m.content)
            }
            _ = saveStats(m.statsFile, m.typingTime, m.content)
            return m, tea.Quit
        case tea.KeyCtrlS:
            if err := saveFile(m.filename, m.content); err == nil {
                m.modified = false
            }
            _ = saveStats(m.statsFile, m.typingTime, m.content)
        case tea.KeyUp:
            if m.cursor.row > 0 {
                m.cursor.row--
                if m.cursor.col > len(m.content[m.cursor.row]) {
                    m.cursor.col = len(m.content[m.cursor.row])
                }
            }
        case tea.KeyDown:
            if m.cursor.row < len(m.content)-1 {
                m.cursor.row++
                if m.cursor.col > len(m.content[m.cursor.row]) {
                    m.cursor.col = len(m.content[m.cursor.row])
                }
            }
        case tea.KeyLeft:
            if m.cursor.col > 0 {
                m.cursor.col--
            } else if m.cursor.row > 0 {
                m.cursor.row--
                m.cursor.col = len(m.content[m.cursor.row])
            }
        case tea.KeyRight:
            if m.cursor.col < len(m.content[m.cursor.row]) {
                m.cursor.col++
            } else if m.cursor.row < len(m.content)-1 {
                m.cursor.row++
                m.cursor.col = 0
            }
        case tea.KeyEnter:
            m.modified = true
            currentLine := m.content[m.cursor.row]
            beforeCursor := currentLine[:m.cursor.col]
            afterCursor := currentLine[m.cursor.col:]
            m.content[m.cursor.row] = beforeCursor
            newContent := make([]string, len(m.content)+1)
            copy(newContent[:m.cursor.row+1], m.content[:m.cursor.row+1])
            newContent[m.cursor.row+1] = afterCursor
            copy(newContent[m.cursor.row+2:], m.content[m.cursor.row+1:])
            m.content = newContent
            m.cursor.row++
            m.cursor.col = 0
        case tea.KeyBackspace:
            m.modified = true
            if m.cursor.col > 0 {
                line := m.content[m.cursor.row]
                m.content[m.cursor.row] = line[:m.cursor.col-1] + line[m.cursor.col:]
                m.cursor.col--
            } else if m.cursor.row > 0 {
                prevLine := m.content[m.cursor.row-1]
                currentLine := m.content[m.cursor.row]
                m.cursor.col = len(prevLine)
                m.content[m.cursor.row-1] = prevLine + currentLine
                newContent := make([]string, len(m.content)-1)
                copy(newContent[:m.cursor.row], m.content[:m.cursor.row])
                copy(newContent[m.cursor.row:], m.content[m.cursor.row+1:])
                m.content = newContent
                m.cursor.row--
            }
        case tea.KeySpace:
            m.modified = true
            line := m.content[m.cursor.row]
            m.content[m.cursor.row] = line[:m.cursor.col] + " " + line[m.cursor.col:]
            m.cursor.col++
        case tea.KeyRunes:
            m.modified = true
            line := m.content[m.cursor.row]
            m.content[m.cursor.row] = line[:m.cursor.col] + string(msg.Runes) + line[m.cursor.col:]
            m.cursor.col += len(msg.Runes)
        }
    }
    return m, nil
}

func wordWrap(line string, width int) []string {
    if width <= 0 || len(line) <= width {
        return []string{line}
    }

    var wrapped []string
    var currentLine strings.Builder
    words := strings.Fields(line)

    leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
    if leadingSpaces > 0 {
        currentLine.WriteString(strings.Repeat(" ", leadingSpaces))
    }

    for _, word := range words {
        wordLen := len(word)
        currentLen := currentLine.Len()
        if currentLen > 0 && currentLen+1+wordLen > width {
            if currentLine.Len() > 0 {
                wrapped = append(wrapped, currentLine.String())
                currentLine.Reset()
            }
        }
        if currentLine.Len() > 0 {
            currentLine.WriteString(" ")
        }
        if wordLen > width {
            remainingWord := word
            for len(remainingWord) > 0 {
                availableSpace := width - currentLine.Len()
                if availableSpace <= 0 {
                    wrapped = append(wrapped, currentLine.String())
                    currentLine.Reset()
                    availableSpace = width
                }
                take := availableSpace
                if take > len(remainingWord) {
                    take = len(remainingWord)
                }
                currentLine.WriteString(remainingWord[:take])
                remainingWord = remainingWord[take:]
                if currentLine.Len() >= width && len(remainingWord) > 0 {
                    wrapped = append(wrapped, currentLine.String())
                    currentLine.Reset()
                }
            }
        } else {
            currentLine.WriteString(word)
        }
    }
    if currentLine.Len() > 0 {
        wrapped = append(wrapped, currentLine.String())
    }
    if len(wrapped) == 0 {
        wrapped = []string{line}
    }
    return wrapped
}

func (m Model) View() string {
    var s strings.Builder

    maxContentHeight := m.viewport.height - 2
    if maxContentHeight < 1 {
        maxContentHeight = 1
    }

    visualCursorRow := 0
    for i := 0; i < m.cursor.row; i++ {
        line := m.content[i]
        trimmedLine := strings.TrimSpace(line)
        isGhostText := strings.HasPrefix(trimmedLine, "<!--") && strings.HasSuffix(trimmedLine, "-->")
        var wrappedLines []string
        if isGhostText {
            content := strings.TrimSuffix(strings.TrimPrefix(trimmedLine, "<!--"), "-->")
            content = strings.TrimSpace(content)
            wrappedLines = wordWrap(content, m.viewport.width)
        } else {
            wrappedLines = wordWrap(line, m.viewport.width)
        }
        visualCursorRow += len(wrappedLines)
    }

    var scrollOffset int
    if visualCursorRow >= maxContentHeight {
        scrollOffset = visualCursorRow - maxContentHeight + 1
    }

    visibleLines := 0
    currentVisualRow := 0

    for i := 0; i < len(m.content); i++ {
        line := m.content[i]
        trimmedLine := strings.TrimSpace(line)
        isGhostText := strings.HasPrefix(trimmedLine, "<!--") && strings.HasSuffix(trimmedLine, "-->")
        var wrappedLines []string
        if isGhostText {
            content := strings.TrimSuffix(strings.TrimPrefix(trimmedLine, "<!--"), "-->")
            content = strings.TrimSpace(content)
            wrappedLines = wordWrap(content, m.viewport.width)
        } else {
            wrappedLines = wordWrap(line, m.viewport.width)
        }
        for wrapIndex, wrappedLine := range wrappedLines {
            if currentVisualRow < scrollOffset {
                currentVisualRow++
                continue
            }
            if visibleLines >= maxContentHeight {
                break
            }
            if i == m.cursor.row && wrapIndex == 0 {
                if m.cursor.col < len(line) {
                    if m.cursor.col < len(wrappedLine) {
                        s.WriteString(wrappedLine[:m.cursor.col])
                        s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Bold(true).Render("│"))
                        s.WriteString(wrappedLine[m.cursor.col:])
                    } else {
                        s.WriteString(wrappedLine)
                        if wrapIndex == len(wrappedLines)-1 {
                            s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Bold(true).Render("│"))
                        }
                    }
                } else {
                    s.WriteString(wrappedLine)
                    if wrapIndex == len(wrappedLines)-1 {
                        s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FF1493")).Bold(true).Render("│"))
                    }
                }
            } else if isGhostText {
                ghostStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Italic(true)
                s.WriteString(ghostStyle.Render(wrappedLine))
            } else {
                s.WriteString(wrappedLine)
            }
            s.WriteString("\n")
            visibleLines++
            currentVisualRow++
        }
        if visibleLines >= maxContentHeight {
            break
        }
    }

    for i := visibleLines; i < maxContentHeight; i++ {
        s.WriteString("~\n")
    }

    wordCount := 0
    for _, line := range m.content {
        trimmedLine := strings.TrimSpace(line)
        if strings.HasPrefix(trimmedLine, "<!--") && strings.HasSuffix(trimmedLine, "-->") {
            continue
        }
        words := strings.Fields(line)
        wordCount += len(words)
    }

    minutes := int(m.typingTime.Minutes())
    timeStr := fmt.Sprintf("%dm", minutes)
    targetWords := 500
    progress := float64(wordCount) / float64(targetWords)
    if progress > 1.0 {
        progress = 1.0
    }

    leftText := fmt.Sprintf("%d/%d", wordCount, targetWords)
    rightText := timeStr
    padding := "  "
    minTextWidth := len(leftText) + len(rightText) + len(padding)*2 + 2

    if m.viewport.width < minTextWidth+3 {
        statusBar := fmt.Sprintf("%d words %s", wordCount, timeStr)
        if len(statusBar) > m.viewport.width {
            statusBar = fmt.Sprintf("%dw", wordCount)
        }
        s.WriteString("\n")
        s.WriteString(statusBar)
    } else {
        availableWidth := m.viewport.width - minTextWidth
        if availableWidth < 5 {
            availableWidth = 5
        }
        filledWidth := int(progress * float64(availableWidth))
        var progressBar strings.Builder
        progressBar.WriteString("[")
        for i := 0; i < availableWidth; i++ {
            if i < filledWidth {
                progressBar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#AA6688")).Render("━"))
            } else {
                progressBar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render("─"))
            }
        }
        progressBar.WriteString("]")
        leftStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
        rightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
        statusBar := leftStyle.Render(leftText) + padding + progressBar.String() + padding + rightStyle.Render(rightText)
        s.WriteString("\n")
        s.WriteString(statusBar)
    }

    return s.String()
}



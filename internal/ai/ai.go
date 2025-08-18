package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// loadAPIKey loads the API key from environment or config file
func loadAPIKey() string {
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

// getRecentNotes reads notes from the last few days
func getRecentNotes(days int) (string, error) {
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

func callAnthropic(prompt string) (string, error) {
	apiKey := loadAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("No API key found. Run 'river onboard' to set up AI features")
	}
	client := anthropic.NewClient()
	systemPrompt := `You are an AI assistant helping with personal productivity. Based on the provided notes from the last few days, generate organized lists of TODOs in different categories.

Create the following sections:

**WORK TODOs:**
- Tasks related to professional projects, meetings, deadlines
- Follow-ups with colleagues or clients
- Work-related goals and commitments

**PERSONAL TODOs:**
- Personal tasks, household items, family commitments
- Health, fitness, and self-care items
- Financial and administrative tasks

**PROJECTS & IDEAS:**
- Ideas that need further exploration or research
- Long-term goals and strategic thinking
- Creative projects or learning opportunities
- Reflection and planning items

Focus on:
- Incomplete tasks or projects mentioned
- Follow-ups needed 
- Ideas that could be developed further
- Goals or commitments that need action
- Any deadlines or time-sensitive items

IMPORTANT: For each TODO item, include a brief rationale in parentheses that cites or references the specific note content that led to this suggestion. For example:
"1. Follow up with John about the project proposal (mentioned meeting him on Tuesday but no follow-up scheduled)"

Format your response with clear section headers and numbered lists under each. Be specific and concise. If a category has no clear TODOs, you may omit that section or suggest general productivity actions based on the content themes.`
	userPrompt := fmt.Sprintf("Here are my notes from the last few days:\n\n%s\n\nPlease generate organized lists of TODOs based on this content, categorized by Work, Home, and Deeper Thought items.", prompt)
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1000,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
		Messages: []anthropic.MessageParam{
			{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					{OfText: &anthropic.TextBlockParam{Text: userPrompt, Type: "text"}},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("Anthropic API error: %v", err)
	}
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no response content from Anthropic")
	}
	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}
	return "", fmt.Errorf("unexpected response format from Anthropic")
}

func callAnthropicForInsights(prompt string) (string, error) {
	apiKey := loadAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("No API key found. Run 'river onboard' to set up AI features")
	}
	client := anthropic.NewClient()
	systemPrompt := `You are an AI assistant specialized in analyzing personal notes to identify patterns, themes, and insights. Based on the provided notes from the last few days, provide a thoughtful analysis.

Please create the following sections:

**THEMES & PATTERNS:**
- Recurring topics or concerns that appear across multiple days
- Emotional patterns or mood trends you notice
- Productivity or energy level patterns
- Common challenges or obstacles mentioned

**KEY INSIGHTS:**
- What the notes reveal about current priorities and focus areas
- Potential blind spots or areas that might need attention
- Connections between different thoughts or ideas
- Progress or changes you can observe over time

**OBSERVATIONS:**
- Notable differences between days or shifts in thinking
- Areas where there seems to be mental clarity vs confusion
- Signs of growth, learning, or development
- Recurring questions or curiosities

Be thoughtful and nuanced in your analysis. Focus on helping the person understand their own thinking patterns and mental landscape. Cite specific examples from the notes when possible to support your observations.

Format your response with clear section headers and insightful commentary. Be encouraging and constructive while being honest about what you observe.`
	userPrompt := fmt.Sprintf("Here are my notes from the last few days:\n\n%s\n\nPlease analyze these notes for patterns, themes, and insights about my thinking and mental state.", prompt)
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 1200,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
		Messages: []anthropic.MessageParam{{
			Role:    anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{OfText: &anthropic.TextBlockParam{Text: userPrompt, Type: "text"}}},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("Anthropic API error: %v", err)
	}
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no response content from Anthropic")
	}
	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}
	return "", fmt.Errorf("unexpected response format from Anthropic")
}

func callAnthropicForSimpleTodos(prompt string) (string, error) {
	apiKey := loadAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("No API key found. Run 'river onboard' to set up AI features")
	}
	client := anthropic.NewClient()
	systemPrompt := `You are an AI assistant that extracts actionable TODOs from personal notes. Based on the provided notes, identify clear, specific tasks that need to be completed.

Focus on:
- Explicit tasks mentioned (things to do, call, buy, schedule, etc.)
- Incomplete projects or commitments
- Follow-ups needed with people
- Deadlines or time-sensitive items
- Ideas that require action to move forward
- Problems mentioned that need solutions

For each TODO, include:
1. A clear, actionable description
2. Brief context from the notes explaining why this is needed (in parentheses)

Format as a simple numbered list. Be specific and concrete - avoid vague items. Only include things that are genuinely actionable. If no clear TODOs can be identified, say so and suggest that the person might want to be more explicit about action items in future notes.

Keep the list focused and practical - aim for quality over quantity.`
	userPrompt := fmt.Sprintf("Here are my notes from the last few days:\n\n%s\n\nPlease extract actionable TODOs from these notes.", prompt)
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 800,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
		Messages: []anthropic.MessageParam{{
			Role:    anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{OfText: &anthropic.TextBlockParam{Text: userPrompt, Type: "text"}}},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("Anthropic API error: %v", err)
	}
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no response content from Anthropic")
	}
	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}
	return "", fmt.Errorf("unexpected response format from Anthropic")
}

func callAnthropicForPrompts(notes string) ([]string, error) {
	apiKey := loadAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("No API key found. Run 'river onboard' to set up AI features")
	}
	client := anthropic.NewClient()
	systemPrompt := `You are an AI assistant that creates personalized journal prompts based on someone's recent journal entries. Your goal is to help them reflect more deeply, explore unresolved thoughts, and continue their personal growth journey.

Based on their recent notes, generate 7 thoughtful journal prompts that:

1. Build on themes and topics they've been exploring
2. Help them dig deeper into unresolved questions or concerns
3. Encourage reflection on patterns you notice
4. Challenge them to think about things from new perspectives
5. Support their goals and aspirations
6. Address any emotional or mental patterns you observe
7. Connect different ideas they've mentioned

Guidelines:
- Make prompts specific to their content, not generic
- Reference specific topics, people, or situations they've mentioned when relevant
- Vary the types of prompts (reflection, planning, gratitude, challenge, insight, etc.)
- Keep prompts open-ended but focused
- Make them thought-provoking but not overwhelming
- Consider their current emotional state and energy level

Format your response as a JSON array of strings, with exactly 7 prompts. Each prompt should be a complete question or writing prompt. Example format:
["First prompt here?", "Second prompt here?", "Third prompt here?", "Fourth prompt here?", "Fifth prompt here?", "Sixth prompt here?", "Seventh prompt here?"]`
	userPrompt := fmt.Sprintf("Here are my journal entries from the last week:\n\n%s\n\nPlease generate 7 personalized journal prompts based on these entries.", notes)
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 800,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
		Messages: []anthropic.MessageParam{{
			Role:    anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{OfText: &anthropic.TextBlockParam{Text: userPrompt, Type: "text"}}},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("Anthropic API error: %v", err)
	}
	if len(response.Content) == 0 {
		return nil, fmt.Errorf("no response content from Anthropic")
	}
	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			var prompts []string
			responseText := strings.TrimSpace(content.Text)
			startIdx := strings.Index(responseText, "[")
			endIdx := strings.LastIndex(responseText, "]")
			if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
				jsonStr := responseText[startIdx : endIdx+1]
				if err := json.Unmarshal([]byte(jsonStr), &prompts); err != nil {
					jsonStr = strings.Trim(jsonStr, "[]")
					parts := strings.Split(jsonStr, "\", \"")
					for _, part := range parts {
						part = strings.Trim(part, "\"")
						part = strings.ReplaceAll(part, "\\\"", "\"")
						if part != "" {
							prompts = append(prompts, part)
						}
					}
				}
			}
			if len(prompts) == 0 {
				return nil, fmt.Errorf("could not parse prompts from response")
			}
			return prompts, nil
		}
	}
	return nil, fmt.Errorf("unexpected response format from Anthropic")
}

func callAnthropicForStatsInsights(stats AggregatedStats, recentNotes string) (string, error) {
	apiKey := loadAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("No API key found. Run 'river onboard' to set up AI features")
	}
	client := anthropic.NewClient()
	statsSummary := fmt.Sprintf(`Writing Statistics Summary:
- Total Words Written: %d
- Total Writing Time: %s
- Days Active: %d
- Current Streak: %d days
- Longest Streak: %d days
- Average Words per Day: %d
- Most Productive Day: %s (%d words)

Recent Writing Activity (Last 14 Days):
`, stats.TotalWords, formatDuration(stats.TotalTypingTime), stats.TotalDays,
		stats.CurrentStreak, stats.LongestStreak,
		stats.AverageWords, stats.MostProductiveDay, stats.MostProductiveWords)
	startIdx := len(stats.DailyStats) - 14
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < len(stats.DailyStats); i++ {
		stat := stats.DailyStats[i]
		statsSummary += fmt.Sprintf("- %s: %d words in %s\n",
			stat.Date.Format("Mon, Jan 2"), stat.Words, formatDuration(stat.TypingTime))
	}
	systemPrompt := `You are an AI assistant that analyzes writing habits and journal statistics to provide personalized insights. Based on the provided statistics and recent journal entries, create a comprehensive analysis that helps the writer understand their patterns and improve their practice.

Please provide insights in the following sections:

**📊 PRODUCTIVITY PATTERNS**
- Analyze writing frequency, volume trends, and time patterns
- Identify peak productivity days/times if evident
- Comment on consistency and streak patterns
- Note any concerning gaps or declines

**🎯 HABITS & CONSISTENCY**
- Evaluate the strength of their writing habit
- Comment on their streak performance
- Suggest ways to improve consistency
- Recognize achievements and milestones

**💭 CONTENT THEMES**
- Based on recent entries, identify recurring themes or concerns
- Note any emotional patterns or mood trends
- Highlight areas of focus or preoccupation
- Suggest unexplored topics they might benefit from

**🚀 RECOMMENDATIONS**
- Provide 3-5 specific, actionable suggestions
- Include both habit-building and content-focused advice
- Suggest optimal writing times or goals based on their data
- Recommend prompts or exercises based on their patterns

Keep the tone encouraging but honest. Use data to support observations. Make recommendations specific and achievable. Format with clear headers and bullet points.`
	userPrompt := fmt.Sprintf("%s\n\nRecent Journal Entries:\n%s\n\nPlease analyze my writing patterns and provide personalized insights.", statsSummary, recentNotes)
	ctx := context.Background()
	response, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 1500,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt, Type: "text"}},
		Messages: []anthropic.MessageParam{{
			Role:    anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{OfText: &anthropic.TextBlockParam{Text: userPrompt, Type: "text"}}},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("Anthropic API error: %v", err)
	}
	if len(response.Content) == 0 {
		return "", fmt.Errorf("no response content from Anthropic")
	}
	for _, content := range response.Content {
		if content.Type == "text" && content.Text != "" {
			return content.Text, nil
		}
	}
	return "", fmt.Errorf("unexpected response format from Anthropic")
}

// CallAnthropicForStatsInsights is an exported wrapper used by other packages.
func CallAnthropicForStatsInsights(stats AggregatedStats, recentNotes string) (string, error) {
	return callAnthropicForStatsInsights(stats, recentNotes)
}

// Public command helpers (CLI-facing)
func GenerateTodos() error {
	fmt.Println("🤔 Thinking about your recent notes...")
	notes, err := getRecentNotes(3)
	if err != nil {
		return fmt.Errorf("error reading recent notes: %v", err)
	}
	if strings.TrimSpace(notes) == "" {
		fmt.Println("📝 No recent notes found. Try writing some thoughts first!")
		return nil
	}
	fmt.Println("📖 Analyzing notes from the last 3 days...")
	todos, err := callAnthropic(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}
	fmt.Println("\n✨ Here are some TODOs based on your recent notes:")
	fmt.Println(todos)
	return nil
}

func GenerateInsights() error {
	fmt.Println("🔍 Analyzing your recent notes for insights...")
	notes, err := getRecentNotes(3)
	if err != nil {
		return fmt.Errorf("error reading recent notes: %v", err)
	}
	if strings.TrimSpace(notes) == "" {
		fmt.Println("📝 No recent notes found. Try writing some thoughts first!")
		return nil
	}
	fmt.Println("🧠 Identifying patterns and themes...")
	insights, err := callAnthropicForInsights(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}
	fmt.Println("\n💡 Here are insights from your recent notes:")
	fmt.Println(insights)
	return nil
}

func GenerateSimpleTodos() error {
	fmt.Println("📋 Extracting TODOs from your recent notes...")
	notes, err := getRecentNotes(3)
	if err != nil {
		return fmt.Errorf("error reading recent notes: %v", err)
	}
	if strings.TrimSpace(notes) == "" {
		fmt.Println("📝 No recent notes found. Try writing some thoughts first!")
		return nil
	}
	fmt.Println("✅ Identifying actionable items...")
	todos, err := callAnthropicForSimpleTodos(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}
	fmt.Println("\n📝 Here are actionable TODOs from your notes:")
	fmt.Println(todos)
	return nil
}

func GeneratePrompts() error {
	fmt.Println("✨ Creating personalized prompts based on your recent writing...")
	notes, err := getRecentNotes(7)
	if err != nil {
		return fmt.Errorf("error reading recent notes: %v", err)
	}
	if strings.TrimSpace(notes) == "" {
		fmt.Println("📝 No recent notes found. Try writing some thoughts first!")
		return nil
	}
	fmt.Println("🔮 Analyzing your journal entries from the last week...")
	prompts, err := callAnthropicForPrompts(notes)
	if err != nil {
		return fmt.Errorf("error calling AI: %v", err)
	}
	fmt.Println("\n🌟 Here are personalized journal prompts based on your recent reflections:\n")
	for i, prompt := range prompts {
		fmt.Printf("%d. %s\n\n", i+1, prompt)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	riverDir := filepath.Join(homeDir, "river", "notes")
	promptsFile := filepath.Join(riverDir, ".prompts")
	var promptData strings.Builder
	promptData.WriteString(fmt.Sprintf("# Generated on %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	for i, prompt := range prompts {
		promptData.WriteString(fmt.Sprintf("%d. %s\n", i+1, prompt))
	}
	if err := os.WriteFile(promptsFile, []byte(promptData.String()), 0644); err != nil {
		fmt.Printf("\n⚠️  Could not save prompts to file: %v\n", err)
	} else {
		fmt.Printf("\n💾 Prompts saved to %s\n", promptsFile)
		fmt.Println("   These prompts will be used for your daily notes over the next week.")
	}
	fmt.Println("\n💡 Tip: Run 'river prompts' weekly to get fresh, personalized prompts!")
	return nil
}

// GenerateNotesForInsights returns recent notes content (last 7 days) for use by the stats UI.
func GenerateNotesForInsights() (string, error) { return getRecentNotes(7) }

// Shared types/utilities for stats insights
type AggregatedStats struct {
	TotalWords          int
	TotalTypingTime     time.Duration
	TotalDays           int
	CurrentStreak       int
	LongestStreak       int
	AverageWords        int
	MostProductiveDay   string
	MostProductiveWords int
	DailyStats          []DailyStat
}

type DailyStat struct {
	Date       time.Time
	Words      int
	TypingTime time.Duration
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// Convenience function for statsui to pull recent notes text (last 7 days)
// (kept above; avoid duplicate definitions)
